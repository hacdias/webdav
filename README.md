# webdav
Simple Go WebDAV server.
Forked from hacdias/webdav


# config
```
# Server related settings
address: 0.0.0.0
port: 1234
tls: false
cert: cert.pem
key: key.pem

# CORS configuration
cors:
  enabled: true
  credentials: true
  allowed_headers:
    - Depth
  allowed_hosts:
    - http://localhost:1234
  allowed_methods:
    - GET
  exposed_headers:
    - Content-Length
    - Content-Range

# https://bcrypt-generator.com/
users:
  - username: webdav
    password: webdav
    scopes:
      - root: E:/
        alias: pce
        rules:
          - path: /Temp
            allow_w: true

```
