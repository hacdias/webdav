package lib

import (
	"context"
	"os"
	"path"

	"golang.org/x/net/webdav"
)

// propfindFS serves Stat from the Readdir of the parent, which a PROPFIND
// otherwise repeats per entry. It must only live for a single request.
type propfindFS struct {
	webdav.FileSystem
	infos map[string]os.FileInfo
}

func (f propfindFS) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	if info, ok := f.infos[name]; ok {
		return info, nil
	}

	return f.FileSystem.Stat(ctx, name)
}

func (f propfindFS) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	file, err := f.FileSystem.OpenFile(ctx, name, flag, perm)
	if err != nil {
		return nil, err
	}

	return propfindFile{File: file, name: name, infos: f.infos}, nil
}

type propfindFile struct {
	webdav.File
	name  string
	infos map[string]os.FileInfo
}

// Stat is the last lookup of an entry, so it is dropped to bound memory.
func (f propfindFile) Stat() (os.FileInfo, error) {
	if info, ok := f.infos[f.name]; ok {
		delete(f.infos, f.name)
		return info, nil
	}

	return f.File.Stat()
}

func (f propfindFile) Readdir(count int) ([]os.FileInfo, error) {
	fis, err := f.File.Readdir(count)
	for _, fi := range fis {
		// Readdir does not follow links, Stat does.
		if t := fi.Mode().Type(); t == 0 || t == os.ModeDir {
			f.infos[path.Join(f.name, fi.Name())] = fi
		}
	}

	return fis, err
}
