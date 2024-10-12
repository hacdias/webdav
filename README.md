# webdav

[![Go Report Card](https://goreportcard.com/badge/github.com/hacdias/webdav?style=flat-square)](https://goreportcard.com/report/hacdias/webdav)
[![Version](https://img.shields.io/github/release/hacdias/webdav.svg?style=flat-square)](https://github.com/hacdias/webdav/releases/latest)
[![Docker Pulls](https://img.shields.io/docker/pulls/hacdias/webdav?style=flat-square)](https://hub.docker.com/r/hacdias/webdav)

A simple and standalone [WebDAV](https://en.wikipedia.org/wiki/WebDAV) server.

## Install

For a manual install, please refer to the [releases](https://github.com/hacdias/webdav/releases) page and download the correct binary for your system. Alternatively, you can build or install it from source using the Go toolchain. You can either clone the repository and execute `go build`, or directly install it, using:

```
go install github.com/hacdias/webdav/v5@latest
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

To use with Docker, you need to provide a configuration file and mount the data directories. For example, let's take the following configuration file that simply sets the port to `6060` and the directory to `/data`.

```yaml
port: 6060
directory: /data
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
port: 6065

# TLS-related settings if you want to enable TLS directly.
tls: false
cert: cert.pem
key: key.pem

# Prefix to apply to the WebDAV path-ing. Default is '/'.
prefix: /

# Enable or disable debug logging. Default is 'false'.
debug: false

# Disable sniffing the files to detect their content type. Default is 'false'.
noSniff: false

# The directory that will be able to be accessed by the users when connecting.
# This directory will be used by users unless they have their own 'directory' defined.
# Default is '.' (current directory).
directory: .

# The default permissions for users. This is a case insensitive option. Possible
# permissions: C (Create), R (Read), U (Update), D (Delete). You can combine multiple
# permissions. For example, to allow to read and create, set "RC". Default is "R".
permissions: R

# The default permissions rules for users. Default is none.
rules: []

# Logging configuration
log:
  # Logging format ('console', 'json'). Default is 'console'.
  format: console
  # Enable or disable colors. Default is 'true'. Only applied if format is 'console'.
  colors: true
  # Logging outputs. You can have more than one output. Default is only 'stderr'.
  outputs:
  - stderr

# CORS configuration
cors:
  # Whether or not CORS configuration should be applied. Default is 'false'.
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

# The list of users. If the list is empty, then there will be no authentication.
# Otherwise, basic authentication will automatically be configured.
#
# If you're delegating the authentication to a different service, you can proxy
# the username using basic authentication, and then disable webdav's password
# check using the option:
#
# noPassword: true
users:
  # Example 'admin' user with plaintext password.
  - username: admin
    password: admin
  # Example 'john' user with bcrypt encrypted password, with custom directory.
  - username: john
    password: "{bcrypt}$2y$10$zEP6oofmXFeHaeMfBNLnP.DO8m.H.Mwhd24/TOX2MWLxAExXi4qgi"
    directory: /another/path
  # Example user whose details will be picked up from the environment.
  - username: "{env}ENV_USERNAME"
    password: "{env}ENV_PASSWORD"
  - username: basic
    password: basic
    # Override default permissions.
    permissions: CRUD
    rules:
      # With this rule, the user CANNOT access /some/files.
      - path: /some/file
        permissions: none
      # With this rule, the user CAN create, read, update and delete within /public/access.
      - path: /public/access/
        permissions: CRUD
      # With this rule, the user CAN read and update all files ending with .js. It uses
      # a regular expression.
      - regex: "^.+.js$"
        permissions: RU
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
  proxy_set_header Host $host;
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

### Fail2Ban Setup

To add security against brute-force attacks in your WebDAV server, you can configure Fail2Ban to ban IP addresses after a set number of failed login attempts.

#### Filter Configuration

Create a new filter rule under `filter.d/webdav.conf`:

```ini
[INCLUDES]
before = common.conf

[Definition]
# Failregex to match "invalid password" and extract remote_address only
failregex = ^.*invalid password\s*\{.*"remote_address":\s*"<HOST>"\s*\}

# Failregex to match "invalid username" and extract remote_address only (if applicable)
failregex += ^.*invalid username\s*\{.*"remote_address":\s*"<HOST>"\s*\}

ignoreregex =
```

This configuration will capture invalid login attempts and extract the IP address to ban.

#### Jail Configuration

In `jail.d/webdav.conf`, define the jail that monitors your WebDAV log for failed login attempts:

```ini
[webdav]

enabled = true
port = [your_port]
filter = webdav
logpath = [your_log_path]
banaction = iptables-allports
ignoreself = false
```

- Replace `[your_port]` with the port your WebDAV server is running on.
- Replace `[your_log_path]` with the path to your WebDAV log file.

#### Final Steps

1. Restart Fail2Ban to apply these configurations:

   ```bash
   sudo systemctl restart fail2ban
   ```

2. Verify that Fail2Ban is running and monitoring your WebDAV logs:

   ```bash
   sudo fail2ban-client status webdav
   ```

With this setup, Fail2Ban will automatically block IP addresses that exceed the allowed number of failed login attempts.

## Contributing

Feel free to open an issue or a pull request.

## License

[MIT License](LICENSE) © [Henrique Dias](https://hacdias.com)
