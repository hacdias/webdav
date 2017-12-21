# webdav

[![Build](https://img.shields.io/travis/hacdias/webdav.svg?style=flat-square)](https://travis-ci.org/hacdias/webdav)
[![Go Report Card](https://goreportcard.com/badge/github.com/hacdias/webdav?style=flat-square)](https://goreportcard.com/report/hacdias/webdav)

```webdav``` is a simple tool that creates a WebDAV server for you. By default, it runs on a random free port and supports JSON and YAML configuration. Here is a simple YAML configuration example:

```yaml
scope: /path/to/files
address: 0.0.0.0
port: 8080
users:
  - username: admin
    password: admin
  - username: basic
    password: basic
    modify:   false
    rules:
      - regex: false
      - allow: false
      - path: /some/file
```

You can specify the path to the configuration file using the `--config` flag. By default, it will search for a `config.{yaml,json}` file on your current working directory.

Download it [here](https://github.com/hacdias/webdav/releases).
