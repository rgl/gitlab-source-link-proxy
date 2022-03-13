Basic Authorization support for GitLab Source Link requests.

You can see this used at [rgl/gitlab-vagrant](https://github.com/rgl/gitlab-vagrant).

# GitLab configuration

Configure GitLab nginx to proxy the Visual Studio requests through our proxy by applying the patch at:

https://github.com/rgl/gitlab-vagrant/blob/master/gitlab-http.conf-gitlab-source-link-proxy.patch

Restart nginx:

```bash
gitlab-ctl restart nginx
```

Build the binary:

```bash
make
```

Manually run it:

```bash
./dist/gitlab-source-link-proxy_$(go env GOOS)_$(go env GOARCH)/gitlab-source-link-proxy \
  --gitlab-base-url https://gitlab.example.com
```

Try it:

```bash
http --verify=no -v https://root:password@gitlab.example.com/example/ubuntu-vagrant/raw/master/.gitignore User-Agent:SourceLink
```

Install it as a systemd service:

```bash
# add the gitlab-source-link-proxy user.
groupadd --system gitlab-source-link-proxy
adduser \
    --system \
    --disabled-login \
    --no-create-home \
    --gecos '' \
    --ingroup gitlab-source-link-proxy \
    --home /opt/gitlab-source-link-proxy \
    gitlab-source-link-proxy
install -d -o root -g gitlab-source-link-proxy -m 750 /opt/gitlab-source-link-proxy
install -d -o root -g gitlab-source-link-proxy -m 750 /opt/gitlab-source-link-proxy/bin

# install the binary.
install -o root -g root -m 555 gitlab-source-link-proxy /opt/gitlab-source-link-proxy/bin

# install the systemd service.
cat >/etc/systemd/system/gitlab-source-link-proxy.service <<EOF
[Unit]
Description=gitlab-source-link-proxy
After=network.target

[Service]
Type=simple
User=gitlab-source-link-proxy
Group=gitlab-source-link-proxy
ExecStart=/opt/gitlab-source-link-proxy/bin/gitlab-source-link-proxy --gitlab-base-url https://gitlab.example.com
WorkingDirectory=/opt/gitlab-source-link-proxy
Restart=on-abort

[Install]
WantedBy=multi-user.target
EOF
systemctl enable gitlab-source-link-proxy
systemctl start gitlab-source-link-proxy
```

Show the service status and logs:

```bash
systemctl status gitlab-source-link-proxy
journalctl -u gitlab-source-link-proxy
```

# Visual Studio configuration

To be able to step through external source code you need to disable `Enable Just My Code` setting in the `Tools | Options | Debugger` window.

To be able to download source code you need to configure Visual Studio to authenticate your GitLab domain requests.

For Visual Studio 15.8+, open the `Tools | Options | Debugger` window and check `Fall back to Git Credential Manager authentication for all Source Link requests`.

For older versions of Visual Studio 15:

  1. Open the `Developer Command Prompt for VS 2017`
  2. run `notepad "%VSINSTALLDIR%\Common7\IDE\CommonExtensions\Platform\Debugger\VsDebugPresentationPackage.pkgdef"`
  3. Add your GitLab domain as a Git Credential Manager Authority in the corresponding ini section, e.g.:
      ```ini
      [$RootKey$\Debugger\GitCredentialManager\Authorities]
      "raw.githubusercontent.com"="https://github.com"
      "gitlab.example.com"="https://gitlab.example.com"
      ```
  4. run `devenv /setup`

# Visual Studio Code OmniSharp configuration

To be able to step through external source code you need to disable `Just My Code` in the [Debugger Launch Settings](https://github.com/OmniSharp/omnisharp-vscode/blob/master/debugger-launchjson.md#just-my-code).

**NB There is no support for accessing private repositories, as it does not support authentication. For more information see the [Source Link options documentation](https://github.com/OmniSharp/omnisharp-vscode/blob/master/debugger-launchjson.md#source-link-options) and upvote the [#2071 issue](https://github.com/OmniSharp/omnisharp-vscode/issues/2071).**
