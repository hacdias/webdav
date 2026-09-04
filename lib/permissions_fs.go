package lib

import (
	"context"
	"errors"
	"io"
	"os"
	"path"

	"golang.org/x/net/webdav"
)

var _ webdav.FileSystem = permissionsFS{}

// permissionsFS wraps a [webdav.FileSystem] so directory listings only report
// entries the user may read. PROPFIND and COPY reach descendants by enumerating
// through the file system, so rules have to be applied as those listings are produced.
//
// The wrapped file system is a named field rather than an embedded one on
// purpose. allowedThroughout has to read unfiltered listings, and a promoted
// OpenFile would make walking the filtered view by accident a one-character
// change that authorizes everything without failing any obvious way.
type permissionsFS struct {
	fs    webdav.FileSystem
	perms UserPermissions
}

func (f permissionsFS) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	return f.fs.Mkdir(ctx, name, perm)
}

func (f permissionsFS) RemoveAll(ctx context.Context, name string) error {
	return f.fs.RemoveAll(ctx, name)
}

func (f permissionsFS) Rename(ctx context.Context, oldName, newName string) error {
	return f.fs.Rename(ctx, oldName, newName)
}

func (f permissionsFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	return f.fs.Stat(ctx, name)
}

func (f permissionsFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	file, err := f.fs.OpenFile(ctx, name, flag, perm)
	if err != nil {
		return nil, err
	}

	// The handler strips its prefix first, which for the default prefix "/" also
	// drops the leading slash. Rules are written with one.
	return &permissionsFile{File: file, name: cleanPath(name), perms: f.perms}, nil
}

type permissionsFile struct {
	webdav.File
	name  string
	perms UserPermissions
}

func (f *permissionsFile) Readdir(count int) ([]os.FileInfo, error) {
	if count <= 0 {
		fis, err := f.File.Readdir(count)
		if err != nil {
			return nil, err
		}

		return f.readable(fis), nil
	}

	// A positive count asks for that many entries, so keep reading until that
	// many survive filtering: a short read would look like the end of the listing.
	var entries []os.FileInfo

	for len(entries) < count {
		fis, err := f.File.Readdir(count - len(entries))
		if err != nil {
			if len(entries) > 0 && errors.Is(err, io.EOF) {
				return entries, nil
			}

			return nil, err
		}

		if len(fis) == 0 {
			break
		}

		entries = append(entries, f.readable(fis)...)
	}

	return entries, nil
}

// readable returns the entries whose path the user is allowed to read.
func (f *permissionsFile) readable(fis []os.FileInfo) []os.FileInfo {
	allowed := make([]os.FileInfo, 0, len(fis))

	for _, fi := range fis {
		if f.perms.allowedAt(path.Join(f.name, fi.Name()), func(p Permissions) bool {
			return p.Read
		}) {
			allowed = append(allowed, fi)
		}
	}

	return allowed
}

// allowedThroughout reports whether check holds for every descendant of name.
// Rename and RemoveAll act on a subtree in one call without consulting the file
// system per descendant, so MOVE and DELETE need this before dispatching.
//
// It reads f.fs directly rather than the [permissionsFS] view: the latter omits
// the very entries this needs to refuse on.
func (f permissionsFS) allowedThroughout(ctx context.Context, name string, check func(Permissions) bool) (bool, error) {
	info, err := f.fs.Stat(ctx, name)
	if err != nil {
		if os.IsNotExist(err) {
			// Nothing to walk; the request fails later on its own terms.
			return true, nil
		}

		return false, err
	}

	if !info.IsDir() {
		return true, nil
	}

	file, err := f.fs.OpenFile(ctx, name, os.O_RDONLY, 0)
	if err != nil {
		return false, err
	}

	entries, err := file.Readdir(-1)
	err = errors.Join(err, file.Close())
	if err != nil {
		return false, err
	}

	for _, entry := range entries {
		child := path.Join(name, entry.Name())

		if !f.perms.allowedAt(child, check) {
			return false, nil
		}

		if entry.IsDir() {
			ok, err := f.allowedThroughout(ctx, child, check)
			if !ok || err != nil {
				return ok, err
			}
		}
	}

	return true, nil
}
