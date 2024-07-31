package lib

import (
	"errors"
	"fmt"
	"net/http"
	"path/filepath"
	"regexp"
	"strings"
)

type Rule struct {
	Permissions Permissions
	Path        string
	Regex       *regexp.Regexp
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

type UserPermissions struct {
	Directory   string
	Permissions Permissions
	Rules       []*Rule
}

// Allowed checks if the user has permission to access a directory/file
func (p UserPermissions) Allowed(r *http.Request, destinationExists func(string) bool) bool {
	// Go through rules beginning from the last one.
	for i := len(p.Rules) - 1; i >= 0; i-- {
		rule := p.Rules[i]

		if rule.Matches(r.URL.Path) {
			return rule.Permissions.Allowed(r, destinationExists)
		}
	}

	return p.Permissions.Allowed(r, destinationExists)
}

func (p *UserPermissions) Validate() error {
	var err error

	p.Directory, err = filepath.Abs(p.Directory)
	if err != nil {
		return fmt.Errorf("invalid permissions: %w", err)
	}

	for _, r := range p.Rules {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("invalid permissions: %w", err)
		}
	}

	return nil
}

type Permissions struct {
	Create bool
	Read   bool
	Update bool
	Delete bool
}

func (p *Permissions) UnmarshalText(data []byte) error {
	text := strings.ToLower(string(data))
	if text == "none" {
		return nil
	}

	for _, c := range text {
		switch c {
		case 'c':
			p.Create = true
		case 'r':
			p.Read = true
		case 'u':
			p.Update = true
		case 'd':
			p.Delete = true
		default:
			return fmt.Errorf("invalid permission: %q", c)
		}
	}

	return nil
}

func (p Permissions) Allowed(r *http.Request, destinationExists func(string) bool) bool {
	switch r.Method {
	case "GET", "HEAD", "OPTIONS", "POST", "PROPFIND":
		// Note: POST backend implementation just returns the same thing as GET.
		return p.Read
	case "MKCOL":
		return p.Create
	case "PROPPATCH":
		return p.Update
	case "PUT":
		if destinationExists(r.URL.Path) {
			return p.Update
		} else {
			return p.Create
		}
	case "COPY", "MOVE":
		if destinationExists(r.Header.Get("Destination")) {
			return p.Update
		} else {
			return p.Create
		}
	case "DELETE":
		return p.Delete
	case "LOCK", "UNLOCK":
		return p.Create || p.Read || p.Update || p.Delete
	default:
		return false
	}
}
