package lib

import (
	"context"
	"log"
	"mime"
	"os"
	"path"

	"golang.org/x/net/webdav"
)

type WebDavFileInfo struct {
	os.FileInfo
	test bool
}

func (w *WebDavFileInfo) ContentType(ctx context.Context) (string, error) {
	// TODO: remove debug logging
	log.Println("NOT sniffing files")
	if w.FileInfo.IsDir() {
		return "inode/directory", nil
	} else if mimeType := mime.TypeByExtension(path.Ext(w.FileInfo.Name())); mimeType != "" {
		return mimeType, nil
	} else {
		return "application/octet-stream", nil
	}
}

type WebDavDir struct {
	webdav.Dir
	NoSniff bool
}

func (d WebDavDir) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	if !d.NoSniff {
		// TODO: remove debug logging
		log.Println("USING THE DEFAULT STAT")
		return d.Dir.Stat(ctx, name)
	}
	// TODO: remove debug logging
	log.Println("USING THE WRAPPED STAT")

	info, err := d.Dir.Stat(ctx, name)

	if err != nil {
		return nil, err
	}

	return WebDavFileInfo{
		FileInfo: info,
		test:     false,
	}, nil
}
