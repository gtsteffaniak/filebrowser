package fileutils

import (
	"bytes"
	"io/fs"
	"time"
)

// memFS is a read-only fs.FS that holds every file's contents in memory.
// It is built once at startup from a disk-backed filesystem so that asset
// requests never touch the disk.
type memFS struct {
	files map[string]memFile
}

type memFile struct {
	data    []byte
	modTime time.Time
}

// loadMemFS reads every regular file under src into memory.
func loadMemFS(src fs.FS) (*memFS, error) {
	m := &memFS{files: make(map[string]memFile)}
	err := fs.WalkDir(src, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(src, name)
		if err != nil {
			return err
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		m.files[name] = memFile{data: data, modTime: info.ModTime()}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return m, nil
}

// Size returns the total number of bytes held in memory.
func (m *memFS) Size() int {
	total := 0
	for _, f := range m.files {
		total += len(f.data)
	}
	return total
}

// Len returns the number of files held in memory.
func (m *memFS) Len() int {
	return len(m.files)
}

// Open implements fs.FS. Only regular files are supported, which is all the
// asset callers need (ReadFile, Stat and template parsing).
func (m *memFS) Open(name string) (fs.File, error) {
	if !fs.ValidPath(name) {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrInvalid}
	}
	f, ok := m.files[name]
	if !ok {
		return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
	}
	return &memOpenFile{
		Reader: bytes.NewReader(f.data),
		info: memFileInfo{
			name:    baseName(name),
			size:    int64(len(f.data)),
			modTime: f.modTime,
		},
	}, nil
}

type memOpenFile struct {
	*bytes.Reader
	info memFileInfo
}

func (f *memOpenFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *memOpenFile) Close() error               { return nil }

type memFileInfo struct {
	name    string
	size    int64
	modTime time.Time
}

func (i memFileInfo) Name() string       { return i.name }
func (i memFileInfo) Size() int64        { return i.size }
func (i memFileInfo) Mode() fs.FileMode  { return 0o444 }
func (i memFileInfo) ModTime() time.Time { return i.modTime }
func (i memFileInfo) IsDir() bool        { return false }
func (i memFileInfo) Sys() any           { return nil }

func baseName(name string) string {
	for i := len(name) - 1; i >= 0; i-- {
		if name[i] == '/' {
			return name[i+1:]
		}
	}
	return name
}
