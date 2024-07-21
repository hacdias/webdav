FROM golang:1.22-alpine3.20 as build

ARG DOCKER_META_VERSION="untracked"

RUN apk --update add ca-certificates

WORKDIR /webdav/

COPY ./go.mod ./
COPY ./go.sum ./
RUN go mod download

COPY . /webdav/
RUN go build -o main -ldflags="-X 'github.com/hacdias/webdav/v4/cmd.version=$DOCKER_META_VERSION'" .

FROM scratch

COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /webdav/main /bin/webdav

EXPOSE 80

ENTRYPOINT [ "webdav" ]
CMD [ "-p", "80" ]
