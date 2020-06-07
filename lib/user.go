package lib

import (
	"regexp"
	"strings"

	// "golang.org/x/net/webdav"
	"github.com/hacdias/webdav/v3/webdav"
)

// Rule is a dissalow/allow rule.
type Rule struct {
	Regex  bool
	Allow  bool
	Path   string
	Regexp *regexp.Regexp
}

// User contains the settings of each user.
type User struct {
	Username string
	Password string
	Scope    string
	Modify   bool
	Rules    []*Rule
	Handler  *webdav.Handler
}

// Allowed checks if the user has permission to access a directory/file
func (u User) Allowed(url string) bool {
	var rule *Rule
	i := len(u.Rules) - 1

	for i >= 0 {
		rule = u.Rules[i]

		if rule.Regex {
			if rule.Regexp.MatchString(url) {
				return rule.Allow
			}
		} else if strings.HasPrefix(url, rule.Path) {
			return rule.Allow
		}

		i--
	}

	return true
}
