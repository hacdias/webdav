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

The configuration can be provided as a YAML, JSON or TOML file. Below is an example of a YAML configuration file with all the options available, as well as what they mean.

```yaml
address: 0.0.0.0
port: 0

# TLS-related settings if you want to enable TLS directly.
tls: false
cert: cert.pem
key: key.pem

# Prefix to apply to the WebDAV path-ing. Default is "/".
prefix: /

# Enable or disable debug logging. Default is false.
debug: false

# Whether or not to have authentication. With authentication on, you need to
# define one or more users. Default is false.
auth: true

# The directory that will be able to be accessed by the users when connecting.
# This directory will be used by users unless they have their own 'scope' defined.
# Default is "/".
scope: /

# Whether the users can, by default, modify the contents. Default is false.
modify: true

# Default permissions rules to apply at the paths.
rules: []

# The list of users. Must be defined if auth is set to true.
users:
  # Example 'admin' user with plaintext password.
  - username: admin
    password: admin
  # Example 'john' user with bcrypt encrypted password, with custom scope.
  - username: john
    password: "{bcrypt}$2y$10$zEP6oofmXFeHaeMfBNLnP.DO8m.H.Mwhd24/TOX2MWLxAExXi4qgi"
    scope: /another/path
  # Example user whose details will be picked up from the environment.
  - username: "{env}ENV_USERNAME"
    password: "{env}ENV_PASSWORD"
  - username: basic
    password: basic
    # Override default modify.
    modify: false
    rules:
      # With this rule, the user CANNOT access /some/files.
      - path: /some/file
        allow: false
      # With this rule, the user CAN modify /public/access.
      - path: /public/access/
        modify: true
      # With this rule, the user CAN modify all files ending with .js. It uses
      # a regular expression.
      - path: "^*.js$"
        regex: true
        modify: true

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

```conf
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
