dist: gitlab-source-link-proxy.tgz

build: gitlab-source-link-proxy

gitlab-source-link-proxy: *.go go.*
	GOOS=linux GOARCH=amd64 go build -v -o $@ -ldflags="-s -w"

gitlab-source-link-proxy.tgz: gitlab-source-link-proxy
	rm -f $@
	tar czf $@ $^
	sha256sum $@

clean:
	rm -rf gitlab-source-link-proxy*

.PHONY: dist build clean
