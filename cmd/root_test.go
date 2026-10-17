package cmd

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const reloadLockBody = `<?xml version="1.0" encoding="utf-8"?><D:lockinfo xmlns:D="DAV:"><D:lockscope><D:exclusive/></D:lockscope><D:locktype><D:write/></D:locktype></D:lockinfo>`

func freeConfigPort(t *testing.T) int {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	port := listener.Addr().(*net.TCPAddr).Port
	require.NoError(t, listener.Close())
	return port
}

func reloadConfigText(dir string, port int, bob bool) string {
	text := fmt.Sprintf("address: 127.0.0.1\nport: %d\ndirectory: %q\npermissions: CRUD\nlog: {outputs: []}\nusers:\n  - username: alice\n    password: alicepass\n", port, dir)
	if bob {
		text += "    rules:\n      - path: /secret\n        permissions: none\n  - username: bob\n    password: bobpass\n"
	}
	return text
}

func replaceConfig(t *testing.T, path, text string) {
	t.Helper()
	tmp := path + ".tmp"
	require.NoError(t, os.WriteFile(tmp, []byte(text), 0600))
	if err := os.Rename(tmp, path); err != nil {
		require.Eventually(t, func() bool { return os.Rename(tmp, path) == nil },
			2*time.Second, 20*time.Millisecond, "failed to replace config file: %v", err)
	}
}

func runConfigServer(t *testing.T, path string) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	errCh := make(chan error, 1)
	rootCmd.SetArgs([]string{"--config", path})
	go func() { errCh <- rootCmd.ExecuteContext(ctx) }()
	t.Cleanup(func() {
		defer rootCmd.SetArgs(nil)
		cancel()
		select {
		case err := <-errCh:
			require.NoError(t, err)
		case <-time.After(5 * time.Second):
			t.Error("server did not stop after context cancellation")
		}
	})
}

// Each request uses a new connection so listener assertions cannot pass through
// an old keep-alive connection. This helper is also used from Eventually polls.
func configRequest(url, method, path, user, body string, headers map[string]string) (int, http.Header, error) {
	req, err := http.NewRequest(method, url+path, strings.NewReader(body))
	if err != nil {
		return 0, nil, err
	}
	req.Close = true
	req.SetBasicAuth(user, user+"pass")
	req.Header.Set("Depth", "0")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	client := &http.Client{Timeout: time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	err = resp.Body.Close()
	return resp.StatusCode, resp.Header, err
}

func awaitConfigServer(t *testing.T, url, user string) {
	t.Helper()
	require.Eventually(t, func() bool {
		code, _, err := configRequest(url, "PROPFIND", "/", user, "", nil)
		return err == nil && code == http.StatusMultiStatus
	}, 10*time.Second, 50*time.Millisecond)
}

func TestConfigReload(t *testing.T) {
	dir := t.TempDir()
	data := filepath.Join(dir, "data")
	require.NoError(t, os.Mkdir(data, 0700))
	require.NoError(t, os.WriteFile(filepath.Join(data, "file.txt"), []byte("original"), 0600))
	require.NoError(t, os.WriteFile(filepath.Join(data, "secret"), []byte("secret"), 0600))
	path := filepath.Join(dir, "config.yml")
	port := freeConfigPort(t)
	replaceConfig(t, path, reloadConfigText(data, port, false))
	runConfigServer(t, path)
	url := "http://127.0.0.1:" + strconv.Itoa(port)
	awaitConfigServer(t, url, "alice")

	code, headers, err := configRequest(url, "LOCK", "/file.txt", "alice", reloadLockBody,
		map[string]string{"Timeout": "Second-600", "Content-Type": "application/xml"})
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, code)
	token := headers.Get("Lock-Token")
	require.NotEmpty(t, token)

	replaceConfig(t, path, reloadConfigText(data, port, true))
	awaitConfigServer(t, url, "bob")
	code, _, err = configRequest(url, "GET", "/secret", "alice", "", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusForbidden, code)
	code, _, err = configRequest(url, "PUT", "/file.txt", "bob", "changed", nil)
	require.NoError(t, err)
	require.Equal(t, http.StatusLocked, code)

	for _, invalid := range []string{"", "users: [ {username: x} ]"} {
		require.NoError(t, os.WriteFile(path, []byte(invalid), 0600))
		require.Never(t, func() bool {
			code, _, err := configRequest(url, "PROPFIND", "/", "bob", "", nil)
			return err != nil || code != http.StatusMultiStatus
		}, 1500*time.Millisecond, 50*time.Millisecond)
	}
	require.NoError(t, os.WriteFile(path, []byte(reloadConfigText(data, port, true)+carolConfig), 0600))
	awaitConfigServer(t, url, "carol")
	code, _, err = configRequest(url, "UNLOCK", "/file.txt", "alice", "", map[string]string{"Lock-Token": token})
	require.NoError(t, err)
	require.Equal(t, http.StatusNoContent, code)
}

const carolConfig = "  - username: carol\n    password: carolpass\n"

func TestConfigReloadRearm(t *testing.T) {
	data, dir := t.TempDir(), t.TempDir()
	path, port := filepath.Join(dir, "config.yml"), freeConfigPort(t)
	replaceConfig(t, path, reloadConfigText(data, port, false))
	runConfigServer(t, path)
	url := "http://127.0.0.1:" + strconv.Itoa(port)
	awaitConfigServer(t, url, "alice")
	text := reloadConfigText(data, port, true)
	require.NoError(t, os.WriteFile(path+".new", []byte(text), 0600))
	// Windows briefly refuses these while the file is open.
	require.Eventually(t, func() bool { return os.Remove(path) == nil }, 2*time.Second, 10*time.Millisecond)
	require.Eventually(t, func() bool { return os.Rename(path+".new", path) == nil }, 2*time.Second, 10*time.Millisecond)
	awaitConfigServer(t, url, "bob")
	require.NoError(t, os.WriteFile(path, []byte(text+carolConfig), 0600))
	awaitConfigServer(t, url, "carol")
}

func TestConfigNotWatchedWhenServed(t *testing.T) {
	data, port := t.TempDir(), freeConfigPort(t)
	path := filepath.Join(data, "config.yml")
	replaceConfig(t, path, reloadConfigText(data, port, false))
	runConfigServer(t, path)
	url := "http://127.0.0.1:" + strconv.Itoa(port)
	awaitConfigServer(t, url, "alice")
	require.NoError(t, os.WriteFile(path, []byte(reloadConfigText(data, port, true)), 0600))
	require.Never(t, func() bool {
		code, _, err := configRequest(url, "PROPFIND", "/", "bob", "", nil)
		return err == nil && code == http.StatusMultiStatus
	}, 1500*time.Millisecond, 50*time.Millisecond)
}
