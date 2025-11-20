package main

import (
	"github.com/hacdias/webdav/v5/cmd"

	_ "golang.org/x/crypto/x509roots/fallback"
)

func main() {
	cmd.Execute()
}
