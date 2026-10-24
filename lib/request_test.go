package lib

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestWindowsAlias(t *testing.T) {
	t.Parallel()

	for _, segment := range []string{
		"SUPERS~1",
		"CREDEN~1.TXT",
		"supers~12.txt",
		"secret.",
		"secret ",
		"...",
	} {
		require.True(t, windowsAlias(segment), segment)
	}

	// Names Office and editors create, which no short name looks like.
	for _, segment := range []string{
		"secret",
		"~$doc.docx",
		"~WRL0001.tmp",
		"file~",
		"a.~1~",
		"report~2024x",
		".hidden",
	} {
		require.False(t, windowsAlias(segment), segment)
	}
}

func TestLocalPath(t *testing.T) {
	t.Parallel()

	require.True(t, localPath("/"))
	require.True(t, localPath("/public/file.txt"))
	require.True(t, localPath("/public/"))

	// Elsewhere these are ordinary names, or fail to open, in both layers alike.
	windows := runtime.GOOS == "windows"
	for _, p := range []string{
		"/public/a\x00b",
		`/a\..\secret/file.txt`,
		"/secret::$INDEX_ALLOCATION/file.txt",
		"/con",
		"/SUPERS~1/file.txt",
		"/secret./file.txt",
		"/secret ",
		"/a\xffb",
	} {
		require.Equal(t, !windows, localPath(p), p)
	}
}
