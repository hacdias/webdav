package lib

import (
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
	Regex  bool
	Allow  bool
	Modify bool
	Path   string
	// TODO: remove Regex and replace by this. It encodes
	Regexp *regexp.Regexp `mapstructure:"-"`
}

func (r *Rule) Validate() error {
	if r.Regex {
		rp, err := regexp.Compile(r.Path)
		if err != nil {
			return fmt.Errorf("invalid rule: %w", err)
		}
		r.Regexp = rp
		r.Path = ""
		r.Regex = false
	}

	return nil
}

// Matches checks if [Rule] matches the given path.
func (r *Rule) Matches(path string) bool {
	if r.Regexp != nil {
		return r.Regexp.MatchString(path)
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
