package lib

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func reloadRequest(h http.Handler, method, path, user, body string, headers map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.SetBasicAuth(user, user+"pass")
	req.Header.Set("Depth", "0")
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestReloadableHandler(t *testing.T) {
	t.Parallel()
	dir := makeTestDirectory(t, map[string][]byte{"file.txt": []byte("original")})
	cfg := writeAndParseConfig(t, fmt.Sprintf("directory: %q\npermissions: CRUD\nusers:\n  - username: alice\n    password: alicepass", dir), ".yml")
	h, err := NewReloadableHandler(cfg)
	require.NoError(t, err)
	resp := reloadRequest(h, "LOCK", "/file.txt", "alice", exclusiveWriteLock, map[string]string{"Timeout": "Second-600"})
	require.Equal(t, http.StatusOK, resp.Code)
	token := resp.Header().Get("Lock-Token")
	require.NotEmpty(t, token)
	require.Equal(t, http.StatusUnauthorized, reloadRequest(h, "PROPFIND", "/", "bob", "", nil).Code)

	cfg = writeAndParseConfig(t, fmt.Sprintf("directory: %q\npermissions: CRUD\nusers:\n  - username: alice\n    password: alicepass\n  - username: bob\n    password: bobpass", dir), ".yml")
	require.NoError(t, h.Reload(cfg))
	require.Equal(t, http.StatusMultiStatus, reloadRequest(h, "PROPFIND", "/", "bob", "", nil).Code)
	require.Equal(t, http.StatusLocked, reloadRequest(h, "PUT", "/file.txt", "bob", "changed", nil).Code)
	require.Equal(t, http.StatusNoContent, reloadRequest(h, "UNLOCK", "/file.txt", "alice", "", map[string]string{"Lock-Token": token}).Code)
	resp = reloadRequest(h, "GET", "/file.txt", "bob", "", nil)
	require.Equal(t, http.StatusOK, resp.Code)
	require.Equal(t, "original", resp.Body.String())
}

func TestReloadableHandlerInFlight(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	cfg := writeAndParseConfig(t, fmt.Sprintf("directory: %q\npermissions: CRUD", dir), ".yml")
	h, err := NewReloadableHandler(cfg)
	require.NoError(t, err)
	reader, writer := io.Pipe()
	defer func() { _ = reader.Close(); _ = writer.Close() }()
	req := httptest.NewRequest("PUT", "/upload.txt", reader)
	rec := httptest.NewRecorder()
	done := make(chan struct{})
	go func() {
		h.ServeHTTP(rec, req)
		close(done)
	}()
	require.Eventually(t, func() bool {
		_, err := os.Stat(filepath.Join(dir, "upload.txt"))
		return err == nil
	}, 5*time.Second, 10*time.Millisecond)
	cfg = writeAndParseConfig(t, fmt.Sprintf("directory: %q\npermissions: R", dir), ".yml")
	require.NoError(t, h.Reload(cfg))
	require.Equal(t, http.StatusForbidden, reloadRequest(h, "PUT", "/other.txt", "", "denied", nil).Code)
	// Deferred pipe cleanup releases the writer if the request fails.
	go func() {
		_, _ = io.WriteString(writer, "completed")
		_ = writer.Close()
	}()
	require.Eventually(t, func() bool {
		select {
		case <-done:
			return true
		default:
			return false
		}
	}, 5*time.Second, 10*time.Millisecond)
	require.Equal(t, http.StatusCreated, rec.Code)
	content, err := os.ReadFile(filepath.Join(dir, "upload.txt"))
	require.NoError(t, err)
	require.Equal(t, "completed", string(content))
}
