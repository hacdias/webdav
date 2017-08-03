package main

import (
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
)

var (
	config         string
	defaultConfigs = []string{"config.json", "config.yaml", "config.yml"}
)

func init() {
	flag.StringVar(&config, "config", "", "Configuration file")
}

func basicAuth(c *cfg) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)

		username, password, authOK := r.BasicAuth()
		if authOK == false {
			http.Error(w, "Not authorized", 401)
			return
		}

		p, ok := c.auth[username]
		if !ok {
			http.Error(w, "Not authorized", 401)
			return
		}

		if password != p {
			http.Error(w, "Not authorized", 401)
			return
		}

		c.webdav.ServeHTTP(w, r)
	})
}

func main() {
	flag.Parse()
	cfg := parseConfig()
	handler := basicAuth(cfg)

	// Builds the address and a listener.
	laddr := ":" + cfg.port
	listener, err := net.Listen("tcp", laddr)
	if err != nil {
		log.Fatal(err)
	}

	// Tell the user the port in which is listening.
	fmt.Println("Listening on", listener.Addr().String())

	// Starts the server.
	if err := http.Serve(listener, handler); err != nil {
		log.Fatal(err)
	}
}
