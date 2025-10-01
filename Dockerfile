FROM golang:1.25-alpine3.22 AS build

ARG VERSION="untracked"

RUN apk --update add ca-certificates curl

WORKDIR /webdav/

COPY ./go.mod ./
COPY ./go.sum ./
RUN go mod download

COPY . /webdav/
RUN go build -o main -trimpath -ldflags="-s -w -X 'github.com/hacdias/webdav/v5/cmd.version=$VERSION'" .

FROM alpine:3.22

RUN apk --no-cache add curl ca-certificates

COPY --from=build /webdav/main /bin/webdav

EXPOSE 6065

HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
  CMD curl -f http://localhost:6065/ || exit 1

ENTRYPOINT [ "webdav" ]
