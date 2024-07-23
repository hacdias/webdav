package lib

import (
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
)

var readMethods = []string{
	http.MethodGet,
	http.MethodHead,
	http.MethodOptions,
	"PROPFIND",
}

type Rule struct {
	Allow  bool
	Modify bool
	Path   string
	Regex  *regexp.Regexp
}

func (r *Rule) Validate() error {
	if r.Regex != nil && r.Path != "" {
		return errors.New("invalid rule: cannot define both regex and path")
	}

	return nil
}

// Matches checks if [Rule] matches the given path.
func (r *Rule) Matches(path string) bool {
	if r.Regex != nil {
		return r.Regex.MatchString(path)
	}

	return strings.HasPrefix(path, r.Path)
}

type Permissions struct {
	Scope  string
	Modify bool
	Rules  []*Rule
}

// Allowed checks if the user has permission to access a directory/file
func (p Permissions) Allowed(r *http.Request) bool {
	// Determine whether or not it is a read or write request.
	readRequest := false
	for _, method := range readMethods {
		if r.Method == method {
			readRequest = true
			break
		}
	}

	// Go through rules beginning from the last one.
	for i := len(p.Rules) - 1; i >= 0; i-- {
		rule := p.Rules[i]

		if rule.Matches(r.URL.Path) {
			return rule.Allow && (readRequest || rule.Modify)
		}
	}

	return readRequest || p.Modify
}

func (p *Permissions) Validate() error {
	for _, r := range p.Rules {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("invalid permissions: %w", err)
		}
	}

	return nil
}
