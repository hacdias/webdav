package lib

import (
	"context"
	"mime"
	"os"
	"path"

	"golang.org/x/net/webdav"
)

type Dir struct {
	webdav.Dir
	noSniff bool
}

func (d Dir) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	// Skip wrapping if NoSniff is off
	if !d.noSniff {
		return d.Dir.Stat(ctx, name)
	}

	info, err := d.Dir.Stat(ctx, name)
	if err != nil {
		return nil, err
	}

	return noSniffFileInfo{info}, nil
}

func (d Dir) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	// Skip wrapping if NoSniff is off
	if !d.noSniff {
		return d.Dir.OpenFile(ctx, name, flag, perm)
	}

	file, err := d.Dir.OpenFile(ctx, name, flag, perm)
	if err != nil {
		return nil, err
	}

	return noSniffFile{File: file}, nil
}

type noSniffFileInfo struct {
	os.FileInfo
}

func (w noSniffFileInfo) ContentType(ctx context.Context) (contentType string, err error) {
	if mimeType := mime.TypeByExtension(path.Ext(w.Name())); mimeType != "" {
		// We can figure out the mime from the extension.
		return mimeType, nil
	} else {
		// We can't figure out the mime type without sniffing, call it an octet stream.
		return "application/octet-stream", nil
	}
}

type noSniffFile struct {
	webdav.File
}

func (f noSniffFile) Stat() (os.FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}

	return noSniffFileInfo{info}, nil
}

func (f noSniffFile) Readdir(count int) (fis []os.FileInfo, err error) {
	fis, err = f.File.Readdir(count)
	if err != nil {
		return nil, err
	}

	for i := range fis {
		fis[i] = noSniffFileInfo{fis[i]}
	}
	return fis, nil
}
