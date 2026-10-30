FROM golang:1.26-alpine3.22 AS build

ARG VERSION="untracked"

WORKDIR /webdav/

COPY ./go.mod ./
COPY ./go.sum ./
RUN go mod download

RUN mkdir -p -m 1777 /rootfs/tmp

COPY . /webdav/
RUN go build -o main -trimpath -ldflags="-s -w -X 'github.com/hacdias/webdav/v5/cmd.version=$VERSION'" .

FROM scratch

# Partial updates spool request bodies to the temporary directory.
COPY --from=build /rootfs/ /
COPY --from=build /webdav/main /bin/webdav

EXPOSE 6065

ENTRYPOINT [ "webdav" ]
