FROM alpine:latest as certs
RUN apk --update add ca-certificates

FROM golang:alpine as build
WORKDIR /app/

COPY go.mod .
COPY go.sum .
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -v -a

FROM scratch
COPY --from=certs /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/ca-certificates.crt
COPY --from=build /app/webdav /webdav

EXPOSE 80

ENTRYPOINT [ "/webdav" ]
