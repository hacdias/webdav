FROM golang:1.22-alpine3.20 AS build

ARG VERSION="untracked"

RUN apk --update add ca-certificates

WORKDIR /webdav/

COPY ./go.mod ./
COPY ./go.sum ./
RUN go mod download

COPY . /webdav/
RUN go build -o main -ldflags="-X 'github.com/hacdias/webdav/v4/cmd.version=$VERSION'" .

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /webdav/main /bin/webdav

EXPOSE 6065

ENTRYPOINT [ "webdav" ]
