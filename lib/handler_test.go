package lib

import (
	"fmt"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
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

func TestServerListingCharacters(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{
		"富/foo.txt": []byte("foo"),
		"你好.txt":    []byte("bar"),
		"z*.txt":    []byte("zbar"),
		"foo.txt":   []byte("foo"),
		"🌹.txt":     []byte("foo"),
	})

	srv := makeTestServer(t, "directory: "+dir)
	client := gowebdav.NewClient(srv.URL, "", "")

	// By default, reading permissions.
	files, err := client.ReadDir("/")
	require.NoError(t, err)
	require.Len(t, files, 5)

	names := []string{
		files[0].Name(),
		files[1].Name(),
		files[2].Name(),
		files[3].Name(),
		files[4].Name(),
	}
	sort.Strings(names)

	require.Equal(t, []string{
		"foo.txt",
		"z*.txt",
		"你好.txt",
		"富",
		"🌹.txt",
	}, names)

	data, err := client.Read("/z*.txt")
	require.NoError(t, err)
	require.EqualValues(t, []byte("zbar"), data)
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

func TestServerRules(t *testing.T) {
	t.Parallel()

	dir := makeTestDirectory(t, map[string][]byte{
		"foo.txt":   []byte("foo"),
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
    - path: "/b"
      permissions: R
    - path: "/a/foo.txt"
      permissions: none
    - path: "/c"
      permissions: none
`, dir))

	client := gowebdav.NewClient(srv.URL, "basic", "basic")

	files, err := client.ReadDir("/")
	require.NoError(t, err)
	require.Len(t, files, 4)

	err = client.Write("/foo.txt", []byte("new"), 0666)
	require.NoError(t, err)

	err = client.Write("/new.txt", []byte("new"), 0666)
	require.NoError(t, err)

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
