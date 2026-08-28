package lib

import (
	"context"
	"os"
	"path"
	"strings"

	"golang.org/x/net/webdav"
)

// hiddenFS wraps a [webdav.FileSystem] and omits entries whose base name matches
// one of the configured patterns from directory listings. The files themselves
// are left untouched and can still be accessed directly by their exact path,
// they are only removed from the parent collection's listing.
type hiddenFS struct {
	webdav.FileSystem
	patterns []string
}

func (h hiddenFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	file, err := h.FileSystem.OpenFile(ctx, name, flag, perm)
	if err != nil {
		return nil, err
	}

	return hiddenFile{File: file, patterns: h.patterns}, nil
}

type hiddenFile struct {
	webdav.File
	patterns []string
}

func (f hiddenFile) Readdir(count int) ([]os.FileInfo, error) {
	fis, err := f.File.Readdir(count)
	if err != nil {
		return nil, err
	}

	filtered := make([]os.FileInfo, 0, len(fis))
	for _, fi := range fis {
		if matchHidden(f.patterns, fi.Name()) {
			continue
		}
		filtered = append(filtered, fi)
	}
	return filtered, nil
}

// matchHidden reports whether name matches any of the patterns. Matching is case
// insensitive and uses [path.Match] against the base name, so both plain names
// like "Thumbs.db" and globs like "*.tmp" work. Patterns are validated when the
// configuration is parsed, so a bad pattern here is simply treated as no match.
func matchHidden(patterns []string, name string) bool {
	name = strings.ToLower(name)
	for _, pattern := range patterns {
		if ok, err := path.Match(strings.ToLower(pattern), name); err == nil && ok {
			return true
		}
	}
	return false
}
