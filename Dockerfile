FROM golang:1.22-alpine3.20 as build

LABEL org.opencontainers.image.source=https://github.com/hacdias/webdav
LABEL org.opencontainers.image.documentation=https://github.com/hacdias/webdav
LABEL org.opencontainers.image.description="A standalone WebDAV server"
LABEL org.opencontainers.image.licenses=MIT

RUN apk --update add ca-certificates

WORKDIR /webdav/

COPY ./go.mod ./
COPY ./go.sum ./
RUN go mod download

COPY . /webdav/
RUN go build -o main .

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /webdav/main /bin/webdav

EXPOSE 80

ENTRYPOINT [ "webdav", "-p", "80" ]
