package lib

// Conditional request handling, as defined by RFC 9110 section 13.1.
//
// Upstream golang.org/x/net/webdav still leaves this to a TODO in handlePut:
//
//	// TODO(rost): Support the If-Match, If-None-Match headers? See bradfitz'
//	// comments in http.checkEtag.
//
// so a PUT ignores precondition headers entirely. What that looks like in
// practice:
//
//	PUT + If-None-Match: *           over an existing file -> 201 Created (should be 412)
//	PUT + If-Match: "<stale etag>"   over an existing file -> 201 Created (should be 412)
//
// Clients that implement "safe overwrite" (create-only, no clobbering) probe for
// this first: they write to an existing resource with If-None-Match: * and give
// up when the server happily overwrites it, reporting that the service does not
// support safe overwrites. This file supplies the missing check so such requests
// are answered with 412 before anything reaches the write path.

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/net/webdav"
)

// preconditionMethods lists the methods whose preconditions are evaluated here.
// For GET and HEAD the same headers are cache validation, which ServeContent
// already implements, so those stay with the upstream handler.
var preconditionMethods = map[string]struct{}{
	http.MethodPut:    {}, // also covers Content-Range partial uploads
	"PATCH":           {},
	http.MethodDelete: {},
	"MKCOL":           {},
	"PROPPATCH":       {},
	"COPY":            {}, // the conditions describe the request URI, not the destination
	"MOVE":            {}, // likewise
}

// resourceState is the current state of the request URI, as the condition
// expressions see it.
type resourceState struct {
	exists  bool
	etag    string
	modTime time.Time
}

// checkPreconditions evaluates the request's precondition headers in the order
// given by RFC 9110 section 13.2.2. It returns 0 when the request may proceed,
// and otherwise the status to answer with. reason is for logs only.
func checkPreconditions(r *http.Request, fs webdav.FileSystem, davPath string) (status int, reason string) {
	if _, ok := preconditionMethods[r.Method]; !ok {
		return 0, ""
	}

	ifMatch := r.Header.Get("If-Match")
	ifNoneMatch := r.Header.Get("If-None-Match")
	ifUnmodifiedSince := r.Header.Get("If-Unmodified-Since")
	if ifMatch == "" && ifNoneMatch == "" && ifUnmodifiedSince == "" {
		return 0, ""
	}

	st, err := statResource(r.Context(), fs, davPath)
	if err != nil {
		// Without the current state nothing can be evaluated, so leave the
		// request to upstream, which answers 404 or 403 on its own terms.
		return 0, ""
	}

	// If-Match comes first, and it makes If-Unmodified-Since irrelevant.
	if ifMatch != "" {
		tags, star, ok := parseETagList(ifMatch)
		if !ok {
			return http.StatusBadRequest, "malformed If-Match"
		}

		switch {
		case star && !st.exists:
			return http.StatusPreconditionFailed, "If-Match:* but resource is missing"
		case !star && !st.exists:
			return http.StatusPreconditionFailed, "If-Match but resource is missing"
		case !star && !anyMatch(tags, st.etag, strongMatch):
			return http.StatusPreconditionFailed, "If-Match etag mismatch"
		}
	} else if ifUnmodifiedSince != "" {
		// RFC 9110 section 13.1.4: the condition is ignored when the resource
		// has no Last-Modified.
		if t, err := http.ParseTime(ifUnmodifiedSince); err == nil && st.exists && !st.modTime.IsZero() {
			if st.modTime.Truncate(time.Second).After(t) {
				return http.StatusPreconditionFailed, "If-Unmodified-Since resource modified"
			}
		}
	}

	// If-None-Match is what "safe overwrite" and create-only uploads rely on.
	if ifNoneMatch != "" {
		tags, star, ok := parseETagList(ifNoneMatch)
		if !ok {
			return http.StatusBadRequest, "malformed If-None-Match"
		}

		switch {
		case star && st.exists:
			return http.StatusPreconditionFailed, "If-None-Match:* but resource exists"
		case !star && st.exists && anyMatch(tags, st.etag, weakMatch):
			return http.StatusPreconditionFailed, "If-None-Match etag matched"
		}
	}

	return 0, ""
}

// statResource reads the current state of the resource the request names.
func statResource(ctx context.Context, fs webdav.FileSystem, davPath string) (resourceState, error) {
	fi, err := fs.Stat(ctx, davPath)
	if err != nil {
		if os.IsNotExist(err) {
			// A missing resource is a legitimate state, and the one that lets
			// If-None-Match: * proceed.
			return resourceState{}, nil
		}

		return resourceState{}, err
	}

	return resourceState{
		exists:  true,
		etag:    etagOf(fi),
		modTime: fi.ModTime(),
	}, nil
}

// etagOf mirrors the heuristic upstream findETag uses: the modification time in
// nanoseconds followed by the size, both in hexadecimal. The order matters.
//
// It has to agree with upstream, or conditional requests carrying a concrete
// ETag are decided against a value no client ever saw. A contract test in
// precondition_test.go compares it with the ETag of a real HEAD response, so a
// change upstream fails the test rather than silently breaking these checks.
func etagOf(fi os.FileInfo) string {
	return fmt.Sprintf(`"%x%x"`, fi.ModTime().UnixNano(), fi.Size())
}

// parseETagList parses the value of If-Match or If-None-Match. star reports that
// the value was exactly "*", and tags holds the entity-tags as written, W/
// prefix included. ok is false for a malformed value, such as an unclosed quote.
func parseETagList(v string) (tags []string, star bool, ok bool) {
	s := strings.TrimSpace(v)
	if s == "*" {
		return nil, true, true
	}
	if s == "" {
		return nil, false, false
	}

	for i := 0; i < len(s); {
		// Skip the comma and whitespace separators between entity-tags.
		for i < len(s) && (s[i] == ',' || s[i] == ' ' || s[i] == '\t') {
			i++
		}
		if i >= len(s) {
			break
		}

		start := i
		if strings.HasPrefix(s[i:], "W/") {
			i += 2
		}
		if i >= len(s) || s[i] != '"' {
			return nil, false, false
		}

		i++ // opening quote
		for i < len(s) && s[i] != '"' {
			if s[i] == '\\' && i+1 < len(s) {
				i++ // skip the escaped character
			}
			i++
		}
		if i >= len(s) {
			return nil, false, false // unterminated quote
		}
		i++ // closing quote

		tags = append(tags, s[start:i])
	}

	return tags, false, true
}

// anyMatch reports whether one of the candidate entity-tags matches the current
// ETag, with the comparison given by cmp.
func anyMatch(tags []string, current string, cmp func(a, b string) bool) bool {
	if current == "" {
		return false
	}

	for _, tag := range tags {
		if cmp(tag, current) {
			return true
		}
	}

	return false
}

// isWeak reports whether tag is a weak validator, which is written with a W/ prefix.
func isWeak(tag string) bool { return strings.HasPrefix(tag, "W/") }

// strongMatch is the comparison If-Match asks for: neither side may be weak, and
// the two have to be byte for byte equal.
func strongMatch(a, b string) bool { return !isWeak(a) && !isWeak(b) && a == b }

// weakMatch is the comparison If-None-Match asks for: the W/ prefix is ignored
// and the two have to be byte for byte equal otherwise.
func weakMatch(a, b string) bool {
	return strings.TrimPrefix(a, "W/") == strings.TrimPrefix(b, "W/")
}
