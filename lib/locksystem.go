package lib

import (
	"path/filepath"
	"time"

	"golang.org/x/net/webdav"
)

var _ webdav.LockSystem = &lockSystem{}

// LockSystem wraps a [webdav.LockSystem] with a root directory, allowing
// to reuse the same [webdav.LockSystem] for multiple users with different base
// directories, meaning we can correctly lock the files across different users.
type lockSystem struct {
	webdav.LockSystem
	directory string
}

func (l *lockSystem) Confirm(now time.Time, name0, name1 string, conditions ...webdav.Condition) (release func(), err error) {
	if name0 != "" {
		name0 = filepath.Join(l.directory, name0)
	}

	if name1 != "" {
		name1 = filepath.Join(l.directory, name1)
	}

	return l.LockSystem.Confirm(now, name0, name1, conditions...)
}

func (l *lockSystem) Create(now time.Time, details webdav.LockDetails) (token string, err error) {
	details.Root = filepath.Join(l.directory, details.Root)
	return l.LockSystem.Create(now, details)
}
