package cmd

import (
	"errors"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/hacdias/webdav/webdav"
	"github.com/spf13/pflag"
	v "github.com/spf13/viper"
	wd "golang.org/x/net/webdav"
)

func parseRules(raw []interface{}) []*webdav.Rule {
	rules := []*webdav.Rule{}

	for _, v := range raw {
		if r, ok := v.(map[interface{}]interface{}); ok {
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

			path, ok := r["path"].(string)
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
	}

	return rules
}

func loadFromEnv(v string) (string, error) {
	v = strings.TrimPrefix(v, "{env}")
	if v == "" {
		return "", errors.New("no environment variable specified")
	}

	v = os.Getenv(v)
	if v == "" {
		return "", errors.New("the environment variable is empty")
	}

	return v, nil
}

func parseUsers(raw []interface{}, c *webdav.Config) {
	var err error
	for _, v := range raw {
		if u, ok := v.(map[interface{}]interface{}); ok {
			username, ok := u["username"].(string)
			if !ok {
				log.Fatal("user needs an username")
			}

			if strings.HasPrefix(username, "{env}") {
				username, err = loadFromEnv(username)
				checkErr(err)
			}

			password, ok := u["password"].(string)
			if !ok {
				password = ""
			}

			if strings.HasPrefix(password, "{env}") {
				password, err = loadFromEnv(password)
				checkErr(err)
			}

			user := &webdav.User{
				Username: username,
				Password: password,
				Scope:    c.User.Scope,
				Modify:   c.User.Modify,
				Rules:    c.User.Rules,
			}

			if scope, ok := u["scope"].(string); ok {
				user.Scope = scope
			}

			if modify, ok := u["modify"].(bool); ok {
				user.Modify = modify
			}

			if rules, ok := u["rules"].([]interface{}); ok {
				user.Rules = parseRules(rules)
			}

			user.Handler = &wd.Handler{
				FileSystem: wd.Dir(user.Scope),
				LockSystem: wd.NewMemLS(),
			}

			c.Users[username] = user
		}
	}
}

func readConfig(flags *pflag.FlagSet) *webdav.Config {
	cfg := &webdav.Config{
		User: &webdav.User{
			Scope:  getOpt(flags, "scope"),
			Modify: getOptB(flags, "modify"),
			Rules:  []*webdav.Rule{},
			Handler: &wd.Handler{
				FileSystem: wd.Dir(getOpt(flags, "scope")),
				LockSystem: wd.NewMemLS(),
			},
		},
		Auth:  getOptB(flags, "auth"),
		Cors:  getOptB(flags, "cors"),
		Users: map[string]*webdav.User{},
	}

	rawRules := v.Get("rules")
	if rules, ok := rawRules.([]interface{}); ok {
		cfg.User.Rules = parseRules(rules)
	}

	rawUsers := v.Get("users")
	if users, ok := rawUsers.([]interface{}); ok {
		parseUsers(users, cfg)
	}

	if len(cfg.Users) != 0 && !cfg.Auth {
		log.Print("Users will be ignored due to auth=false")
	}

	return cfg
}
