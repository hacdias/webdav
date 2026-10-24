package lib

import (
	"os"
	"path/filepath"
	"runtime"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestRenameAcrossMount(t *testing.T) {
	source := makeTestDirectory(t, map[string][]byte{
		"file.txt":               []byte("cross mount"),
		"folder/empty":           nil,
		"folder/nested/file.txt": []byte("nested"),
	})
	target := t.TempDir()
	sourceFile := filepath.Join(source, "file.txt")
	modTime := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)
	require.NoError(t, os.Chmod(sourceFile, 0600))
	require.NoError(t, os.Chtimes(sourceFile, modTime, modTime))

	require.NoError(t, renameAcrossMount(sourceFile, filepath.Join(target, "file.txt")))
	require.NoFileExists(t, filepath.Join(source, "file.txt"))
	data, err := os.ReadFile(filepath.Join(target, "file.txt"))
	require.NoError(t, err)
	require.Equal(t, []byte("cross mount"), data)
	info, err := os.Stat(filepath.Join(target, "file.txt"))
	require.NoError(t, err)
	if runtime.GOOS != "windows" {
		require.Equal(t, os.FileMode(0600), info.Mode().Perm())
	}
	require.WithinDuration(t, modTime, info.ModTime(), time.Second)

	require.NoError(t, renameAcrossMount(filepath.Join(source, "folder"), filepath.Join(target, "folder")))
	require.NoDirExists(t, filepath.Join(source, "folder"))
	require.DirExists(t, filepath.Join(target, "folder", "empty"))
	data, err = os.ReadFile(filepath.Join(target, "folder", "nested", "file.txt"))
	require.NoError(t, err)
	require.Equal(t, []byte("nested"), data)
}

func TestRenameAcrossMountPreservesSymlink(t *testing.T) {
	source := makeTestDirectory(t, map[string][]byte{
		"file.txt": []byte("target"),
	})
	oldPath := filepath.Join(source, "link.txt")
	if err := os.Symlink("file.txt", oldPath); err != nil {
		t.Skipf("symbolic links are unavailable: %v", err)
	}
	newPath := filepath.Join(t.TempDir(), "link.txt")

	require.NoError(t, renameAcrossMount(oldPath, newPath))
	info, err := os.Lstat(newPath)
	require.NoError(t, err)
	require.NotZero(t, info.Mode()&os.ModeSymlink)
	target, err := os.Readlink(newPath)
	require.NoError(t, err)
	require.Equal(t, "file.txt", target)
}

func TestMultiDirRenameBackslash(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("backslash is a separator only on Windows")
	}

	root := makeTestDirectory(t, map[string][]byte{
		"outside.txt": []byte("outside"),
		"alpha":       nil,
		"beta":        nil,
	})
	m := multiDir{mounts: DirectoryMounts{
		{Name: "alpha", Path: filepath.Join(root, "alpha")},
		{Name: "beta", Path: filepath.Join(root, "beta")},
	}}

	err := m.Rename(t.Context(), `/alpha/x\..\..\outside.txt`, "/beta/stolen.txt")
	require.ErrorIs(t, err, os.ErrNotExist)
	require.FileExists(t, filepath.Join(root, "outside.txt"))
}

func TestIsCrossDeviceError(t *testing.T) {
	err := syscall.EXDEV
	if runtime.GOOS == "windows" {
		err = windowsErrorNotSameDevice
	}
	require.True(t, isCrossDeviceError(err))
	require.False(t, isCrossDeviceError(os.ErrPermission))
}
