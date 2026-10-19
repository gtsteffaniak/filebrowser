//go:build linux

package indexing

import (
	"os"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// statxFileInfo is an os.FileInfo built from a single statx result (directory listing).
type statxFileInfo struct {
	name     string
	mode     uint16
	size     uint64
	mtime    unix.StatxTimestamp
	btime    unix.StatxTimestamp
	hasBtime bool
}

func (s *statxFileInfo) Name() string       { return s.name }
func (s *statxFileInfo) Size() int64        { return int64(s.size) }
func (s *statxFileInfo) Mode() os.FileMode  { return modeFromStatx(s.mode) }
func (s *statxFileInfo) ModTime() time.Time { return time.Unix(s.mtime.Sec, int64(s.mtime.Nsec)) }
func (s *statxFileInfo) IsDir() bool        { return s.mode&unix.S_IFDIR != 0 }
func (s *statxFileInfo) Sys() interface{}   { return s }

func (s *statxFileInfo) birthTime() *time.Time {
	if s.hasBtime && s.btime.Sec > 0 {
		t := time.Unix(s.btime.Sec, int64(s.btime.Nsec))
		return &t
	}
	t := s.ModTime()
	return &t
}

func modeFromStatx(mode uint16) os.FileMode {
	fm := os.FileMode(mode & 0o777)
	switch mode & 0o170000 {
	case unix.S_IFBLK:
		fm |= os.ModeDevice
	case unix.S_IFCHR:
		fm |= os.ModeDevice | os.ModeCharDevice
	case unix.S_IFDIR:
		fm |= os.ModeDir
	case unix.S_IFIFO:
		fm |= os.ModeNamedPipe
	case unix.S_IFLNK:
		fm |= os.ModeSymlink
	case unix.S_IFSOCK:
		fm |= os.ModeSocket
	}
	if mode&unix.S_ISGID != 0 {
		fm |= os.ModeSetgid
	}
	if mode&unix.S_ISUID != 0 {
		fm |= os.ModeSetuid
	}
	if mode&unix.S_ISVTX != 0 {
		fm |= os.ModeSticky
	}
	return fm
}

const statxDirListingMask = unix.STATX_TYPE | unix.STATX_MODE | unix.STATX_SIZE | unix.STATX_MTIME | unix.STATX_BTIME

var (
	statxDirListingOnce      sync.Once
	statxDirListingSupported bool
)

func statxDirListingAvailable(dirFd int) bool {
	statxDirListingOnce.Do(func() {
		var stx unix.Statx_t
		err := unix.Statx(dirFd, ".", unix.AT_SYMLINK_NOFOLLOW, statxDirListingMask, &stx)
		statxDirListingSupported = err != unix.ENOSYS && err != syscall.EOPNOTSUPP
	})
	return statxDirListingSupported
}

func readDirFileInfos(dir *os.File) ([]os.FileInfo, error) {
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	dirFd := int(dir.Fd())
	useStatx := statxDirListingAvailable(dirFd)
	infos := make([]os.FileInfo, 0, len(names))
	for _, name := range names {
		fi, err := statDirEntry(dirFd, name, useStatx)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		infos = append(infos, fi)
	}
	return infos, nil
}

func statDirEntry(dirFd int, name string, useStatx bool) (os.FileInfo, error) {
	if useStatx {
		var stx unix.Statx_t
		err := unix.Statx(dirFd, name, unix.AT_SYMLINK_NOFOLLOW, statxDirListingMask, &stx)
		if err != nil {
			return nil, err
		}
		return &statxFileInfo{
			name:     name,
			mode:     stx.Mode,
			size:     stx.Size,
			mtime:    stx.Mtime,
			btime:    stx.Btime,
			hasBtime: stx.Mask&unix.STATX_BTIME != 0,
		}, nil
	}

	var st unix.Stat_t
	if err := unix.Fstatat(dirFd, name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return nil, err
	}
	return &fstatatFileInfo{name: name, stat: st}, nil
}

// fstatatFileInfo is used when the kernel does not support statx (birth time = mtime).
type fstatatFileInfo struct {
	name string
	stat unix.Stat_t
}

func (f *fstatatFileInfo) Name() string       { return f.name }
func (f *fstatatFileInfo) Size() int64        { return f.stat.Size }
func (f *fstatatFileInfo) Mode() os.FileMode  { return modeFromStatx(uint16(f.stat.Mode)) }
func (f *fstatatFileInfo) ModTime() time.Time { return time.Unix(f.stat.Mtim.Sec, f.stat.Mtim.Nsec) }
func (f *fstatatFileInfo) IsDir() bool        { return f.stat.Mode&unix.S_IFDIR != 0 }
func (f *fstatatFileInfo) Sys() interface{}   { return &f.stat }

func (f *fstatatFileInfo) birthTime() *time.Time {
	t := f.ModTime()
	return &t
}
