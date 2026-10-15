package lib

import (
	"context"
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/webdav"
)

// Count actual filesystem calls beneath the permission and request wrappers.
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
	dir := makeTestDirectory(t, map[string][]byte{
		"photo.arw": []byte("RAW contents"), "sub/photo.jpg": []byte("jpeg"),
		"denied.arw": []byte("private"),
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
		r.Header.Set("Depth", "1")
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, r)
		require.Equal(t, 207, w.Code)
		return w.Body.String()
	}
	first := list()
	require.Contains(t, first, "photo.arw")
	require.Contains(t, first, "sub/")
	require.NotContains(t, first, "denied.arw")
	require.Equal(t, 1, counter.stats, "enumerated children must not be statted again")
	require.Equal(t, 0, counter.fileStats, "property lookup must reuse the same metadata")
	require.Equal(t, 4, counter.opens, "real file opens must still enforce filesystem access")

	// A later request must observe changed source metadata, never a session cache.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "photo.arw"), []byte("changed"), 0664))
	second := list()
	require.NotEqual(t, first, second)
	require.Contains(t, second, ">7<")

	// Even cached metadata must not bypass a failed filesystem open.
	counter.deniedName = "/photo.arw"
	require.NotContains(t, list(), "/photo.arw")
}

func TestPropfindMetadataMatchesUpstream(t *testing.T) {
	dir := makeTestDirectory(t, map[string][]byte{
		"photo & space.arw": []byte("RAW contents"), "sub/photo.jpg": []byte("jpeg"),
	})
	// Readdir gives link metadata, but WebDAV Stat follows its target.
	_ = os.Symlink(filepath.Join(dir, "sub"), filepath.Join(dir, "linked"))
	for _, noSniff := range []bool{false, true} {
		cfg := writeAndParseConfig(t, fmt.Sprintf("directory: %s\nnoSniff: %t\nprefix: /dav", dir, noSniff), ".yml")
		require.NoError(t, cfg.Validate())
		h, err := NewHandler(cfg)
		require.NoError(t, err)
		handler := h.(*Handler)
		for _, body := range []string{
			`<d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/></d:prop></d:propfind>`,
			`<d:propfind xmlns:d="DAV:"><d:prop><d:displayname/><d:resourcetype/><d:getcontentlength/><d:getlastmodified/><d:getcontenttype/><d:getetag/><d:unknown/></d:prop></d:propfind>`,
		} {
			for _, depth := range []string{"0", "1", "infinity"} {
				upstream := httptest.NewRecorder()
				r := httptest.NewRequest("PROPFIND", "/dav/", strings.NewReader(body))
				r.Header.Set("Depth", depth)
				handler.user.handler.ServeHTTP(upstream, r)
				actual := httptest.NewRecorder()
				r = httptest.NewRequest("PROPFIND", "/dav/", strings.NewReader(body))
				r.Header.Set("Depth", depth)
				handler.ServeHTTP(actual, r)
				require.Equal(t, upstream.Code, actual.Code)
				require.Equal(t, upstream.Body.String(), actual.Body.String(), "noSniff=%t depth=%s", noSniff, depth)
			}
		}
	}
}

func BenchmarkPropfindDirectory(b *testing.B) {
	dir := b.TempDir()
	for i := range 1000 {
		require.NoError(b, os.WriteFile(filepath.Join(dir, fmt.Sprintf("photo-%04d.arw", i)), []byte("RAW"), 0664))
	}
	cfg := &Config{Prefix: "/", NoSniff: true, UserPermissions: UserPermissions{
		Directory: dir, Permissions: Permissions{Read: true}, RulesBehavior: RulesOverwrite,
	}}
	require.NoError(b, cfg.Validate())
	h, err := NewHandler(cfg)
	require.NoError(b, err)
	handler := h.(*Handler)
	body := `<d:propfind xmlns:d="DAV:"><d:prop><d:resourcetype/><d:getcontentlength/><d:getlastmodified/></d:prop></d:propfind>`
	for _, name := range []string{"upstream", "cached"} {
		b.Run(name, func(b *testing.B) {
			b.ReportAllocs()
			for b.Loop() {
				r := httptest.NewRequest("PROPFIND", "/", strings.NewReader(body))
				r.Header.Set("Depth", "1")
				w := httptest.NewRecorder()
				if name == "upstream" {
					handler.user.handler.ServeHTTP(w, r)
				} else {
					handler.ServeHTTP(w, r)
				}
				require.Equal(b, 207, w.Code)
			}
		})
	}
}
