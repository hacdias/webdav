# webdav

[![Go Report Card](https://goreportcard.com/badge/github.com/hacdias/webdav?style=flat-square)](https://goreportcard.com/report/hacdias/webdav)
[![Version](https://img.shields.io/github/release/hacdias/webdav.svg?style=flat-square)](https://github.com/hacdias/webdav/releases/latest)
[![Docker Pulls](https://img.shields.io/docker/pulls/hacdias/webdav?style=flat-square)](https://hub.docker.com/r/hacdias/webdav)

A simple and standalone [WebDAV](https://en.wikipedia.org/wiki/WebDAV) server.

## Install

For a manual install, please refer to the [releases](https://github.com/hacdias/webdav/releases) page and download the correct binary for your system. Alternatively, you can build or install it from source using the Go toolchain. You can either clone the repository and execute `go build`, or directly install it, using:

```
go install github.com/hacdias/webdav/v4@latest
```

### Docker

Docker images are provided on both [GitHub's registry](https://github.com/hacdias/webdav/pkgs/container/webdav) and [Docker Hub](https://hub.docker.com/r/hacdias/webdav). You can pull the images using one of the following two commands. Note that this commands pull the latest released version. You can use specific tags to pin specific versions, or use `main` for the development branch.

```bash
# GitHub Registry
docker pull ghcr.io/hacdias/webdav:latest

# Docker Hub
docker pull hacdias/webdav:latest
```

## Usage

For usage information regarding the CLI, run `webdav --help`.

### Docker

To use with Docker, you need to provide a configuration file and mount the data directories. For example, let's take the following configuration file that simply sets the port to `6060` and the scope to `/data`.

```yaml
port: 6060
scope: /data
```

You can now run with the following Docker command, where you mount the configuration file inside the container, and the data directory too, as well as forwarding the port 6060. You will need to change this to match your own configuration.

```bash
docker run \
  -p 6060:6060 \
  -v $(pwd)/config.yml:/config.yml:ro \
  -v $(pwd)/data:/data \
  ghcr.io/hacdias/webdav -c /config.yml
```

## Configuration

`webdav` command line interface is really easy to use so you can easily create a WebDAV server for your own user. By default, it runs on a random free port and supports JSON, YAML and TOML configuration. An example of a YAML configuration with the default configurations:

```yaml
address: 0.0.0.0
port: 0

auth: true
tls: false
cert: cert.pem
key: key.pem
prefix: /
debug: false

# Default user settings (will be merged)
scope: .
modify: true
rules: []

# CORS configuration
cors:
  enabled: true
  credentials: true
  allowed_headers:
    - Depth
  allowed_hosts:
    - http://localhost:8080
  allowed_methods:
    - GET
  exposed_headers:
    - Content-Length
    - Content-Range

users:
  - username: admin
    password: admin
    scope: /a/different/path
  - username: encrypted
    password: "{bcrypt}$2y$10$zEP6oofmXFeHaeMfBNLnP.DO8m.H.Mwhd24/TOX2MWLxAExXi4qgi"
  - username: "{env}ENV_USERNAME"
    password: "{env}ENV_PASSWORD"
  - username: basic
    password: basic
    modify: false
    rules:
      - regex: false
        allow: false
        path: /some/file
      - path: /public/access/
        modify: true
```


### CORS

The `allowed_*` properties are optional, the default value for each of them will be `*`. `exposed_headers` is optional as well, but is not set if not defined. Setting `credentials` to `true` will allow you to:

1. Use `withCredentials = true` in javascript.
2. Use the `username:password@host` syntax.

## Caveats

### Reverse Proxy Service

When using a reverse proxy implementation, like Caddy, Nginx, or Apache, note that you need to forward the correct headers in order to avoid 502 errors. Here's a Nginx configuration example:

```nginx
location / {
  proxy_pass http://127.0.0.1:8080;
  proxy_set_header X-Real-IP $remote_addr;
  proxy_set_header REMOTE-HOST $remote_addr;
  proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
  proxy_set_header Host $http_host;
  proxy_redirect off;
}
```

## Examples

### Systemd

Example configuration of a [`systemd`](https://en.wikipedia.org/wiki/Systemd) service:

```toml
[Unit]
Description=WebDAV
After=network.target

[Service]
Type=simple
User=root
ExecStart=/usr/bin/webdav --config /opt/webdav.yml
Restart=on-failure

[Install]
WantedBy=multi-user.target
```

## Contributing

Feel free to open an issue or a pull request.

## License

[MIT License](LICENSE) © [Henrique Dias](https://hacdias.com)
