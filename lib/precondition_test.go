package lib

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// headETag returns the current ETag of a resource, as a HEAD response reports it.
func headETag(t *testing.T, srvURL, path string) string {
	t.Helper()

	req, err := http.NewRequest(http.MethodHead, srvURL+path, nil)
	require.NoError(t, err)

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusOK, resp.StatusCode)

	etag := resp.Header.Get("ETag")
	require.NotEmpty(t, etag)

	return etag
}

// putStatus sends a PUT with the given headers and returns the status.
func putStatus(t *testing.T, srvURL, path string, headers map[string]string) int {
	t.Helper()

	status, _ := doRequest(t, http.MethodPut, srvURL+path, "", "", headers, "new-content")
	return status
}

// TestPreconditionSafeOverwrite covers "safe overwrite": If-None-Match: * has to
// refuse to create a resource that already exists.
func TestPreconditionSafeOverwrite(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{
		"foo.txt":     []byte("foo"),
		"sub/bar.txt": []byte("bar"),
	})

	srv := makeTestServer(t, "directory: "+dir+"\npermissions: CRUD")
	defer srv.Close()

	// The resource is missing, so the creation goes ahead.
	require.Equal(t, http.StatusCreated, putStatus(t, srv.URL, "/new.txt",
		map[string]string{"If-None-Match": "*"}))
	require.FileExists(t, filepath.Join(dir, "new.txt"))

	// The resource exists, so the upload is refused and nothing is written.
	require.Equal(t, http.StatusPreconditionFailed, putStatus(t, srv.URL, "/foo.txt",
		map[string]string{"If-None-Match": "*"}))

	data, err := os.ReadFile(filepath.Join(dir, "foo.txt")) //nolint:gosec // test directory
	require.NoError(t, err)
	require.Equal(t, "foo", string(data))

	// A collection that exists is refused the same way.
	require.Equal(t, http.StatusPreconditionFailed, putStatus(t, srv.URL, "/sub",
		map[string]string{"If-None-Match": "*"}))

	// Without the header an overwrite stays an overwrite.
	require.Equal(t, http.StatusCreated, putStatus(t, srv.URL, "/foo.txt", nil))

	data, err = os.ReadFile(filepath.Join(dir, "foo.txt")) //nolint:gosec // test directory
	require.NoError(t, err)
	require.Equal(t, "new-content", string(data))
}

// TestPreconditionIfMatch covers optimistic concurrency: the ETag in If-Match has
// to match the resource as it stands.
func TestPreconditionIfMatch(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{"foo.txt": []byte("foo")})

	srv := makeTestServer(t, "directory: "+dir+"\npermissions: CRUD")
	defer srv.Close()

	etag := headETag(t, srv.URL, "/foo.txt")

	// Contract test: etagOf has to agree with upstream findETag, or conditional
	// requests carrying an ETag are decided against a value clients never saw.
	fi, err := os.Stat(filepath.Join(dir, "foo.txt"))
	require.NoError(t, err)
	require.Equal(t, etag, etagOf(fi))

	require.Equal(t, http.StatusPreconditionFailed, putStatus(t, srv.URL, "/foo.txt",
		map[string]string{"If-Match": `"not-the-etag"`}))

	// If-Match also fails when there is nothing to match against.
	require.Equal(t, http.StatusPreconditionFailed, putStatus(t, srv.URL, "/missing.txt",
		map[string]string{"If-Match": `"whatever"`}))
	require.Equal(t, http.StatusPreconditionFailed, putStatus(t, srv.URL, "/missing.txt",
		map[string]string{"If-Match": "*"}))

	// The ETag as it stands allows the update.
	require.Equal(t, http.StatusCreated, putStatus(t, srv.URL, "/foo.txt",
		map[string]string{"If-Match": etag}))

	data, err := os.ReadFile(filepath.Join(dir, "foo.txt")) //nolint:gosec // test directory
	require.NoError(t, err)
	require.Equal(t, "new-content", string(data))

	// A weak validator satisfies the weak comparison only, which If-Match rejects.
	require.Equal(t, http.StatusPreconditionFailed, putStatus(t, srv.URL, "/foo.txt",
		map[string]string{"If-Match": "W/" + headETag(t, srv.URL, "/foo.txt")}))
}

// TestPreconditionIfNoneMatchETag covers If-None-Match carrying concrete ETags,
// which uses the weak comparison and a list of candidates.
func TestPreconditionIfNoneMatchETag(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{"foo.txt": []byte("foo")})

	srv := makeTestServer(t, "directory: "+dir+"\npermissions: CRUD")
	defer srv.Close()

	etag := headETag(t, srv.URL, "/foo.txt")

	require.Equal(t, http.StatusPreconditionFailed, putStatus(t, srv.URL, "/foo.txt",
		map[string]string{"If-None-Match": etag}))
	require.Equal(t, http.StatusPreconditionFailed, putStatus(t, srv.URL, "/foo.txt",
		map[string]string{"If-None-Match": `"another", W/` + etag}))

	// No candidate matches, so the overwrite goes ahead.
	require.Equal(t, http.StatusCreated, putStatus(t, srv.URL, "/foo.txt",
		map[string]string{"If-None-Match": `"another"`}))
}

// TestPreconditionIfUnmodifiedSince covers the time based precondition.
func TestPreconditionIfUnmodifiedSince(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{"foo.txt": []byte("foo")})

	srv := makeTestServer(t, "directory: "+dir+"\npermissions: CRUD")
	defer srv.Close()

	require.Equal(t, http.StatusPreconditionFailed, putStatus(t, srv.URL, "/foo.txt",
		map[string]string{"If-Unmodified-Since": "Wed, 01 Jan 2020 00:00:00 GMT"}))
	require.Equal(t, http.StatusCreated, putStatus(t, srv.URL, "/foo.txt",
		map[string]string{"If-Unmodified-Since": "Wed, 01 Jan 2030 00:00:00 GMT"}))
}

// TestPreconditionMalformed covers a header that cannot be parsed.
func TestPreconditionMalformed(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{"foo.txt": []byte("foo")})

	srv := makeTestServer(t, "directory: "+dir+"\npermissions: CRUD")
	defer srv.Close()

	require.Equal(t, http.StatusBadRequest, putStatus(t, srv.URL, "/foo.txt",
		map[string]string{"If-None-Match": `"unterminated`}))
}

// TestPreconditionIgnoresReads checks that reading is left to upstream, which
// answers those headers from ServeContent, and never turns into a 412 here.
func TestPreconditionIgnoresReads(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{"foo.txt": []byte("foo")})

	srv := makeTestServer(t, "directory: "+dir+"\npermissions: CRUD")
	defer srv.Close()

	status, _ := doRequest(t, http.MethodGet, srv.URL+"/foo.txt", "", "",
		map[string]string{"If-None-Match": "*"}, "")
	require.Equal(t, http.StatusNotModified, status)
}
