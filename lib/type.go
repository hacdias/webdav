package lib

import (
	"regexp"
	webdav "webdav/lib_official_webdav"
)

// Rule is a dissalow/allow rule.
type Rule struct {
	Regex   bool
	Allow_r bool
	Allow_w bool
	Path    string
	Regexp *regexp.Regexp
}

type Scope struct {
	Root string
	Allow_w bool
	Rules    []*Rule
	Handler  *webdav.Handler
}

// User contains the settings of each user.
type User struct {
	Username string
	Password string
	Scopes   map[string]*Scope
}
