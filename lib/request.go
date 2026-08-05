package lib

import (
	"errors"
	"net/http"
	"net/url"
	"path"
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
		destination = u.Path

		if prefix != "" {
			destination = strings.TrimPrefix(u.Path, prefix)
			if len(destination) >= len(u.Path) {
				return nil, errors.New("invalid url prefix")
			}
		}

		ctx.destination = cleanPath(destination)
	}

	path := r.URL.Path

	if prefix != "" {
		path = strings.TrimPrefix(r.URL.Path, prefix)
		if len(path) >= len(r.URL.Path) {
			return nil, errors.New("invalid url prefix")
		}
	}

	ctx.path = cleanPath(path)

	return ctx, nil
}
