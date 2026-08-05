package lib

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/studio-b12/gowebdav"
)

func makeTestDirectory(t *testing.T, m map[string][]byte) string {
	dir := t.TempDir()

	for path, data := range m {
		filename := filepath.Join(dir, path)

		if data == nil {
			err := os.MkdirAll(filename, 0775)
			require.NoError(t, err)
		} else {
			err := os.MkdirAll(filepath.Dir(filename), 0775)
			require.NoError(t, err)

			err = os.WriteFile(filename, data, 0664)
			require.NoError(t, err)
		}
	}

	return dir
}

func makeTestServer(t *testing.T, yamlConfig string) *httptest.Server {
	cfg := writeAndParseConfig(t, yamlConfig, ".yml")
	require.NoError(t, cfg.Validate())

	handler, err := NewHandler(cfg)
	require.NoError(t, err)

	return httptest.NewServer(handler)
}

func TestServerDefaults(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{
		"foo.txt":     []byte("foo"),
		"sub/bar.txt": []byte("bar"),
	})

	srv := makeTestServer(t, "directory: "+dir)
	client := gowebdav.NewClient(srv.URL, "", "")

	// By default, reading permissions.
	files, err := client.ReadDir("/")
	require.NoError(t, err)
	require.Len(t, files, 2)

	data, err := client.Read("/foo.txt")
	require.NoError(t, err)
	require.EqualValues(t, []byte("foo"), data)

	files, err = client.ReadDir("/sub")
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "bar.txt", files[0].Name())

	data, err = client.Read("/sub/bar.txt")
	require.NoError(t, err)
	require.EqualValues(t, []byte("bar"), data)

	// By default, no modification permissions.
	require.ErrorContains(t, client.Mkdir("/dir", 0666), "403")
	require.ErrorContains(t, client.MkdirAll("/dir/path", 0666), "403")
	require.ErrorContains(t, client.Remove("/foo.txt"), "403")
	require.ErrorContains(t, client.RemoveAll("/foo.txt"), "403")
	require.ErrorContains(t, client.Rename("/foo.txt", "/file2.txt", false), "403")
	require.ErrorContains(t, client.Copy("/foo.txt", "/file2.txt", false), "403")
	require.ErrorContains(t, client.Write("/foo.txt", []byte("hello world 2"), 0666), "403")
}

func TestServerPartialUpdateOptions(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{
		"foo.txt": []byte("hello world"),
	})
	srv := makeTestServer(t, "directory: "+dir+"\npermissions: CRUD")
	defer srv.Close()

	req, err := http.NewRequest(http.MethodOptions, srv.URL+"/foo.txt", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusOK, resp.StatusCode)
	require.Contains(t, resp.Header.Get("DAV"), "sabredav-partialupdate")
	require.Contains(t, resp.Header.Get("Allow"), "PATCH")
	require.Equal(t, partialUpdateContentType, resp.Header.Get("Accept-Patch"))
}

func TestServerPatchPartialUpdate(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		initialData string
		body        string
		updateRange string
		wantData    string
	}{{
		name:        "start",
		initialData: "hello world",
		body:        "DAV",
		updateRange: "bytes=6-",
		wantData:    "hello DAVld",
	}, {
		name:        "suffix",
		initialData: "hello world",
		body:        "DAV",
		updateRange: "bytes=-5",
		wantData:    "hello DAVld",
	}, {
		name:        "append",
		initialData: "hello",
		body:        " world",
		updateRange: "append",
		wantData:    "hello world",
	}, {
		name:        "suffix_zero",
		initialData: "hello",
		body:        " world",
		updateRange: "bytes=-0",
		wantData:    "hello world",
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := makeTestDirectory(t, map[string][]byte{
				"foo.txt": []byte(tc.initialData),
			})
			srv := makeTestServer(t, "directory: "+dir+"\npermissions: CRUD")
			defer srv.Close()

			req, err := http.NewRequest("PATCH", srv.URL+"/foo.txt", strings.NewReader(tc.body))
			require.NoError(t, err)
			req.Header.Set("Content-Type", partialUpdateContentType)
			req.Header.Set("X-Update-Range", tc.updateRange)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			require.Equal(t, http.StatusNoContent, resp.StatusCode)
			data, err := os.ReadFile(filepath.Join(dir, "foo.txt"))
			require.NoError(t, err)
			require.Equal(t, tc.wantData, string(data))
		})
	}
}

func TestServerPatchPartialUpdateCreatesSparseFile(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, nil)
	srv := makeTestServer(t, "directory: "+dir+"\npermissions: CRUD")
	defer srv.Close()

	req, err := http.NewRequest("PATCH", srv.URL+"/new.bin", strings.NewReader("x"))
	require.NoError(t, err)
	req.Header.Set("Content-Type", partialUpdateContentType)
	req.Header.Set("X-Update-Range", "bytes=3-")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()

	require.Equal(t, http.StatusCreated, resp.StatusCode)
	data, err := os.ReadFile(filepath.Join(dir, "new.bin"))
	require.NoError(t, err)
	require.Equal(t, []byte{0, 0, 0, 'x'}, data)
}

func TestServerPutContentRangePartialUpdate(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{
		"foo.txt": []byte("hello world"),
	})
	srv := makeTestServer(t, "directory: "+dir+"\npermissions: CRUD")
	defer srv.Close()

	req, err := http.NewRequest(http.MethodPut, srv.URL+"/foo.txt", strings.NewReader("DAV"))
	require.NoError(t, err)
	req.Header.Set("Content-Range", "bytes 6-8/*")
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, resp.Body)

	require.Equal(t, http.StatusNoContent, resp.StatusCode)
	data, err := os.ReadFile(filepath.Join(dir, "foo.txt"))
	require.NoError(t, err)
	require.Equal(t, "hello DAVld", string(data))
}

func TestServerPartialUpdateErrors(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name          string
		method        string
		body          string
		contentLength int64
		path          string
		headerName    string
		updateRange   string
		contentRange  string
		wantStatus    int
	}{{
		name:          "patch_missing_content_length",
		method:        "PATCH",
		body:          "DAV",
		contentLength: -1,
		updateRange:   "bytes=6-8",
		wantStatus:    http.StatusLengthRequired,
	}, {
		name:        "patch_invalid_range",
		method:      "PATCH",
		body:        "DAV",
		updateRange: "bytes=8-6",
		wantStatus:  http.StatusRequestedRangeNotSatisfiable,
	}, {
		name:        "patch_length_mismatch",
		method:      "PATCH",
		body:        "TOOLONG",
		updateRange: "bytes=6-8",
		wantStatus:  http.StatusRequestedRangeNotSatisfiable,
	}, {
		name:          "put_content_range_length_mismatch",
		method:        http.MethodPut,
		body:          "TOOLONG",
		contentLength: -1,
		contentRange:  "bytes 6-8/*",
		wantStatus:    http.StatusRequestedRangeNotSatisfiable,
	}, {
		name:        "if_none_match",
		method:      "PATCH",
		body:        "DAV",
		headerName:  "If-None-Match",
		updateRange: "bytes=0-2",
		wantStatus:  http.StatusPreconditionFailed,
	}, {
		name:        "if_match",
		method:      "PATCH",
		path:        "/missing.txt",
		body:        "DAV",
		headerName:  "If-Match",
		updateRange: "bytes=0-2",
		wantStatus:  http.StatusPreconditionFailed,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := makeTestDirectory(t, map[string][]byte{
				"foo.txt": []byte("hello world"),
			})
			srv := makeTestServer(t, "directory: "+dir+"\npermissions: CRUD")
			defer srv.Close()

			var body io.Reader = strings.NewReader(tc.body)
			if tc.contentLength < 0 {
				body = io.NopCloser(strings.NewReader(tc.body))
			}
			path := tc.path
			if path == "" {
				path = "/foo.txt"
			}
			req, err := http.NewRequest(tc.method, srv.URL+path, body)
			require.NoError(t, err)
			if tc.contentLength < 0 {
				req.ContentLength = tc.contentLength
			}
			if tc.method == "PATCH" {
				req.Header.Set("Content-Type", partialUpdateContentType)
				req.Header.Set("X-Update-Range", tc.updateRange)
			}
			if tc.contentRange != "" {
				req.Header.Set("Content-Range", tc.contentRange)
			}
			if tc.headerName != "" {
				req.Header.Set(tc.headerName, "*")
			}
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			require.Equal(t, tc.wantStatus, resp.StatusCode)
			data, err := os.ReadFile(filepath.Join(dir, "foo.txt"))
			require.NoError(t, err)
			require.Equal(t, "hello world", string(data))
		})
	}
}

func TestServerPartialUpdateETagPreconditions(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name         string
		method       string
		headerName   string
		headerValue  func(string) string
		contentRange string
		wantStatus   int
		wantData     string
	}{{
		name:        "if_match_matches",
		method:      "PATCH",
		headerName:  "If-Match",
		headerValue: func(etag string) string { return etag },
		wantStatus:  http.StatusNoContent,
		wantData:    "hello DAVld",
	}, {
		name:        "if_match_mismatch",
		method:      "PATCH",
		headerName:  "If-Match",
		headerValue: func(string) string { return `"definitely-wrong"` },
		wantStatus:  http.StatusPreconditionFailed,
		wantData:    "hello world",
	}, {
		name:        "if_match_list_matches",
		method:      "PATCH",
		headerName:  "If-Match",
		headerValue: func(etag string) string { return `"definitely-wrong", ` + etag },
		wantStatus:  http.StatusNoContent,
		wantData:    "hello DAVld",
	}, {
		name:        "if_none_match_matches",
		method:      "PATCH",
		headerName:  "If-None-Match",
		headerValue: func(etag string) string { return etag },
		wantStatus:  http.StatusPreconditionFailed,
		wantData:    "hello world",
	}, {
		name:        "if_none_match_mismatch",
		method:      "PATCH",
		headerName:  "If-None-Match",
		headerValue: func(string) string { return `"definitely-wrong"` },
		wantStatus:  http.StatusNoContent,
		wantData:    "hello DAVld",
	}, {
		name:         "put_content_range_if_match_mismatch",
		method:       http.MethodPut,
		headerName:   "If-Match",
		headerValue:  func(string) string { return `"definitely-wrong"` },
		contentRange: "bytes 6-8/*",
		wantStatus:   http.StatusPreconditionFailed,
		wantData:     "hello world",
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := makeTestDirectory(t, map[string][]byte{
				"foo.txt": []byte("hello world"),
			})
			srv := makeTestServer(t, "directory: "+dir+"\npermissions: CRUD")
			defer srv.Close()

			req, err := http.NewRequest(http.MethodHead, srv.URL+"/foo.txt", nil)
			require.NoError(t, err)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			_ = resp.Body.Close()
			require.Equal(t, http.StatusOK, resp.StatusCode)
			etag := resp.Header.Get("ETag")
			require.NotEmpty(t, etag)

			req, err = http.NewRequest(tc.method, srv.URL+"/foo.txt", strings.NewReader("DAV"))
			require.NoError(t, err)
			if tc.method == "PATCH" {
				req.Header.Set("Content-Type", partialUpdateContentType)
				req.Header.Set("X-Update-Range", "bytes=6-8")
			}
			if tc.contentRange != "" {
				req.Header.Set("Content-Range", tc.contentRange)
			}
			req.Header.Set(tc.headerName, tc.headerValue(etag))
			resp, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()

			require.Equal(t, tc.wantStatus, resp.StatusCode)
			data, err := os.ReadFile(filepath.Join(dir, "foo.txt"))
			require.NoError(t, err)
			require.Equal(t, tc.wantData, string(data))
		})
	}
}

func TestServerPartialUpdateHonorsLocks(t *testing.T) {
	t.Parallel()

	const createLockBody = `<?xml version="1.0" encoding="utf-8" ?>
		<D:lockinfo xmlns:D='DAV:'>
			<D:lockscope><D:exclusive/></D:lockscope>
			<D:locktype><D:write/></D:locktype>
			<D:owner>test</D:owner>
		</D:lockinfo>`

	testCases := []struct {
		name     string
		lockPath string
		depth    string
		ifPath   string
	}{{
		name:     "file",
		lockPath: "/foo.txt",
		depth:    "0",
		ifPath:   "/foo.txt",
	}, {
		name:     "root",
		lockPath: "/",
		depth:    "infinity",
		ifPath:   "/",
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			dir := makeTestDirectory(t, map[string][]byte{
				"foo.txt": []byte("hello world"),
			})
			srv := makeTestServer(t, "directory: "+dir+"\npermissions: CRUD")
			defer srv.Close()

			req, err := http.NewRequest("LOCK", srv.URL+tc.lockPath, strings.NewReader(createLockBody))
			require.NoError(t, err)
			req.Header.Set("Content-Type", "application/xml")
			req.Header.Set("Depth", tc.depth)
			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			_, _ = io.Copy(io.Discard, resp.Body)
			require.Equal(t, http.StatusOK, resp.StatusCode)
			lockToken := resp.Header.Get("Lock-Token")

			req, err = http.NewRequest("PATCH", srv.URL+"/foo.txt", strings.NewReader("DAV"))
			require.NoError(t, err)
			req.Header.Set("Content-Type", partialUpdateContentType)
			req.Header.Set("X-Update-Range", "bytes=6-8")
			resp, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			require.Equal(t, 423, resp.StatusCode)

			req, err = http.NewRequest("PATCH", srv.URL+"/foo.txt", strings.NewReader("DAV"))
			require.NoError(t, err)
			req.Header.Set("Content-Type", partialUpdateContentType)
			req.Header.Set("X-Update-Range", "bytes=6-8")
			req.Header.Set("If", fmt.Sprintf("<%s%s> (%s)", srv.URL, tc.ifPath, lockToken))
			resp, err = http.DefaultClient.Do(req)
			require.NoError(t, err)
			defer func() { _ = resp.Body.Close() }()
			require.Equal(t, http.StatusNoContent, resp.StatusCode)

			data, err := os.ReadFile(filepath.Join(dir, "foo.txt"))
			require.NoError(t, err)
			require.Equal(t, "hello DAVld", string(data))
		})
	}
}

func TestServerListingCharacters(t *testing.T) {
	t.Parallel()

	contents := map[string][]byte{
		"富/foo.txt": []byte("foo"),
		"你好.txt":    []byte("bar"),
		"foo.txt":   []byte("foo"),
		"🌹.txt":     []byte("foo"),
	}
	expectedNames := []string{"foo.txt", "你好.txt", "富", "🌹.txt"}
	if runtime.GOOS != "windows" {
		// Asterisks are invalid in Windows filenames.
		contents["z*.txt"] = []byte("zbar")
		expectedNames = append(expectedNames, "z*.txt")
	}
	sort.Strings(expectedNames)

	dir := makeTestDirectory(t, contents)

	srv := makeTestServer(t, "directory: "+dir)
	client := gowebdav.NewClient(srv.URL, "", "")

	// By default, reading permissions.
	files, err := client.ReadDir("/")
	require.NoError(t, err)
	require.Len(t, files, len(expectedNames))

	names := make([]string, len(files))
	for i, file := range files {
		names[i] = file.Name()
	}
	sort.Strings(names)

	require.Equal(t, expectedNames, names)

	if runtime.GOOS != "windows" {
		data, err := client.Read("/z*.txt")
		require.NoError(t, err)
		require.EqualValues(t, []byte("zbar"), data)
	}
}

func TestServerAuthentication(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{
		"foo.txt":     []byte("foo"),
		"sub/bar.txt": []byte("bar"),
	})

	srv := makeTestServer(t, fmt.Sprintf(`
directory: %s
permissions: CRUD

users:
  - username: basic
    password: basic
  - username: bcrypt
    password: "{bcrypt}$2a$12$222dfz8Nweoyvy8OwI8.me9nfaRfuz8lqGkiiYSMH1lLMHO26qWom"
`, dir))

	t.Run("Basic Auth (Plaintext)", func(t *testing.T) {
		t.Parallel()

		client := gowebdav.NewClient(srv.URL, "basic", "basic")

		files, err := client.ReadDir("/")
		require.NoError(t, err)
		require.Len(t, files, 2)
	})

	t.Run("Basic Auth (BCrypt)", func(t *testing.T) {
		t.Parallel()
		client := gowebdav.NewClient(srv.URL, "bcrypt", "bcrypt")

		files, err := client.ReadDir("/")
		require.NoError(t, err)
		require.Len(t, files, 2)
	})

	t.Run("Unauthorized (No Credentials)", func(t *testing.T) {
		t.Parallel()
		client := gowebdav.NewClient(srv.URL, "", "")
		_, err := client.ReadDir("/")
		require.ErrorContains(t, err, "401")
	})

	t.Run("Unauthorized (Wrong User)", func(t *testing.T) {
		t.Parallel()
		client := gowebdav.NewClient(srv.URL, "wrong", "basic")
		_, err := client.ReadDir("/")
		require.ErrorContains(t, err, "401")
	})

	t.Run("Unauthorized (Wrong Password)", func(t *testing.T) {
		t.Parallel()
		client := gowebdav.NewClient(srv.URL, "basic", "wrong")
		_, err := client.ReadDir("/")
		require.ErrorContains(t, err, "401")
	})
}

func TestServerAuthenticationNoPassword(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{
		"foo.txt":     []byte("foo"),
		"sub/bar.txt": []byte("bar"),
	})

	srv := makeTestServer(t, fmt.Sprintf(`
directory: %s
noPassword: true
permissions: CRUD

users:
  - username: basic
`, dir))

	t.Run("Basic Auth", func(t *testing.T) {
		t.Parallel()

		client := gowebdav.NewClient(srv.URL, "basic", "")
		files, err := client.ReadDir("/")
		require.NoError(t, err)
		require.Len(t, files, 2)
	})

	t.Run("Unauthorized Wrong User", func(t *testing.T) {
		t.Parallel()
		client := gowebdav.NewClient(srv.URL, "wrong", "")
		_, err := client.ReadDir("/")
		require.ErrorContains(t, err, "401")
	})
}

func TestServerRulesRestrictive(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{
		"foo.txt":   []byte("foo"),
		"bar.js":    []byte("foo js"),
		"a/foo.js":  []byte("foo js"),
		"a/foo.txt": []byte("foo txt"),
		"b/foo.txt": []byte("foo b"),
		"c/a.txt":   []byte("b"),
		"c/b.txt":   []byte("b"),
		"c/c.txt":   []byte("b"),
	})

	srv := makeTestServer(t, fmt.Sprintf(`
directory: %s
permissions: CRUD

users:
  - username: basic
    password: basic
    rules:
    - regex: "^.+.js$"
      permissions: R
    - path: "/b/"
      permissions: R
    - path: "/a/foo.txt"
      permissions: none
    - path: "/c/"
      permissions: none
`, dir))

	client := gowebdav.NewClient(srv.URL, "basic", "basic")

	files, err := client.ReadDir("/")
	require.NoError(t, err)
	require.Len(t, files, 5)

	err = client.Write("/foo.txt", []byte("new"), 0666)
	require.NoError(t, err)

	err = client.Write("/new.txt", []byte("new"), 0666)
	require.NoError(t, err)

	err = client.Copy("/bar.js", "/b/bar.js", false)
	require.ErrorContains(t, err, "403")

	err = client.Copy("/bar.js", "/bar.jsx", false)
	require.NoError(t, err)

	err = client.Copy("/b/foo.txt", "/foo1.txt", false)
	require.NoError(t, err)

	err = client.Rename("/b/foo.txt", "/foo2.txt", false)
	require.ErrorContains(t, err, "403")

	_, err = client.Read("/a/foo.txt")
	require.ErrorContains(t, err, "403")

	err = client.Write("/a/foo.js", []byte("new"), 0666)
	require.ErrorContains(t, err, "403")

	err = client.Write("/b/foo.txt", []byte("new"), 0666)
	require.ErrorContains(t, err, "403")

	_, err = client.ReadDir("/c")
	require.ErrorContains(t, err, "403")

	_, err = client.Read("/c/a.txt")
	require.ErrorContains(t, err, "403")

	err = client.Write("/c/b.txt", []byte("new"), 0666)
	require.ErrorContains(t, err, "403")

	err = client.MkdirAll("/d/foo/bar", 0666)
	require.NoError(t, err)

	err = client.Write("/d/foo/bar/test.txt", []byte("test"), 0666)
	require.NoError(t, err)
}

func TestServerRulesAdditive(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{
		"foo.txt":   []byte("foo"),
		"bar.js":    []byte("foo js"),
		"a/foo.js":  []byte("foo js"),
		"a/foo.txt": []byte("foo txt"),
		"b/foo.txt": []byte("foo b"),
	})

	srv := makeTestServer(t, fmt.Sprintf(`
directory: %s
permissions: none

users:
  - username: basic
    password: basic
    rules:
    - regex: "^.+.js$"
      permissions: R
    - path: "/a/foo.txt"
      permissions: CRU
    - path: "/b/"
      permissions: D
`, dir))

	client := gowebdav.NewClient(srv.URL, "basic", "basic")

	_, err := client.ReadDir("/")
	require.ErrorContains(t, err, "403")

	err = client.Write("/foo.txt", []byte("new"), 0666)
	require.ErrorContains(t, err, "403")

	err = client.Write("/new.txt", []byte("new"), 0666)
	require.ErrorContains(t, err, "403")

	err = client.Copy("/bar.js", "/a/foo.txt", true)
	require.NoError(t, err)

	err = client.Remove("/b/foo.txt")
	require.NoError(t, err)
}

func TestServerRulesPrefix(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{
		"foo.txt":   []byte("foo"),
		"bar.js":    []byte("foo js"),
		"a/foo.js":  []byte("foo js"),
		"a/foo.txt": []byte("foo txt"),
		"b/foo.txt": []byte("foo b"),
		"c/a.txt":   []byte("b"),
		"c/b.txt":   []byte("b"),
		"c/c.txt":   []byte("b"),
	})

	srv := makeTestServer(t, fmt.Sprintf(`
directory: %s
permissions: CRUD
prefix: /prefix

users:
  - username: basic
    password: basic
    rules:
    - regex: "^.+.js$"
      permissions: R
    - path: "/b/"
      permissions: R
    - path: "/a/foo.txt"
      permissions: none
    - path: "/c/"
      permissions: none
`, dir))

	client := gowebdav.NewClient(srv.URL, "basic", "basic")

	files, err := client.ReadDir("/prefix")
	require.NoError(t, err)
	require.Len(t, files, 5)

	err = client.Write("/prefix/foo.txt", []byte("new"), 0666)
	require.NoError(t, err)

	err = client.Write("/prefix/new.txt", []byte("new"), 0666)
	require.NoError(t, err)

	err = client.Copy("/prefix/bar.js", "/prefix/b/bar.js", false)
	require.ErrorContains(t, err, "403")

	err = client.Copy("/prefix/bar.js", "/prefix/bar.jsx", false)
	require.NoError(t, err)

	err = client.Copy("/prefix/b/foo.txt", "/prefix/foo1.txt", false)
	require.NoError(t, err)

	err = client.Rename("/prefix/b/foo.txt", "/prefix/foo2.txt", false)
	require.ErrorContains(t, err, "403")

	_, err = client.Read("/prefix/a/foo.txt")
	require.ErrorContains(t, err, "403")

	err = client.Write("/prefix/a/foo.js", []byte("new"), 0666)
	require.ErrorContains(t, err, "403")

	err = client.Write("/prefix/b/foo.txt", []byte("new"), 0666)
	require.ErrorContains(t, err, "403")

	_, err = client.ReadDir("/prefix/c")
	require.ErrorContains(t, err, "403")

	_, err = client.Read("/prefix/c/a.txt")
	require.ErrorContains(t, err, "403")

	err = client.Write("/prefix/c/b.txt", []byte("new"), 0666)
	require.ErrorContains(t, err, "403")
}

func TestServerMultiDirectories(t *testing.T) {
	t.Parallel()

	dirC := makeTestDirectory(t, map[string][]byte{
		"foo.txt":              []byte("foo"),
		"folder/nested.txt":    []byte("nested"),
		"public/access/ok.txt": []byte("ok"),
	})
	dirD := makeTestDirectory(t, map[string][]byte{
		"bar.txt": []byte("bar"),
	})

	srv := makeTestServer(t, fmt.Sprintf(`
permissions: CRUD
directories:
  - c: %s
  - d: %s
`, dirC, dirD))
	client := gowebdav.NewClient(srv.URL, "", "")

	files, err := client.ReadDir("/")
	require.NoError(t, err)
	require.Len(t, files, 2)
	require.Equal(t, "c", files[0].Name())
	require.Equal(t, "d", files[1].Name())

	data, err := client.Read("/c/foo.txt")
	require.NoError(t, err)
	require.EqualValues(t, []byte("foo"), data)

	data, err = client.Read("/d/bar.txt")
	require.NoError(t, err)
	require.EqualValues(t, []byte("bar"), data)

	err = client.Copy("/c/foo.txt", "/d/copied.txt", false)
	require.NoError(t, err)
	data, err = os.ReadFile(filepath.Join(dirD, "copied.txt"))
	require.NoError(t, err)
	require.EqualValues(t, []byte("foo"), data)

	err = client.Rename("/c/foo.txt", "/d/moved.txt", false)
	require.NoError(t, err)
	require.NoFileExists(t, filepath.Join(dirC, "foo.txt"))
	data, err = os.ReadFile(filepath.Join(dirD, "moved.txt"))
	require.NoError(t, err)
	require.EqualValues(t, []byte("foo"), data)

	err = client.Rename("/d/bar.txt", "/d/renamed.txt", false)
	require.NoError(t, err)
	require.NoFileExists(t, filepath.Join(dirD, "bar.txt"))
	data, err = os.ReadFile(filepath.Join(dirD, "renamed.txt"))
	require.NoError(t, err)
	require.EqualValues(t, []byte("bar"), data)

	err = client.Rename("/c/folder", "/d/folder", false)
	require.NoError(t, err)
	require.NoDirExists(t, filepath.Join(dirC, "folder"))
	data, err = os.ReadFile(filepath.Join(dirD, "folder", "nested.txt"))
	require.NoError(t, err)
	require.EqualValues(t, []byte("nested"), data)

	require.ErrorContains(t, client.Remove("/c"), "405")
	require.Error(t, client.Write("/c", []byte("blocked"), 0666))
	require.ErrorContains(t, client.Rename("/d", "/c/d", false), "403")
}

func TestServerMultiDirectoriesRules(t *testing.T) {
	t.Parallel()

	dirC := makeTestDirectory(t, map[string][]byte{
		"public/access/ok.txt": []byte("ok"),
	})
	dirD := makeTestDirectory(t, map[string][]byte{
		"public/access/no.txt": []byte("no"),
	})

	srv := makeTestServer(t, fmt.Sprintf(`
permissions: none
directories:
  - c: %s
  - d: %s
rules:
  - path: /c/public/access/
    permissions: R
`, dirC, dirD))
	client := gowebdav.NewClient(srv.URL, "", "")

	data, err := client.Read("/c/public/access/ok.txt")
	require.NoError(t, err)
	require.EqualValues(t, []byte("ok"), data)

	_, err = client.Read("/d/public/access/no.txt")
	require.ErrorContains(t, err, "403")

	_, err = client.Read("/public/access/ok.txt")
	require.ErrorContains(t, err, "403")
}

func TestServerMultiDirectoriesPrefix(t *testing.T) {
	t.Parallel()

	dirC := makeTestDirectory(t, map[string][]byte{
		"foo.txt": []byte("foo"),
	})

	srv := makeTestServer(t, fmt.Sprintf(`
permissions: R
prefix: /prefix
directories:
  - c: %s
`, dirC))
	client := gowebdav.NewClient(srv.URL, "", "")

	files, err := client.ReadDir("/prefix")
	require.NoError(t, err)
	require.Len(t, files, 1)
	require.Equal(t, "c", files[0].Name())

	data, err := client.Read("/prefix/c/foo.txt")
	require.NoError(t, err)
	require.EqualValues(t, []byte("foo"), data)
}

func TestServerPermissions(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{
		"foo.txt":   []byte("foo"),
		"a/foo.txt": []byte("foo a"),
		"b/foo.txt": []byte("foo b"),
	})

	srv := makeTestServer(t, fmt.Sprintf(`
directory: %s
permissions: CR

users:
  - username: a
    password: a
    directory: %s/a
  - username: b
    password: b
    directory: %s/b
    permissions: R
`, dir, dir, dir))

	t.Run("User A", func(t *testing.T) {
		t.Parallel()

		client := gowebdav.NewClient(srv.URL, "a", "a")

		files, err := client.ReadDir("/")
		require.NoError(t, err)
		require.Len(t, files, 1)

		data, err := client.Read("/foo.txt")
		require.NoError(t, err)
		require.EqualValues(t, []byte("foo a"), data)

		err = client.Copy("/foo.txt", "/copy.txt", false)
		require.NoError(t, err)

		err = client.Copy("/foo.txt", "/copy.txt", true)
		require.ErrorContains(t, err, "403")

		err = client.Rename("/foo.txt", "/copy.txt", true)
		require.ErrorContains(t, err, "403")

		data, err = client.Read("/copy.txt")
		require.NoError(t, err)
		require.EqualValues(t, []byte("foo a"), data)
	})

	t.Run("User B", func(t *testing.T) {
		t.Parallel()

		client := gowebdav.NewClient(srv.URL, "b", "b")

		files, err := client.ReadDir("/")
		require.NoError(t, err)
		require.Len(t, files, 1)

		data, err := client.Read("/foo.txt")
		require.NoError(t, err)
		require.EqualValues(t, []byte("foo b"), data)

		err = client.Copy("/foo.txt", "/copy.txt", false)
		require.ErrorContains(t, err, "403")
	})
}

func TestServerRulesDotSegments(t *testing.T) {
	t.Parallel()

	// Sends a request with an unmodified request-target, since a WebDAV client
	// would normalize the dot segments away before they reach the server.
	do := func(t *testing.T, method, url string, header map[string]string) int {
		t.Helper()

		req, err := http.NewRequest(method, url, nil)
		require.NoError(t, err)
		for k, v := range header {
			req.Header.Set(k, v)
		}

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		return resp.StatusCode
	}

	makeServer := func(t *testing.T) (*httptest.Server, string) {
		dir := makeTestDirectory(t, map[string][]byte{
			"public/pub.txt":  []byte("public"),
			"secret/flag.txt": []byte("secret"),
			"secret/keep.txt": []byte("keep"),
		})

		srv := makeTestServer(t, fmt.Sprintf(`
directory: %s
permissions: CRUD
rules:
  - path: "/secret/"
    permissions: none
`, dir))
		t.Cleanup(srv.Close)
		return srv, dir
	}

	t.Run("Source Path", func(t *testing.T) {
		t.Parallel()
		srv, _ := makeServer(t)

		for _, path := range []string{
			"/secret/flag.txt",
			"/public/../secret/flag.txt",
			"/public/%2e%2e/secret/flag.txt",
			"/public/../secret/",
			"/secret/.",
			"/secret/%2e",
		} {
			require.Equal(t, http.StatusForbidden, do(t, "GET", srv.URL+path, nil), path)
		}

		require.Equal(t, http.StatusForbidden, do(t, "PUT", srv.URL+"/public/%2e%2e/secret/new.txt", nil))
		require.Equal(t, http.StatusForbidden, do(t, "DELETE", srv.URL+"/public/%2e%2e/secret/keep.txt", nil))

		// A request that resolves outside a rule must still succeed.
		require.Equal(t, http.StatusOK, do(t, "GET", srv.URL+"/secret/../public/pub.txt", nil))
	})

	t.Run("Destination Header", func(t *testing.T) {
		t.Parallel()
		srv, _ := makeServer(t)

		for _, destination := range []string{
			srv.URL + "/public/%2e%2e/secret/moved.txt",
			srv.URL + "/public/../secret/moved.txt",
			"/public/%2e%2e/secret/moved.txt",
		} {
			code := do(t, "MOVE", srv.URL+"/public/pub.txt", map[string]string{
				"Destination": destination,
				"Overwrite":   "T",
			})
			require.Equal(t, http.StatusForbidden, code, destination)
		}
	})

	t.Run("Regex Rule", func(t *testing.T) {
		t.Parallel()

		dir := makeTestDirectory(t, map[string][]byte{
			"public/pub.txt":  []byte("public"),
			"secret/flag.txt": []byte("secret"),
		})

		srv := makeTestServer(t, fmt.Sprintf(`
directory: %s
permissions: CRUD
rules:
  - regex: "^/secret/"
    permissions: none
`, dir))
		defer srv.Close()

		require.Equal(t, http.StatusForbidden, do(t, "GET", srv.URL+"/secret/flag.txt", nil))
		require.Equal(t, http.StatusForbidden, do(t, "GET", srv.URL+"/public/%2e%2e/secret/flag.txt", nil))
	})

	t.Run("Directory Mounts", func(t *testing.T) {
		t.Parallel()

		alpha := makeTestDirectory(t, map[string][]byte{"a.txt": []byte("a")})
		beta := makeTestDirectory(t, map[string][]byte{"b.txt": []byte("b")})

		srv := makeTestServer(t, fmt.Sprintf(`
permissions: CRUD
directories:
  - name: alpha
    path: %s
  - name: beta
    path: %s
rules:
  - path: "/beta/"
    permissions: none
`, alpha, beta))
		defer srv.Close()

		require.Equal(t, http.StatusForbidden, do(t, "GET", srv.URL+"/beta/b.txt", nil))
		require.Equal(t, http.StatusForbidden, do(t, "GET", srv.URL+"/alpha/%2e%2e/beta/b.txt", nil))
		require.Equal(t, http.StatusOK, do(t, "GET", srv.URL+"/alpha/a.txt", nil))
	})

	t.Run("No Users", func(t *testing.T) {
		t.Parallel()
		srv, _ := makeServer(t)

		// Without a users block no authentication runs, so the rule is the only
		// access control there is.
		require.Equal(t, http.StatusForbidden, do(t, "GET", srv.URL+"/public/%2e%2e/secret/flag.txt", nil))
	})
}

func TestServerRulesBareCollection(t *testing.T) {
	t.Parallel()

	do := func(t *testing.T, method, url string) int {
		t.Helper()

		req, err := http.NewRequest(method, url, nil)
		require.NoError(t, err)
		req.Header.Set("Depth", "1")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		return resp.StatusCode
	}

	dir := makeTestDirectory(t, map[string][]byte{
		"c/secret.txt": []byte("secret"),
		"cd/open.txt":  []byte("open"),
	})

	srv := makeTestServer(t, fmt.Sprintf(`
directory: %s
permissions: CRUD
rules:
  - path: "/c/"
    permissions: none
`, dir))
	defer srv.Close()

	// A rule written "/c/" must also cover the collection named without the
	// trailing slash, otherwise the denied directory can be listed or deleted.
	for _, path := range []string{"/c/", "/c", "/c/secret.txt"} {
		require.Equal(t, http.StatusForbidden, do(t, "PROPFIND", srv.URL+path), path)
		require.Equal(t, http.StatusForbidden, do(t, "DELETE", srv.URL+path), path)
	}

	// A sibling whose name merely starts with the same characters is unaffected.
	require.Equal(t, http.StatusMultiStatus, do(t, "PROPFIND", srv.URL+"/cd"))
	require.Equal(t, http.StatusOK, do(t, "GET", srv.URL+"/cd/open.txt"))

	// A rule governs the collection it names, but must not grant access to it
	// that would not otherwise exist: removing "/pub" acts on the root, which
	// the global permissions still deny.
	grantDir := makeTestDirectory(t, map[string][]byte{"pub/x.txt": []byte("x")})
	grantSrv := makeTestServer(t, fmt.Sprintf(`
directory: %s
permissions: none
rules:
  - path: "/pub/"
    permissions: CRUD
`, grantDir))
	defer grantSrv.Close()

	require.Equal(t, http.StatusNoContent, do(t, "DELETE", grantSrv.URL+"/pub/x.txt"))
	require.Equal(t, http.StatusForbidden, do(t, "DELETE", grantSrv.URL+"/pub"))
	require.Equal(t, http.StatusForbidden, do(t, "PROPFIND", grantSrv.URL+"/pub"))
}

func TestServerRulesEmptyPrefixDestination(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{
		"public/a.txt": []byte("a"),
		"public/b.txt": []byte("b"),
		"secret/x.txt": []byte("secret"),
	})

	srv := makeTestServer(t, fmt.Sprintf(`
directory: %s
prefix: ""
permissions: CRUD
rules:
  - path: "/secret/"
    permissions: none
`, dir))
	defer srv.Close()

	// RFC 4918 has Destination as an absolute URI, so the host must not end up
	// in the value the rules are matched against.
	for name, destination := range map[string]string{
		"absolute":  srv.URL + "/secret/moved.txt",
		"bare path": "/secret/moved.txt",
		"dot":       srv.URL + "/public/%2e%2e/secret/moved.txt",
	} {
		req, err := http.NewRequest("MOVE", srv.URL+"/public/a.txt", nil)
		require.NoError(t, err)
		req.Header.Set("Destination", destination)
		req.Header.Set("Overwrite", "T")

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		require.NoError(t, resp.Body.Close())
		require.Equal(t, http.StatusForbidden, resp.StatusCode, name)
	}

	// A destination outside any rule must still work.
	req, err := http.NewRequest("MOVE", srv.URL+"/public/b.txt", nil)
	require.NoError(t, err)
	req.Header.Set("Destination", srv.URL+"/public/moved.txt")
	req.Header.Set("Overwrite", "T")

	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	require.NoError(t, resp.Body.Close())
	require.Equal(t, http.StatusCreated, resp.StatusCode)
}
