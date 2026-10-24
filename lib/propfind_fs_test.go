package lib

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/webdav"
)

// countedFS counts the calls that reach the backing file system.
type countedFS struct {
	webdav.FileSystem
	stats, fileStats, opens int
	deniedName              string
}

func (f *countedFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	f.stats++
	return f.FileSystem.Stat(ctx, name)
}

func (f *countedFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	f.opens++
	if cleanPath(name) == f.deniedName {
		return nil, os.ErrPermission
	}

	file, err := f.FileSystem.OpenFile(ctx, name, flag, perm)
	if err != nil {
		return nil, err
	}

	return countedFile{File: file, fs: f}, nil
}

type countedFile struct {
	webdav.File
	fs *countedFS
}

func (f countedFile) Stat() (os.FileInfo, error) {
	f.fs.fileStats++
	return f.File.Stat()
}

func TestPropfindReusesDirectoryMetadata(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{
		"photo.arw":     []byte("RAW contents"),
		"sub/photo.jpg": []byte("jpeg"),
		"denied.arw":    []byte("private"),
	})

	cfg := writeAndParseConfig(t, "directory: "+dir+"\nnoSniff: true\nrules:\n  - path: /denied.arw\n    permissions: none", ".yml")
	require.NoError(t, cfg.Validate())

	h, err := NewHandler(cfg)
	require.NoError(t, err)

	handler := h.(*Handler)
	counter := &countedFS{FileSystem: handler.user.fs.fs}
	handler.user.fs.fs = counter
	handler.user.handler.FileSystem = handler.user.fs

	body := `<d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/><d:getcontentlength/><d:getlastmodified/></d:prop></d:propfind>`
	list := func() string {
		r := httptest.NewRequest("PROPFIND", "/", strings.NewReader(body))
		r.Header.Set("Depth", "infinity")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		require.Equal(t, http.StatusMultiStatus, w.Code)
		return w.Body.String()
	}

	first := list()
	require.Contains(t, first, "photo.arw")
	require.Contains(t, first, "sub/photo.jpg")
	require.NotContains(t, first, "denied.arw")

	// Only the requested collection is looked up, and every entry is still opened.
	require.Equal(t, 1, counter.stats)
	require.Equal(t, 1, counter.fileStats)
	require.Equal(t, 6, counter.opens)

	// Nothing is kept across requests.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "photo.arw"), []byte("changed"), 0666))
	require.Contains(t, list(), ">7<")

	// A failed open still hides the entry.
	counter.deniedName = "/photo.arw"
	require.NotContains(t, list(), "/photo.arw")
}

func TestPropfindMetadataMatchesUpstream(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{
		"photo & space.arw": []byte("RAW contents"),
		"sub/photo.jpg":     []byte("jpeg"),
	})
	_ = os.Symlink(filepath.Join(dir, "sub"), filepath.Join(dir, "linked"))

	for _, config := range []string{
		"directory: " + dir,
		"directories:\n  - path: " + dir + "\n    name: a\n  - path: " + filepath.Join(dir, "sub") + "\n    name: b",
	} {
		for _, noSniff := range []bool{false, true} {
			cfg := writeAndParseConfig(t, fmt.Sprintf("%s\nnoSniff: %t\nprefix: /dav", config, noSniff), ".yml")
			require.NoError(t, cfg.Validate())

			h, err := NewHandler(cfg)
			require.NoError(t, err)
			handler := h.(*Handler)

			for _, body := range []string{
				`<d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/></d:prop></d:propfind>`,
				`<d:propfind xmlns:d="DAV:"><d:prop><d:displayname/><d:resourcetype/><d:getcontentlength/><d:getlastmodified/><d:getcontenttype/><d:getetag/><d:unknown/></d:prop></d:propfind>`,
			} {
				for _, depth := range []string{"0", "1", "infinity"} {
					propfind := func(h http.Handler) *httptest.ResponseRecorder {
						r := httptest.NewRequest("PROPFIND", "/dav/", strings.NewReader(body))
						r.Header.Set("Depth", depth)
						w := httptest.NewRecorder()
						h.ServeHTTP(w, r)
						return w
					}

					upstream, actual := propfind(&handler.user.handler), propfind(handler)
					require.Equal(t, upstream.Code, actual.Code)
					require.Equal(t, upstream.Body.String(), actual.Body.String(), "%s noSniff=%t depth=%s", config, noSniff, depth)
				}
			}
		}
	}
}

func BenchmarkPropfindDirectory(b *testing.B) {
	dir := b.TempDir()
	for i := range 1000 {
		require.NoError(b, os.WriteFile(filepath.Join(dir, fmt.Sprintf("photo-%04d.arw", i)), []byte("RAW"), 0666))
	}

	body := `<d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/><d:getcontentlength/><d:getlastmodified/><d:getcontenttype/></d:prop></d:propfind>`

	for _, noSniff := range []bool{false, true} {
		cfg := &Config{Prefix: "/", NoSniff: noSniff, UserPermissions: UserPermissions{
			Directory: dir, Permissions: Permissions{Read: true}, RulesBehavior: RulesOverwrite,
		}}
		require.NoError(b, cfg.Validate())

		h, err := NewHandler(cfg)
		require.NoError(b, err)
		handler := h.(*Handler)

		for _, cached := range []bool{false, true} {
			var serve http.Handler = &handler.user.handler
			if cached {
				serve = handler
			}

			b.Run(fmt.Sprintf("noSniff=%t/cached=%t", noSniff, cached), func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					r := httptest.NewRequest("PROPFIND", "/", strings.NewReader(body))
					r.Header.Set("Depth", "1")
					w := httptest.NewRecorder()
					serve.ServeHTTP(w, r)
					require.Equal(b, http.StatusMultiStatus, w.Code)
				}
			})
		}
	}
}
