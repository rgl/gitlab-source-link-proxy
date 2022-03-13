package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"

	"github.com/jamiealquiza/bicache"
	"golang.org/x/crypto/blake2b"
)

type AccessTokenRequest struct {
	GrantType string `json:"grant_type"`
	Scope     string `json:"scope"` // NB GitLab 10.7.3 has this, but its not documented. See https://gitlab.com/gitlab-org/gitlab-ce/issues/45000
	Username  string `json:"username"`
	Password  string `json:"password"`
}

type AccessTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	Scope       string `json:"scope"` // NB GitLab 10.7.3 has this, but its not documented. See https://gitlab.com/gitlab-org/gitlab-ce/issues/45000
	//ExpiresInSeconds int32  `json:"expires_in"` // NB GitLab 10.7.3 does not have this. See https://gitlab.com/gitlab-org/gitlab-ce/issues/45000
}

// see https://docs.gitlab.com/ce/api/oauth2.html#resource-owner-password-credentials-flow
func GetAccessToken(gitLabTokenURL, username, password string) (*AccessTokenResponse, error) {
	requestJSON, err := json.Marshal(&AccessTokenRequest{
		GrantType: "password",
		Scope:     "read_repository",
		Username:  username,
		Password:  password,
	})
	if err != nil {
		return nil, err
	}
	response, err := http.Post(gitLabTokenURL, "application/json", bytes.NewBuffer(requestJSON))
	if err != nil {
		return nil, err
	}
	dump, _ := httputil.DumpResponse(response, true)
	log.Printf("AccessToken response %q", dump)
	defer response.Body.Close()
	responseBody, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}
	if response.StatusCode != 200 {
		return nil, fmt.Errorf("invalid response %v", response)
	}
	// TODO check content-type.
	var accessTokenResponse AccessTokenResponse
	if err := json.Unmarshal(responseBody, &accessTokenResponse); err != nil {
		return nil, err
	}
	return &accessTokenResponse, nil
}

func GetCachedAccessToken(c *bicache.Bicache, tokenURL, username, password string) ([]byte, error) {
	hp := blake2b.Sum256([]byte(password))
	v := c.Get(username)
	if v != nil {
		if !bytes.HasPrefix(v.([]byte), hp[:]) {
			return nil, fmt.Errorf("invalid password")
		}
		log.Printf("Cache-Hit getting access token for username %s", username)
		return v.([]byte)[len(hp):], nil
	} else {
		log.Printf("Cache-Miss getting access token for username %s", username)
		response, err := GetAccessToken(tokenURL, username, password)
		if err != nil {
			return nil, err
		}
		if response.TokenType != "Bearer" {
			return nil, fmt.Errorf("unknown access token type: %s", response.TokenType)
		}
		t := []byte(response.AccessToken)
		v := append(hp[:], t...)
		c.SetTTL(username, v, 3600)
		return v[len(hp):], nil
	}
}

var (
	version = "unknown"
	commit  = "unknown"
	date    = "unknown"
)

var (
	listenAddressFlag      = flag.String("listen-address", "127.0.0.1:7000", "HOSTNAME:PORT where this http proxy listens at (e.g. 127.0.0.1:7000)")
	baseGitLabURLFlag      = flag.String("gitlab-base-url", "", "GitLab Base URL (e.g. https://gitlab.example.com/)")
	insecureSkipVerifyFlag = flag.Bool("tls-insecure-skip-verify", false, "Skip GitLab TLS verification")
)

func main() {
	flag.Parse()

	log.Printf("Starting gitlab-source-link-proxy (version %s; commit %s; date %s)", version, commit, date)

	if *baseGitLabURLFlag == "" {
		log.Printf("Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
		return
	}

	if *insecureSkipVerifyFlag {
		http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}

	c, err := bicache.New(&bicache.Config{
		MFUSize:    24,        // MFU capacity in keys
		MRUSize:    64,        // MRU capacity in keys
		ShardCount: 64,        // Shard count. Defaults to 512 if unset.
		AutoEvict:  60 * 1000, // Run TTL evictions + MRU->MFU promotions / evictions automatically every 60s.
		EvictLog:   true,      // Emit eviction timing logs.
		NoOverflow: false,     // Disallow Set ops when the MRU cache is full.
	})
	if err != nil {
		log.Fatal(err)
	}
	defer c.Close()

	gitLabBaseURL, err := url.Parse(strings.TrimRight(*baseGitLabURLFlag, "/"))
	if err != nil {
		log.Fatal(err)
	}
	gitLabTokenURL := gitLabBaseURL.String() + "/oauth/token"

	reverseProxy := httputil.NewSingleHostReverseProxy(gitLabBaseURL)
	defaultReverseProxyDirector := reverseProxy.Director
	reverseProxy.Director = func(r *http.Request) {
		defaultReverseProxyDirector(r)
		r.Header.Set("User-Agent", "gitlab-source-link-proxy") // TODO use this user-agent in all this application http requests.
		username, password, ok := r.BasicAuth()
		if !ok {
			log.Print("There is not basic auth in request")
			r.Header.Set("Authorization", "")
			return
		}
		accessToken, err := GetCachedAccessToken(c, gitLabTokenURL, username, password)
		if err != nil {
			log.Printf("Error getting the access token: %v", err)
			r.Header.Set("Authorization", "")
		} else {
			r.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
		}
	}
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		dump, _ := httputil.DumpRequest(r, false)
		log.Printf("%q", dump)
		auth := r.Header.Get("Authorization")
		if auth == "" {
			log.Printf("request not authenticated, requesting authentication")
			w.Header().Set("WWW-Authenticate", `Basic realm="GitLab"`)
			w.Header().Set("Cache-Control", `no-cache`)
			http.Error(w, "HTTP Basic: Access denied", 401)
			return
		}
		reverseProxy.ServeHTTP(w, r)
	})
	log.Fatal(http.ListenAndServe(*listenAddressFlag, nil))
}
