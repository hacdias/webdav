package lib

import (
	"path"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/net/webdav"
)

func TestLockSystemRootLockProtectsDescendants(t *testing.T) {
	t.Parallel()

	locks := newLockSystem(webdav.NewMemLS(), filepath.Join(t.TempDir(), "nested"))
	now := time.Now()

	token, err := locks.Create(now, webdav.LockDetails{
		Root:     "/",
		Duration: time.Minute,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, locks.Unlock(time.Now(), token))
	})

	_, err = locks.Create(now, webdav.LockDetails{
		Root:      "/child.txt",
		Duration:  time.Minute,
		ZeroDepth: true,
	})
	require.ErrorIs(t, err, webdav.ErrLocked)
}

func TestLockSystemSharesLocksAcrossNestedUserDirectories(t *testing.T) {
	t.Parallel()

	shared := webdav.NewMemLS()
	parentDirectory := t.TempDir()
	childDirectory := filepath.Join(parentDirectory, "child")
	parent := newLockSystem(shared, parentDirectory)
	child := newLockSystem(shared, childDirectory)
	now := time.Now()

	token, err := parent.Create(now, webdav.LockDetails{
		Root:     "/",
		Duration: time.Minute,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, parent.Unlock(time.Now(), token))
	})

	_, err = child.Create(now, webdav.LockDetails{
		Root:      "/file.txt",
		Duration:  time.Minute,
		ZeroDepth: true,
	})
	require.ErrorIs(t, err, webdav.ErrLocked)

	// The lock key is slash-separated on every platform, so the child's file
	// nests under the parent's root lock rather than diverging on Windows.
	key, err := child.resolve("/file.txt")
	require.NoError(t, err)
	require.Equal(t, path.Join(filepath.ToSlash(childDirectory), "file.txt"), key)
}

func TestMultiDirLockSystemUsesSlashSeparatedKeys(t *testing.T) {
	t.Parallel()

	mounts := DirectoryMounts{{Name: "docs", Path: filepath.Join(t.TempDir(), "docs")}}
	locks := newMultiDirLockSystem(webdav.NewMemLS(), mounts)

	key, err := locks.resolve("/docs/report.txt")
	require.NoError(t, err)
	require.Equal(t, path.Join(filepath.ToSlash(mounts[0].Path), "report.txt"), key)
	require.NotContains(t, key, "\\")
}
