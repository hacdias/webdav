package lib

import (
	"errors"
	"fmt"
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
	if r.Regex == nil && r.Path == "" {
		return errors.New("invalid rule: must either define a path of a regex")
	}

	if r.Regex != nil && r.Path != "" {
		return errors.New("invalid rule: cannot define both regex and path")
	}

	return nil
}

// Matches checks if [Rule] matches the given path. When caseInsensitive is set
// the backing file system ignores case, so this must too. A regex is tried
// against the folded path as well as the path as written, which only widens it
// to spellings naming the same file.
func (r *Rule) Matches(path string, caseInsensitive bool) bool {
	if r.Regex != nil {
		if caseInsensitive {
			return r.Regex.MatchString(path) || r.Regex.MatchString(foldPath(path))
		}

		return r.Regex.MatchString(path)
	}

	if caseInsensitive {
		return strings.HasPrefix(foldPath(path), foldPath(r.Path))
	}

	return strings.HasPrefix(path, r.Path)
}

// matchesCollection checks if [Rule] names path as the collection it governs,
// such as a rule for "/c/" and a request for "/c". Regex rules are matched
// literally and are not considered here.
func (r *Rule) matchesCollection(path string, caseInsensitive bool) bool {
	if r.Regex != nil || !strings.HasSuffix(r.Path, "/") {
		return false
	}

	if caseInsensitive {
		return foldPath(path) == foldPath(strings.TrimSuffix(r.Path, "/"))
	}

	return path == strings.TrimSuffix(r.Path, "/")
}

type RulesBehavior string

const (
	RulesOverwrite RulesBehavior = "overwrite"
	RulesAppend    RulesBehavior = "append"
)

type UserPermissions struct {
	Directory     string
	Directories   DirectoryMounts
	Permissions   Permissions
	Rules         []*Rule
	RulesBehavior RulesBehavior

	directoryExplicit   bool
	directoriesExplicit bool
	useDirectories      bool
	caseInsensitive     bool
}

type DirectoryMount struct {
	Name string
	Path string
}

type DirectoryMounts []DirectoryMount

// Allowed checks if the user has permission to access a directory/file
func (p UserPermissions) Allowed(r *request, fileExists func(string) bool) bool {
	// For COPY and MOVE requests, we first check the permissions for the destination
	// path. As soon as a rule matches and does not allow the operation at the destination,
	// we fail immediately. If no rule matches, we check the global permissions.
	if r.method == "COPY" || r.method == "MOVE" {
		if !p.allowedAt(r.destination, func(perms Permissions) bool {
			return perms.AllowedDestination(r, fileExists)
		}) {
			return false
		}
	}

	return p.allowedAt(r.path, func(perms Permissions) bool {
		return perms.Allowed(r, fileExists)
	})
}

// allowedAt resolves the permissions that govern path and applies check to them.
func (p UserPermissions) allowedAt(path string, check func(Permissions) bool) bool {
	// Go through rules beginning from the last one. The first matched rule returns.
	// Both senses of matching are tested per rule: in separate passes a broader
	// rule would return first and shadow the narrower one naming the collection.
	for i := len(p.Rules) - 1; i >= 0; i-- {
		if p.Rules[i].Matches(path, p.caseInsensitive) {
			return check(p.Rules[i].Permissions)
		}

		// A rule written with a trailing slash also governs the collection it names,
		// so a rule for "/c/" cannot be evaded by asking for "/c". Such a request acts
		// on an entry of the parent collection, so it needs those permissions too: the
		// rule can restrict the collection, not grant access that would not exist.
		if p.Rules[i].matchesCollection(path, p.caseInsensitive) {
			if !check(p.Rules[i].Permissions) {
				return false
			}

			// Through the rules, not the global permissions alone, which would deny
			// a collection that an enclosing rule grants.
			if parent := parentCollection(path); parent != path {
				return p.allowedAt(parent, check)
			}

			return check(p.Permissions)
		}
	}

	return check(p.Permissions)
}

// parentCollection returns the collection containing path, such as "/data/" for
// "/data/sub", or "/" for a top-level entry. That bounds allowedAt at the root.
func parentCollection(p string) string {
	i := strings.LastIndex(strings.TrimSuffix(p, "/"), "/")
	if i <= 0 {
		return "/"
	}

	return p[:i+1]
}

func (p *UserPermissions) Validate() error {
	var err error

	p.Directory, err = filepath.Abs(p.Directory)
	if err != nil {
		return fmt.Errorf("invalid permissions: %w", err)
	}

	if p.useDirectories || len(p.Directories) > 0 {
		if err := (&p.Directories).Validate(); err != nil {
			return fmt.Errorf("invalid permissions: %w", err)
		}
	}

	p.caseInsensitive = p.hasCaseInsensitiveBacking()

	for _, r := range p.Rules {
		if err := r.Validate(); err != nil {
			return fmt.Errorf("invalid permissions: %w", err)
		}
	}

	switch p.RulesBehavior {
	case RulesAppend, RulesOverwrite:
		// Good to go
	default:
		return fmt.Errorf("invalid rule behavior: %s", p.RulesBehavior)
	}

	return nil
}

// hasCaseInsensitiveBacking reports whether any backing directory resolves names
// regardless of case. Mounts spread over volumes that differ all fold, which
// keeps deny rules effective on the case-insensitive ones.
func (p *UserPermissions) hasCaseInsensitiveBacking() bool {
	if p.useDirectories || len(p.Directories) > 0 {
		for _, mount := range p.Directories {
			if caseInsensitiveFS(mount.Path) {
				return true
			}
		}

		return false
	}

	return caseInsensitiveFS(p.Directory)
}

func (d *DirectoryMounts) Validate() error {
	names := map[string]struct{}{}

	for i := range *d {
		mount := &(*d)[i]
		if mount.Path == "" {
			return errors.New("invalid directories: path must be defined")
		}

		path, err := filepath.Abs(mount.Path)
		if err != nil {
			return fmt.Errorf("invalid directories: %w", err)
		}
		mount.Path = path

		if mount.Name == "" {
			mount.Name = filepath.Base(path)
		}

		if !validDirectoryMountName(mount.Name) {
			return fmt.Errorf("invalid directories: invalid mount name %q", mount.Name)
		}

		if _, ok := names[mount.Name]; ok {
			return fmt.Errorf("invalid directories: duplicate mount name %q", mount.Name)
		}
		names[mount.Name] = struct{}{}
	}

	return nil
}

func validDirectoryMountName(name string) bool {
	if name == "" || name == "." || name == ".." {
		return false
	}

	return !strings.ContainsAny(name, `/\`)
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

// Allowed returns whether this permission set has permissions to execute this
// request in the source directory. This applies to all requests with all methods.
func (p Permissions) Allowed(r *request, fileExists func(string) bool) bool {
	switch r.method {
	case "GET", "HEAD", "OPTIONS", "POST", "PROPFIND":
		// Note: POST backend implementation just returns the same thing as GET.
		return p.Read
	case "MKCOL":
		return p.Create
	case "PROPPATCH":
		return p.Update
	case "PUT", "PATCH":
		if fileExists(r.path) {
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
	case "LOCK":
		// A lock is write-class: it reserves the resource against other writers,
		// and locking a path that does not exist creates it.
		if fileExists(r.path) {
			return p.Update
		} else {
			return p.Create
		}
	case "UNLOCK":
		return p.Create || p.Update
	default:
		return false
	}
}

// AllowedDestination returns whether this permissions set has permissions to execute this
// request in the destination directory. This only applies for COPY and MOVE requests.
func (p Permissions) AllowedDestination(r *request, fileExists func(string) bool) bool {
	switch r.method {
	case "COPY", "MOVE":
		if fileExists(r.destination) {
			return p.Update
		} else {
			return p.Create
		}
	default:
		return false
	}
}
