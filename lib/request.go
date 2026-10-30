package lib

import (
	"errors"
	"net/http"
	"net/url"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"slices"
	"strings"
)

// cleanPath resolves dot segments so that the permission checks see the same
// path that the backing file system will ultimately open. The file systems in
// golang.org/x/net/webdav apply path.Clean before joining the backing
// directory, so without this the two layers disagree on which file a request
// names and a rule can be bypassed with e.g. "/public/../secret/file.txt".
func cleanPath(p string) string {
	if !strings.HasPrefix(p, "/") {
		p = "/" + p
	}

	cleaned := path.Clean(p)

	// path.Clean drops the trailing slash, but rules are prefix matches and are
	// commonly written with one, such as "/c/". Dropping it would stop a request
	// for the collection itself from matching the rule that names it.
	if cleaned != "/" && isCollectionPath(p) {
		cleaned += "/"
	}

	return cleaned
}

// isCollectionPath reports whether p names a collection rather than a resource
// within it. Besides an explicit trailing slash, a trailing "." or ".." segment
// also resolves to the collection itself.
func isCollectionPath(p string) bool {
	return strings.HasSuffix(p, "/") || strings.HasSuffix(p, "/.") || strings.HasSuffix(p, "/..")
}

// localPath reports whether p names the file the rules see. Some operating
// systems rewrite names before opening them, so a rule written for the real
// name would not cover the request. Those are refused rather than resolved.
func localPath(p string) bool {
	name := strings.Trim(p, "/")
	if name == "" || runtime.GOOS != "windows" {
		return true
	}

	// Rejects "\", ":" (alternate data streams), NUL, device names such as
	// "CON", and invalid UTF-8, which is converted to UTF-16 lossily.
	if _, err := filepath.Localize(name); err != nil {
		return false
	}

	return !slices.ContainsFunc(strings.Split(name, "/"), windowsAlias)
}

// shortName matches the 8.3 names Windows generates for long ones, such as
// "SUPERS~1" or "CREDEN~1.TXT".
var shortName = regexp.MustCompile(`^[^.]*~[0-9]+(\.|$)`)

// windowsAlias reports whether Windows opens segment under another name:
// trailing dots and spaces are stripped, and a short name opens the long one.
func windowsAlias(segment string) bool {
	return strings.HasSuffix(segment, ".") ||
		strings.HasSuffix(segment, " ") ||
		shortName.MatchString(segment)
}

type request struct {
	method      string
	path        string
	destination string
}

func newRequest(r *http.Request, prefix string) (*request, error) {
	ctx := &request{
		method: r.Method,
	}

	if destination := r.Header.Get("Destination"); destination != "" {
		u, err := url.Parse(destination)
		if err != nil {
			return nil, errors.New("invalid destination header")
		}

		// RFC 4918, section 10.3, has Destination as an absolute URI, which is
		// what clients send in practice. Only the path is relevant here, and
		// taking it unconditionally keeps the host out of the matched value.
		destination, err = stripPrefix(u.Path, prefix)
		if err != nil {
			return nil, err
		}

		ctx.destination = cleanPath(destination)
		if !localPath(ctx.destination) {
			return nil, errors.New("invalid destination")
		}
	}

	path, err := stripPrefix(r.URL.Path, prefix)
	if err != nil {
		return nil, err
	}

	ctx.path = cleanPath(path)
	if !localPath(ctx.path) {
		return nil, errors.New("invalid path")
	}

	return ctx, nil
}

// stripPrefix removes prefix from p, failing when p does not start with it.
func stripPrefix(p, prefix string) (string, error) {
	if prefix == "" {
		return p, nil
	}
	if stripped := strings.TrimPrefix(p, prefix); len(stripped) < len(p) {
		return stripped, nil
	}
	return "", errors.New("invalid url prefix")
}
