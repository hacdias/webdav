package lib

import (
	"context"
	"io"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/net/webdav"
)

var _ webdav.FileSystem = multiDir{}

type multiDir struct {
	mounts  DirectoryMounts
	noSniff bool
}

func (m multiDir) Mkdir(ctx context.Context, name string, perm os.FileMode) error {
	mount, rest, err := m.resolve(name)
	if err != nil {
		return err
	}
	if rest == "/" {
		return os.ErrExist
	}

	return mount.dir(m.noSniff).Mkdir(ctx, rest, perm)
}

func (m multiDir) OpenFile(ctx context.Context, name string, flag int, perm os.FileMode) (webdav.File, error) {
	if cleanName(name) == "/" {
		if writeFlag(flag) {
			return nil, os.ErrPermission
		}

		return &multiDirRootFile{
			entries: m.rootEntries(ctx),
			info:    virtualDirInfo{name: "/"},
		}, nil
	}

	mount, rest, err := m.resolve(name)
	if err != nil {
		return nil, err
	}
	if rest == "/" && writeFlag(flag) {
		return nil, os.ErrPermission
	}

	file, err := mount.dir(m.noSniff).OpenFile(ctx, rest, flag, perm)
	if err != nil {
		return nil, err
	}

	if rest == "/" {
		return mountRootFile{File: file, name: mount.Name}, nil
	}
	return file, nil
}

func (m multiDir) RemoveAll(ctx context.Context, name string) error {
	if cleanName(name) == "/" {
		return os.ErrInvalid
	}

	mount, rest, err := m.resolve(name)
	if err != nil {
		return err
	}
	if rest == "/" {
		return os.ErrInvalid
	}

	return mount.dir(m.noSniff).RemoveAll(ctx, rest)
}

func (m multiDir) Rename(ctx context.Context, oldName, newName string) error {
	oldMount, oldRest, err := m.resolve(oldName)
	if err != nil {
		return err
	}
	newMount, newRest, err := m.resolve(newName)
	if err != nil {
		return err
	}
	if oldRest == "/" || newRest == "/" {
		return os.ErrInvalid
	}

	if oldMount.Name == newMount.Name {
		return oldMount.dir(m.noSniff).Rename(ctx, oldRest, newRest)
	}

	oldPath := oldMount.filePath(oldRest)
	newPath := newMount.filePath(newRest)
	return os.Rename(oldPath, newPath)
}

func (m multiDir) Stat(ctx context.Context, name string) (os.FileInfo, error) {
	if cleanName(name) == "/" {
		return virtualDirInfo{name: "/"}, nil
	}

	mount, rest, err := m.resolve(name)
	if err != nil {
		return nil, err
	}

	info, err := mount.dir(m.noSniff).Stat(ctx, rest)
	if err != nil {
		return nil, err
	}
	if rest == "/" {
		return namedFileInfo{FileInfo: info, name: mount.Name}, nil
	}
	return info, nil
}

func (m multiDir) resolve(name string) (DirectoryMount, string, error) {
	name = cleanName(name)
	if name == "/" {
		return DirectoryMount{}, "", os.ErrInvalid
	}

	trimmed := strings.TrimPrefix(name, "/")
	mountName, rest, _ := strings.Cut(trimmed, "/")
	for _, mount := range m.mounts {
		if mount.Name == mountName {
			if rest == "" {
				return mount, "/", nil
			}
			return mount, "/" + rest, nil
		}
	}

	return DirectoryMount{}, "", os.ErrNotExist
}

func (m multiDir) rootEntries(ctx context.Context) []os.FileInfo {
	entries := make([]os.FileInfo, 0, len(m.mounts))
	for _, mount := range m.mounts {
		info, err := mount.dir(m.noSniff).Stat(ctx, "/")
		if err != nil {
			entries = append(entries, virtualDirInfo{name: mount.Name})
			continue
		}
		entries = append(entries, namedFileInfo{FileInfo: info, name: mount.Name})
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	return entries
}

func (d DirectoryMount) dir(noSniff bool) Dir {
	return Dir{
		Dir:     webdav.Dir(d.Path),
		noSniff: noSniff,
	}
}

func (d DirectoryMount) filePath(name string) string {
	return filepath.Join(d.Path, filepath.FromSlash(strings.TrimPrefix(name, "/")))
}

func cleanName(name string) string {
	if name == "" || !strings.HasPrefix(name, "/") {
		name = "/" + name
	}
	return path.Clean(name)
}

func writeFlag(flag int) bool {
	return flag&(os.O_WRONLY|os.O_RDWR|os.O_CREATE|os.O_TRUNC|os.O_APPEND) != 0
}

type multiDirRootFile struct {
	entries []os.FileInfo
	info    os.FileInfo
	offset  int
}

func (f *multiDirRootFile) Close() error {
	return nil
}

func (f *multiDirRootFile) Read([]byte) (int, error) {
	return 0, io.EOF
}

func (f *multiDirRootFile) Seek(offset int64, whence int) (int64, error) {
	var next int64
	switch whence {
	case io.SeekStart:
		next = offset
	case io.SeekCurrent:
		next = int64(f.offset) + offset
	case io.SeekEnd:
		next = int64(len(f.entries)) + offset
	default:
		return 0, os.ErrInvalid
	}
	if next < 0 {
		return 0, os.ErrInvalid
	}
	f.offset = int(next)
	return next, nil
}

func (f *multiDirRootFile) Readdir(count int) ([]os.FileInfo, error) {
	if count <= 0 {
		entries := f.entries[f.offset:]
		f.offset = len(f.entries)
		return entries, nil
	}

	if f.offset >= len(f.entries) {
		return nil, io.EOF
	}

	end := f.offset + count
	if end > len(f.entries) {
		end = len(f.entries)
	}
	entries := f.entries[f.offset:end]
	f.offset = end
	return entries, nil
}

func (f *multiDirRootFile) Stat() (os.FileInfo, error) {
	return f.info, nil
}

func (f *multiDirRootFile) Write([]byte) (int, error) {
	return 0, os.ErrPermission
}

type mountRootFile struct {
	webdav.File
	name string
}

func (f mountRootFile) Stat() (os.FileInfo, error) {
	info, err := f.File.Stat()
	if err != nil {
		return nil, err
	}
	return namedFileInfo{FileInfo: info, name: f.name}, nil
}

type namedFileInfo struct {
	os.FileInfo
	name string
}

func (i namedFileInfo) Name() string {
	return i.name
}

type virtualDirInfo struct {
	name string
}

func (i virtualDirInfo) Name() string {
	return i.name
}

func (i virtualDirInfo) Size() int64 {
	return 0
}

func (i virtualDirInfo) Mode() os.FileMode {
	return os.ModeDir | 0555
}

func (i virtualDirInfo) ModTime() time.Time {
	return time.Time{}
}

func (i virtualDirInfo) IsDir() bool {
	return true
}

func (i virtualDirInfo) Sys() any {
	return nil
}
