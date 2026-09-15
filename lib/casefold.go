package lib

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// defaultCaseInsensitiveFS is true on platforms that are case-insensitive by
// default, and false on platforms that are case-sensitive by default. It is used
// as a fallback when probing the file system fails.
const defaultCaseInsensitiveFS = runtime.GOOS == "darwin" || runtime.GOOS == "windows"

// foldPath is used to compare paths in a case-insensitive manner, normalizing
// them to NFC and converting to lower case.
func foldPath(p string) string {
	return norm.NFC.String(strings.ToLower(p))
}

// caseInsensitiveFS probes the file system backing dir to see if it is case
// insensitive. It falls back to the platform default when the probe fails.
func caseInsensitiveFS(dir string) bool {
	info, err := os.Stat(dir)
	if err != nil {
		return defaultCaseInsensitiveFS
	}

	flipped, ok := flipCase(dir)
	if !ok {
		return defaultCaseInsensitiveFS
	}

	other, err := os.Stat(flipped)
	if err != nil {
		return false
	}

	return os.SameFile(info, other)
}

// flipCase swaps the case of the first letter in the last element of dir whose
// case mapping round-trips, reporting false when it holds none.
func flipCase(dir string) (string, bool) {
	flipped := false

	base := strings.Map(func(r rune) rune {
		if flipped {
			return r
		}

		switch {
		case unicode.IsLower(r):
			if u := unicode.ToUpper(r); unicode.ToLower(u) == r {
				flipped = true
				return u
			}
		case unicode.IsUpper(r):
			if l := unicode.ToLower(r); unicode.ToUpper(l) == r {
				flipped = true
				return l
			}
		}

		return r
	}, filepath.Base(dir))

	if !flipped {
		return "", false
	}

	return filepath.Join(filepath.Dir(dir), base), true
}
