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

func TestLockSystemConfirmSkipsDestinationForMove(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ls := newLockSystem(webdav.NewMemLS(), dir)
	now := time.Now()

	// Create a lock on the source file (simulating temp file lock).
	srcToken, err := ls.Create(now, webdav.LockDetails{
		Root:      "/temp.txt",
		Duration:  time.Minute,
		ZeroDepth: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = ls.Unlock(time.Now(), srcToken)
	})

	// MOVE: confirm with source lock and no destination lock.
	// This should succeed — the If header only covers the source.
	release, err := ls.Confirm(now, "/temp.txt", "/original.txt", webdav.Condition{
		Token: srcToken,
	})
	require.NoError(t, err)
	require.NotNil(t, release)
	release()

	// MOVE without any valid lock should fail.
	_, err = ls.Confirm(now, "/temp.txt", "/original.txt")
	require.ErrorIs(t, err, webdav.ErrConfirmationFailed)
}

func TestLockSystemConfirmStillChecksDestinationWhenNoSource(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	ls := newLockSystem(webdav.NewMemLS(), dir)
	now := time.Now()

	// Create a lock on the destination file.
	dstToken, err := ls.Create(now, webdav.LockDetails{
		Root:      "/dest.txt",
		Duration:  time.Minute,
		ZeroDepth: true,
	})
	require.NoError(t, err)
	t.Cleanup(func() {
		_ = ls.Unlock(time.Now(), dstToken)
	})

	// When name0 is empty (e.g. COPY with no source lock),
	// destination conditions should still be checked.
	release, err := ls.Confirm(now, "", "/dest.txt", webdav.Condition{
		Token: dstToken,
	})
	require.NoError(t, err)
	require.NotNil(t, release)
	release()
}
