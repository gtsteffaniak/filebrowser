package rifs

import (
	"os"
	"time"
)

// SimpleFileInfo is a simple `os.FileInfo` implementation useful for testing
// with the bare minimum.
type SimpleFileInfo struct {
	filename string
	isDir    bool
	size     int64
	mode     os.FileMode
	modTime  time.Time
}

// NewSimpleFileInfoWithFile returns a new file-specific SimpleFileInfo.
func NewSimpleFileInfoWithFile(filename string, size int64, mode os.FileMode, modTime time.Time) *SimpleFileInfo {
	return &SimpleFileInfo{
		filename: filename,
		isDir:    false,
		size:     size,
		mode:     mode,
		modTime:  modTime,
	}
}

// NewSimpleFileInfoWithDirectory returns a new directory-specific
// SimpleFileInfo.
func NewSimpleFileInfoWithDirectory(filename string, modTime time.Time) *SimpleFileInfo {
	return &SimpleFileInfo{
		filename: filename,
		isDir:    true,
		mode:     os.ModeDir,
		modTime:  modTime,
	}
}

// Name returns the base name of the file.
func (sfi *SimpleFileInfo) Name() string {
	return sfi.filename
}

// Size returns the length in bytes for regular files; system-dependent for
// others.
func (sfi *SimpleFileInfo) Size() int64 {
	return sfi.size
}

// Mode returns the file mode bits.
func (sfi *SimpleFileInfo) Mode() os.FileMode {
	return sfi.mode
}

// ModTime returns the modification time.
func (sfi *SimpleFileInfo) ModTime() time.Time {
	return sfi.modTime
}

// IsDir returns true if a directory.
func (sfi *SimpleFileInfo) IsDir() bool {
	return sfi.isDir
}

// Sys returns internal state.
func (sfi *SimpleFileInfo) Sys() interface{} {
	return nil
}
