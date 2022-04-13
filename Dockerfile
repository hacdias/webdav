FROM golang:1.16.15-alpine3.15 as builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download
COPY . .

ENV CGO_ENABLED=0
RUN go build -ldflags '-extldflags "-static"' -o webdav

FROM alpine:latest as certs
RUN apk --update add ca-certificates

FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt

EXPOSE 80
COPY --from=builder /build/webdav /webdav

ENTRYPOINT [ "/webdav" ]
