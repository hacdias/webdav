package lib

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
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
func (p UserPermissions) Allowed(r *http.Request, prefix string, fileExists func(string) bool) bool {

	// For COPY and MOVE requests, we first check the permissions for the destination
	// path. As soon as a rule matches and does not allow the operation at the destination,
	// we fail immediately. If no rule matches, we check the global permissions.
	if r.Method == "COPY" || r.Method == "MOVE" {
		u, err := url.Parse(r.Header.Get("Destination"))
		if err != nil {
			return false
		}
		dst := strings.TrimPrefix(u.Path, prefix)
		if !strings.HasPrefix(dst, "/") {
			dst = "/" + dst
		}

		fmt.Println(dst)

		for i := len(p.Rules) - 1; i >= 0; i-- {
			if p.Rules[i].Matches(dst) {
				if !p.Rules[i].Permissions.AllowedDestination(r.Method, dst, fileExists) {
					fmt.Println("disallowed", p.Rules[i].Path)
					return false
				}

				// Only check the first rule that matches, similarly to the source rules.
				break
			}
		}

		if !p.Permissions.AllowedDestination(r.Method, dst, fileExists) {
			fmt.Println("disallowed")
			return false
		}
	}

	// Go through rules beginning from the last one, and check the permissions at
	// the source. The first matched rule returns.
	for i := len(p.Rules) - 1; i >= 0; i-- {
		if p.Rules[i].Matches(r.URL.Path) {
			return p.Rules[i].Permissions.AllowedSource(r.Method, r.URL.Path, fileExists)
		}
	}

	return p.Permissions.AllowedSource(r.Method, r.URL.Path, fileExists)
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

// AllowedSource returns whether this permission set has permissions to execute this
// request in the source directory. This applies to all requests with all methods.
func (p Permissions) AllowedSource(method, filename string, fileExists func(string) bool) bool {
	switch method {
	case "GET", "HEAD", "OPTIONS", "POST", "PROPFIND":
		// Note: POST backend implementation just returns the same thing as GET.
		return p.Read
	case "MKCOL":
		return p.Create
	case "PROPPATCH":
		return p.Update
	case "PUT":
		if fileExists(filename) {
			return p.Update
		} else {
			return p.Create
		}
	case "COPY":
		return p.Read
	case "MOVE":
		return p.Read && p.Delete
	case "DELETE":
		return p.Delete
	case "LOCK", "UNLOCK":
		return p.Create || p.Read || p.Update || p.Delete
	default:
		return false
	}
}

// AllowedDestination returns whether this permissions set has permissions to execute this
// request in the destination directory. This only applies for COPY and MOVE requests.
func (p Permissions) AllowedDestination(method, filename string, fileExists func(string) bool) bool {
	switch method {
	case "COPY", "MOVE":
		if fileExists(filename) {
			return p.Update
		} else {
			return p.Create
		}
	default:
		return false
	}
}
