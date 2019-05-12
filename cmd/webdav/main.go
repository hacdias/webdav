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

		// load username from environment when prefix {env} is added
		if strings.HasPrefix(username, "{env}") {
			var envUsername = strings.TrimPrefix(username, "{env}")
			if envUsername == "" {
				log.Fatal("no environment variable specified for username")
			}
			username = os.Getenv(envUsername)
			if username == "" {
				log.Fatal("username must be set in environment")
			}
		}

		password, ok := r["password"].(string)
		if !ok {
			password = ""
		}

		// load password from environment when prefix {env} is added
		if strings.HasPrefix(password, "{env}") {
			var envPassword = strings.TrimPrefix(password, "{env}")
			if envPassword == "" {
				log.Fatal("no environment variable specified for password")
			}
			password = os.Getenv(envPassword)
			if password == "" {
				log.Fatal("password must be set in environment")
			}
		}

		user := &webdav.User{
			Username: username,
			Password: password,
			Scope:    c.webdav.User.Scope,
			Modify:   c.webdav.User.Modify,
			Rules:    c.webdav.User.Rules,
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
}

func parseConfig() *cfg {
	file := getConfig()

	data := struct {
		Address string                   `json:"address" yaml:"address"`
		Port    string                   `json:"port" yaml:"port"`
		TLS     bool                     `json:"tls" yaml:"tls"`
		Cert    string                   `json:"cert" yaml:"cert"`
		Auth    bool                     `json:"auth" yaml:"auth"`
		Key     string                   `json:"key" yaml:"key"`
		Scope   string                   `json:"scope" yaml:"scope"`
		Modify  bool                     `json:"modify" yaml:"modify"`
		Rules   []map[string]interface{} `json:"rules" yaml:"rules"`
		Users   []map[string]interface{} `json:"users" yaml:"users"`
	}{
		Address: "0.0.0.0",
		Port:    "0",
		TLS:     false,
		Cert:    "cert.pem",
		Key:     "key.pem",
		Scope:   "./",
		Auth:    true,
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
		tls:     data.TLS,
		cert:    data.Cert,
		key:     data.Key,
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
			Auth:  data.Auth,
			Users: map[string]*webdav.User{},
		},
	}

	if len(data.Users) != 0 && !data.Auth {
		log.Print("Users will be ignored due to auth=false")
	}

	if len(data.Rules) != 0 {
		config.webdav.User.Rules = parseRules(data.Rules)
	}

	parseUsers(data.Users, config)
	return config
}

func main() {
	flag.Parse()
	cfg := parseConfig()

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
		if err := http.ServeTLS(listener, cfg.webdav, cfg.cert, cfg.key); err != nil {
			log.Fatal(err)
		}
	} else {
		if err := http.Serve(listener, cfg.webdav); err != nil {
			log.Fatal(err)
		}

	}
}
