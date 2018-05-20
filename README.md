Configure GitLab nginx to proxy the Visual Studio requests through our proxy:

```
vim /var/opt/gitlab/nginx/conf/gitlab-http.conf 
...
  location / {
    if ($http_user_agent ~ "SourceLink") {
      #proxy_set_header Authorization "Basic cm9vdDpwYXNzd29yZA==";
      proxy_pass http://127.0.0.1:7000;
      break;
    }
    proxy_cache off;
    proxy_pass  http://gitlab-workhorse;
  }

gitlab-ctl restart nginx
```

Build this binary:

```bash
go get -u -v github.com/jamiealquiza/bicache
go get -u -v golang.org/x/crypto/blake2b
go build -v
```

Run it:

```bash
./gitlab-source-link-proxy --gitlab-base-url https://gitlab.example.com
```

Try it:

```bash
http --verify=no -v https://root:password@gitlab.example.com/root/ubuntu-vagrant/raw/master/.gitignore User-Agent:SourceLink
```
