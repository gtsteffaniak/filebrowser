//go:build linux

package indexing

import (
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

// statxFileInfo is an os.FileInfo built from a single statx result (directory listing).
type statxFileInfo struct {
	name     string
	mode     uint16
	size     uint64
	blocks   uint64
	nlink    uint32
	ino      uint64
	mtime    unix.StatxTimestamp
	btime    unix.StatxTimestamp
	hasBtime bool
}

func (s *statxFileInfo) Name() string       { return s.name }
func (s *statxFileInfo) Size() int64        { return int64(s.size) }
func (s *statxFileInfo) Mode() os.FileMode  { return modeFromStatx(s.mode) }
func (s *statxFileInfo) ModTime() time.Time { return time.Unix(s.mtime.Sec, int64(s.mtime.Nsec)) }
func (s *statxFileInfo) IsDir() bool        { return s.mode&unix.S_IFMT == unix.S_IFDIR }
func (s *statxFileInfo) Sys() interface{}   { return s }

func (s *statxFileInfo) birthTime() *time.Time {
	if !s.hasBtime || s.btime.Sec <= 0 {
		return nil
	}
	t := time.Unix(s.btime.Sec, int64(s.btime.Nsec))
	return &t
}

// unixFileDetails provides size/nlink/ino from the statx result so callers never
// need a second path-based stat.
func (s *statxFileInfo) unixFileDetails(useLogicalSize bool) (uint64, uint64, uint64) {
	if useLogicalSize {
		return s.size, uint64(s.nlink), s.ino
	}
	return s.blocks * 512, uint64(s.nlink), s.ino
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

const statxDirListingMask = unix.STATX_TYPE | unix.STATX_MODE | unix.STATX_NLINK |
	unix.STATX_INO | unix.STATX_SIZE | unix.STATX_BLOCKS | unix.STATX_MTIME | unix.STATX_BTIME

var (
	statxDirListingOnce      sync.Once
	statxDirListingSupported bool
)

func statxDirListingAvailable(dirFd int) bool {
	statxDirListingOnce.Do(func() {
		var stx unix.Statx_t
		err := unix.Statx(dirFd, ".", unix.AT_SYMLINK_NOFOLLOW, statxDirListingMask, &stx)
		// Only trust statx when the probe fully succeeds; seccomp/container
		// filters (EPERM/EACCES) and missing kernel support (ENOSYS/EOPNOTSUPP)
		// must fall back to fstatat.
		statxDirListingSupported = err == nil
	})
	return statxDirListingSupported
}

// dirStatParallelThreshold is the minimum directory size where concurrent
// per-entry stat calls pay off.
const dirStatParallelThreshold = 256

// dirStatMaxWorkers bounds concurrent statx/fstatat calls so wide directories on
// slow filesystems do not overwhelm the kernel or I/O stack.
const dirStatMaxWorkers = 16

func readDirFileInfos(dir *os.File) ([]os.FileInfo, error) {
	names, err := dir.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	dirFd := int(dir.Fd())
	useStatx := statxDirListingAvailable(dirFd)

	if len(names) < dirStatParallelThreshold {
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

	return statDirEntriesParallel(dirFd, names, useStatx)
}

// statDirEntriesParallel stats each name relative to dirFd across a bounded
// worker pool. Entry order matches names; entries that vanished are skipped.
func statDirEntriesParallel(dirFd int, names []string, useStatx bool) ([]os.FileInfo, error) {
	workers := runtime.GOMAXPROCS(0) * 2
	if workers > dirStatMaxWorkers {
		workers = dirStatMaxWorkers
	}
	if workers > len(names) {
		workers = len(names)
	}

	results := make([]os.FileInfo, len(names))
	var next atomic.Int64
	var errMu sync.Mutex
	var firstErr error
	recordErr := func(err error) {
		errMu.Lock()
		if firstErr == nil {
			firstErr = err
		}
		errMu.Unlock()
	}
	hasErr := func() bool {
		errMu.Lock()
		defer errMu.Unlock()
		return firstErr != nil
	}

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				if hasErr() {
					return
				}
				i := int(next.Add(1)) - 1
				if i >= len(names) {
					return
				}
				fi, err := statDirEntry(dirFd, names[i], useStatx)
				switch {
				case err == nil:
					results[i] = fi
				case os.IsNotExist(err):
					// Entry disappeared between readdir and stat; leave nil.
				default:
					recordErr(err)
					return
				}
			}
		}()
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}
	infos := make([]os.FileInfo, 0, len(names))
	for _, fi := range results {
		if fi != nil {
			infos = append(infos, fi)
		}
	}
	return infos, nil
}

func statDirEntry(dirFd int, name string, useStatx bool) (os.FileInfo, error) {
	if useStatx {
		var stx unix.Statx_t
		err := unix.Statx(dirFd, name, unix.AT_SYMLINK_NOFOLLOW, statxDirListingMask, &stx)
		if err == nil {
			return &statxFileInfo{
				name:     name,
				mode:     stx.Mode,
				size:     stx.Size,
				blocks:   stx.Blocks,
				nlink:    stx.Nlink,
				ino:      stx.Ino,
				mtime:    stx.Mtime,
				btime:    stx.Btime,
				hasBtime: stx.Mask&unix.STATX_BTIME != 0,
			}, nil
		}
		if err != unix.ENOSYS && err != syscall.EOPNOTSUPP {
			return nil, err
		}
		// statx probe succeeded but this call lacks support (e.g. filesystem
		// restrictions): fall through to fstatat for this entry.
	}

	var st unix.Stat_t
	if err := unix.Fstatat(dirFd, name, &st, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return nil, err
	}
	return &fstatatFileInfo{name: name, stat: st}, nil
}

// fstatatFileInfo is used when the kernel does not support statx (no birth time).
type fstatatFileInfo struct {
	name string
	stat unix.Stat_t
}

func (f *fstatatFileInfo) Name() string      { return f.name }
func (f *fstatatFileInfo) Size() int64       { return f.stat.Size }
func (f *fstatatFileInfo) Mode() os.FileMode { return modeFromStatx(uint16(f.stat.Mode)) }
func (f *fstatatFileInfo) ModTime() time.Time {
	return time.Unix(int64(f.stat.Mtim.Sec), int64(f.stat.Mtim.Nsec))
}
func (f *fstatatFileInfo) IsDir() bool      { return f.stat.Mode&unix.S_IFMT == unix.S_IFDIR }
func (f *fstatatFileInfo) Sys() interface{} { return f }

// unixFileDetails provides size/nlink/ino from the fstatat result so callers
// never need a second path-based stat.
func (f *fstatatFileInfo) unixFileDetails(useLogicalSize bool) (uint64, uint64, uint64) {
	if useLogicalSize {
		return uint64(f.stat.Size), uint64(f.stat.Nlink), f.stat.Ino
	}
	return uint64(f.stat.Blocks) * 512, uint64(f.stat.Nlink), f.stat.Ino
}
