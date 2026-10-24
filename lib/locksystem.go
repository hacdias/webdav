package lib

import (
	"errors"
	"path"
	"path/filepath"
	"time"

	"golang.org/x/net/webdav"
)

var _ webdav.LockSystem = &lockSystem{}

// lockSystem wraps a [webdav.LockSystem], mapping virtual request names to the
// real backing paths via resolve. This allows reusing the same
// [webdav.LockSystem] for multiple users with different base directories,
// meaning we can correctly lock the files across different users.
type lockSystem struct {
	webdav.LockSystem
	resolve func(name string) (string, error)
}

// newLockSystem returns a lockSystem for a single-directory user, resolving
// names relative to directory.
func newLockSystem(ls webdav.LockSystem, directory string) *lockSystem {
	return &lockSystem{
		LockSystem: ls,
		resolve: func(name string) (string, error) {
			// Lock names share a slash-separated namespace across users, even
			// on Windows where filepath.Join would emit backslashes and break
			// descendant-lock matching in the underlying LockSystem.
			return path.Join(filepath.ToSlash(directory), name), nil
		},
	}
}

// newMultiDirLockSystem returns a lockSystem for a multi-directory user,
// resolving names against the real backing path of each mount.
func newMultiDirLockSystem(ls webdav.LockSystem, mounts DirectoryMounts) *lockSystem {
	return &lockSystem{
		LockSystem: ls,
		resolve: func(name string) (string, error) {
			if cleanName(name) == "/" {
				return "/", nil
			}

			mount, rest, err := multiDir{mounts: mounts}.resolve(name)
			if err != nil {
				return "", err
			}

			// filePath returns an OS-native path for real file operations; the
			// lock namespace must stay slash-separated so descendant locks match
			// on Windows.
			return filepath.ToSlash(mount.filePath(rest)), nil
		},
	}
}

func (l *lockSystem) Confirm(now time.Time, name0, name1 string, conditions ...webdav.Condition) (release func(), err error) {
	if name0 != "" {
		name0, err = l.resolve(name0)
		if err != nil {
			return nil, err
		}
	}

	if name1 != "" {
		name1, err = l.resolve(name1)
		if err != nil {
			return nil, err
		}
	}

	release, err = l.LockSystem.Confirm(now, name0, name1, conditions...)
	if !errors.Is(err, webdav.ErrConfirmationFailed) || name0 == "" || name1 == "" {
		return release, err
	}

	// A MOVE may only submit the source's token: hold an unlocked destination
	// as [webdav.Handler] does without an If header.
	releaseSource, err := l.LockSystem.Confirm(now, name0, "", conditions...)
	if err != nil {
		return nil, err
	}

	token, err := l.LockSystem.Create(now, webdav.LockDetails{Root: name1, Duration: -1, ZeroDepth: true})
	if err != nil {
		releaseSource()
		return nil, webdav.ErrConfirmationFailed
	}

	return func() {
		_ = l.Unlock(now, token)
		releaseSource()
	}, nil
}

func (l *lockSystem) Create(now time.Time, details webdav.LockDetails) (token string, err error) {
	details.Root, err = l.resolve(details.Root)
	if err != nil {
		return "", err
	}
	return l.LockSystem.Create(now, details)
}
