package lib

import (
	"context"
	"os"
	"path"

	"golang.org/x/net/webdav"
)

// A Depth 1 PROPFIND already obtains child metadata from Readdir. Reuse that
// snapshot for the handler's subsequent Stat calls, only within this request.
// OpenFile remains a real open so filesystem access checks are preserved.
type propfindFS struct {
	webdav.FileSystem
	infos map[string]os.FileInfo
}

func (f *propfindFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	name = cleanPath(name)
	if info, ok := f.infos[name]; ok {
		return info, nil
	}
	info, err := f.FileSystem.Stat(ctx, name)
	if err == nil {
		f.infos[name] = info
	}
	return info, err
}

func (f *propfindFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	file, err := f.FileSystem.OpenFile(ctx, name, flag, perm)
	if err != nil {
		return nil, err
	}
	return propfindFile{File: file, fs: f, name: cleanPath(name), ctx: ctx}, nil
}

type propfindFile struct {
	webdav.File
	fs   *propfindFS
	name string
	ctx  context.Context
}

func (f propfindFile) Stat() (os.FileInfo, error) {
	if err := f.ctx.Err(); err != nil {
		return nil, err
	}
	if info, ok := f.fs.infos[f.name]; ok {
		return info, nil
	}
	return f.File.Stat()
}

func (f propfindFile) Readdir(count int) ([]os.FileInfo, error) {
	infos, err := f.File.Readdir(count)
	for _, info := range infos {
		// Readdir reports a link itself; Stat must still resolve its target.
		if info.Mode()&os.ModeSymlink == 0 {
			f.fs.infos[path.Join(f.name, info.Name())] = info
		}
	}
	return infos, err
}
