package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/hacdias/webdav"
	"golang.org/x/crypto/bcrypt"
	wd "golang.org/x/net/webdav"
	yaml "gopkg.in/yaml.v2"
)

var (
	config         string
	defaultConfigs = []string{
		"config.json",
		"config.yaml",
		"config.yml",
		"/etc/webdav/config.json",
		"/etc/webdav/config.yaml",
		"/etc/webdav/config.yml",
	}
)

func init() {
	flag.StringVar(&config, "config", "", "Configuration file")
}

func parseRules(raw []map[string]interface{}) []*webdav.Rule {
	rules := []*webdav.Rule{}

	for _, r := range raw {
		rule := &webdav.Rule{
			Regex: false,
			Allow: false,
			Path:  "",
		}

		if regex, ok := r["regex"].(bool); ok {
			rule.Regex = regex
		}

		if allow, ok := r["allow"].(bool); ok {
			rule.Allow = allow
		}

		path, ok := r["rule"].(string)
		if !ok {
			continue
		}

		if rule.Regex {
			rule.Regexp = regexp.MustCompile(path)
		} else {
			rule.Path = path
		}

		rules = append(rules, rule)
	}

	return rules
}

func parseUsers(raw []map[string]interface{}, c *cfg) {
	for _, r := range raw {
		username, ok := r["username"].(string)
		if !ok {
			log.Fatal("user needs an username")
		}

		password, ok := r["password"].(string)
		if !ok {
			log.Fatal("user needs a password")
		}

		c.auth[username] = password

		user := &webdav.User{
			Scope:  c.webdav.User.Scope,
			Modify: c.webdav.User.Modify,
			Rules:  c.webdav.User.Rules,
		}

		if scope, ok := r["scope"].(string); ok {
			user.Scope = scope
		}

		if modify, ok := r["modify"].(bool); ok {
			user.Modify = modify
		}

		if rules, ok := r["rules"].([]map[string]interface{}); ok {
			user.Rules = parseRules(rules)
		}

		user.Handler = &wd.Handler{
			FileSystem: wd.Dir(user.Scope),
			LockSystem: wd.NewMemLS(),
		}

		c.webdav.Users[username] = user
	}
}

func getConfig() []byte {
	if config == "" {
		for _, v := range defaultConfigs {
			_, err := os.Stat(v)
			if err == nil {
				config = v
				break
			}
		}
	}

	if config == "" {
		log.Fatal("no config file specified; couldn't find any config.{yaml,json}")
	}

	file, err := ioutil.ReadFile(config)
	if err != nil {
		log.Fatal(err)
	}

	return file
}

type cfg struct {
	webdav  *webdav.Config
	address string
	port    string
	tls     bool
	cert    string
	key     string
	auth    map[string]string
}

func parseConfig() *cfg {
	file := getConfig()

	data := struct {
		Address string                   `json:"address" yaml:"address"`
		Port    string                   `json:"port" yaml:"port"`
		Tls     bool                     `json:"tls" yaml:"tls"`
		Cert    string                   `json:"cert" yaml:"cert"`
		Key     string                   `json:"key" yaml:"key"`
		Scope   string                   `json:"scope" yaml:"scope"`
		Modify  bool                     `json:"modify" yaml:"modify"`
		Rules   []map[string]interface{} `json:"rules" yaml:"rules"`
		Users   []map[string]interface{} `json:"users" yaml:"users"`
	}{
		Address: "0.0.0.0",
		Port:    "0",
		Tls:     false,
		Cert:    "cert.pem",
		Key:     "key.pem",
		Scope:   "./",
		Modify:  true,
	}

	var err error
	if filepath.Ext(config) == ".json" {
		err = json.Unmarshal(file, &data)
	} else {
		err = yaml.Unmarshal(file, &data)
	}

	if err != nil {
		log.Fatal(err)
	}

	config := &cfg{
		address: data.Address,
		port:    data.Port,
		tls:     data.Tls,
		cert:    data.Cert,
		key:     data.Key,
		auth:    map[string]string{},
		webdav: &webdav.Config{
			User: &webdav.User{
				Scope:  data.Scope,
				Modify: data.Modify,
				Rules:  []*webdav.Rule{},
				Handler: &wd.Handler{
					FileSystem: wd.Dir(data.Scope),
					LockSystem: wd.NewMemLS(),
				},
			},
			Users: map[string]*webdav.User{},
		},
	}

	if len(data.Users) == 0 {
		log.Fatal("no user defined")
	}

	if len(data.Rules) != 0 {
		config.webdav.User.Rules = parseRules(data.Rules)
	}

	parseUsers(data.Users, config)
	return config
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

		if !checkPassword(p, password) {
			log.Println("Wrong Password for user", username)
			http.Error(w, "Not authorized", 401)
			return
		}

		c.webdav.ServeHTTP(w, r)
	})
}

func checkPassword(saved, input string) bool {

	if strings.HasPrefix(saved, "{bcrypt}") {
		savedPassword := strings.TrimPrefix(saved, "{bcrypt}")
		return bcrypt.CompareHashAndPassword([]byte(savedPassword), []byte(input)) == nil
	}

	return saved == input
}

func main() {
	flag.Parse()
	cfg := parseConfig()
	handler := basicAuth(cfg)

	// Builds the address and a listener.
	laddr := cfg.address + ":" + cfg.port
	listener, err := net.Listen("tcp", laddr)
	if err != nil {
		log.Fatal(err)
	}

	// Tell the user the port in which is listening.
	fmt.Println("Listening on", listener.Addr().String())

	// Starts the server.
	if cfg.tls {
		if err := http.ServeTLS(listener, handler, cfg.cert, cfg.key); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := http.Serve(listener, handler); err != nil {
			log.Fatal(err)
		}

	}
}
