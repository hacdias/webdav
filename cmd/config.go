package cmd

import (
	"errors"
	"github.com/spf13/pflag"
	v "github.com/spf13/viper"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"webdav/lib"
	webdav "webdav/lib_official_webdav"
)

func parseRules(raw []interface{}, scope *lib.Scope) []*lib.Rule {
	var rules []*lib.Rule

	for _, v := range raw {
		if r, ok := v.(map[interface{}]interface{}); ok {
			rule := &lib.Rule{
				Regex: false,
				Allow_r: true,
				Allow_w: scope.Allow_w,
				Path:  "",
			}

			if regex, ok := r["regex"].(bool); ok {
				rule.Regex = regex
			}

			if allow_r, ok := r["allow_r"].(bool); ok {
				rule.Allow_r = allow_r
			}

			if allow_w, ok := r["allow_w"].(bool); ok {
				rule.Allow_w = allow_w
				if allow_w {
					rule.Allow_r = true
				}
			}

			path, ok := r["path"].(string)
			if !ok {
				continue
			}
			path = strings.TrimPrefix(path, scope.Root)


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


func parseScopes(raw []interface{}, user *lib.User) map[string]*lib.Scope {
	scopes := make(map[string]*lib.Scope)

	for _, v := range raw {
		if s, ok := v.(map[interface{}]interface{}); ok {
			scope := &lib.Scope{
				Root: "",
				Allow_w: false,
				Rules: nil,
				Handler: nil,
			}

			if root, ok := s["root"].(string); ok {
				scope.Root = handlePathSeparator(root)
			}

			scope.Handler = &webdav.Handler{
				Prefix: "",
				FileSystem: lib.WebDavDir{
					Dir:     webdav.Dir(scope.Root),
					NoSniff: false,
				},
				LockSystem: webdav.NewMemLS(),
			}

			if alias, ok := s["alias"].(string); ok {
				if !strings.HasPrefix(alias, "/") {
					alias = strings.Join([]string{"/", alias}, "")
				}
				scope.Handler.Prefix = alias
			}

			scopes[scope.Handler.Prefix] = scope

			if allow_w, ok := s["allow_w"].(bool); ok {
				scope.Allow_w = allow_w
			}

			if rules, ok := s["rules"].([]interface{}); ok {
				scope.Rules = parseRules(rules, scope)
			}

		}
	}

	return scopes
}


func parseUsers(raw []interface{}, c *lib.Config) {
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
				if numPwd, ok := u["password"].(int); ok {
					password = strconv.Itoa(numPwd)
				}
			}

			if strings.HasPrefix(password, "{env}") {
				password, err = loadFromEnv(password)
				checkErr(err)
			}

			user := &lib.User{
				Username: username,
				Password: password,
				Scopes:   nil,
			}

			if scopes, ok := u["scopes"].([]interface{}); ok {
				user.Scopes = parseScopes(scopes, user)
			}

			c.Users[username] = user
		}
	}
}

func readConfig(flags *pflag.FlagSet) *lib.Config {
	cfg := &lib.Config{
		NoSniff: getOptB(flags, "nosniff"),
		Cors: lib.CorsCfg{
			Enabled:     false,
			Credentials: false,
		},
		Users: map[string]*lib.User{},
	}

	rawCors := v.Get("cors")
	if cors, ok := rawCors.(map[string]interface{}); ok {
		parseCors(cors, cfg)
	}

	rawUsers := v.Get("users")
	if users, ok := rawUsers.([]interface{}); ok {
		parseUsers(users, cfg)
	}

	return cfg
}


func parseCors(cfg map[string]interface{}, c *lib.Config) {
	cors := lib.CorsCfg{
		Enabled:     cfg["enabled"].(bool),
		Credentials: cfg["credentials"].(bool),
	}

	cors.AllowedHeaders = corsProperty("allowed_headers", cfg)
	cors.AllowedHosts = corsProperty("allowed_hosts", cfg)
	cors.AllowedMethods = corsProperty("allowed_methods", cfg)
	cors.ExposedHeaders = corsProperty("exposed_headers", cfg)

	c.Cors = cors
}

func corsProperty(property string, cfg map[string]interface{}) []string {
	var def []string

	if property == "exposed_headers" {
		def = []string{}
	} else {
		def = []string{"*"}
	}

	if allowed, ok := cfg[property].([]interface{}); ok {
		items := make([]string, len(allowed))

		for idx, a := range allowed {
			items[idx] = a.(string)
		}

		if len(items) == 0 {
			return def
		}

		return items
	}

	return def
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

/**
 * 分隔符'\\'转'/'，末尾不带分隔符
 */
func handlePathSeparator(path string) string {
	tmp := strings.Replace(path, "\\", "/", -1)
	for len(tmp) > 1 && strings.HasSuffix(tmp, "/") {
		tmp = strings.TrimSuffix(tmp, "/")
	}
	return tmp
}