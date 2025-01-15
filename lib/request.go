package lib

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
)

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

		if prefix != "" {
			destination = strings.TrimPrefix(u.Path, prefix)
			if len(destination) >= len(u.Path) {
				return nil, errors.New("invalid url prefix")
			}
		}

		if !strings.HasPrefix(destination, "/") {
			destination = "/" + destination
		}

		ctx.destination = destination
	}

	path := r.URL.Path

	if prefix != "" {
		path = strings.TrimPrefix(r.URL.Path, prefix)
		if len(path) >= len(r.URL.Path) {
			return nil, errors.New("invalid url prefix")
		}
	}

	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	ctx.path = path

	return ctx, nil
}
