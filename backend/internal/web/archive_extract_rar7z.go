package web

import (
	"errors"
	"fmt"
	"hash"
	"hash/crc32"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bodgit/sevenzip"
	"github.com/gtsteffaniak/go-logger/logger"
	"github.com/nwaples/rardecode/v2"
)

var (
	errArchivePasswordRequired = errors.New("archive is encrypted, a password is required")
	errArchiveBadPassword      = errors.New("incorrect password or corrupt archive")
	errArchiveChecksum         = errors.New("archive entry checksum mismatch")
	errArchiveVolumeDenied     = errors.New("access denied to archive volume")
)

// maxArchivedSymlinkTarget bounds symlink targets stored as entry data (RAR4, 7z).
const maxArchivedSymlinkTarget = 4096

// volumeFS opens archive volumes from one directory, refusing names the user cannot access.
// Missing files still report fs.ErrNotExist so the reader can probe volume naming schemes.
type volumeFS struct {
	dir     fs.FS
	allowed func(name string) bool
}

func (v volumeFS) Open(name string) (fs.File, error) {
	f, err := v.dir.Open(name)
	if err != nil {
		return nil, err
	}
	if v.allowed != nil && !v.allowed(name) {
		_ = f.Close()
		return nil, fmt.Errorf("%w: %s", errArchiveVolumeDenied, name)
	}
	return f, nil
}

// entryReader keeps the error from reading archive data, which extractArchivedRegularFile
// replaces with a generic message, and optionally verifies a CRC32 at EOF.
type entryReader struct {
	r       io.Reader
	crc     hash.Hash32
	wantCRC uint32
	err     error
}

func (e *entryReader) Read(p []byte) (int, error) {
	n, err := e.r.Read(p)
	if e.crc != nil && n > 0 {
		_, _ = e.crc.Write(p[:n])
	}
	if err == io.EOF && e.crc != nil && e.crc.Sum32() != e.wantCRC {
		err = errArchiveChecksum
	}
	if err != nil && err != io.EOF {
		e.err = err
	}
	return n, err
}

func (e *entryReader) Close() error { return nil }

// entryError prefers the underlying archive read error over the generic write error.
// When reading archive data failed (bad password, corrupt data), the partly written
// file at destPath is removed so it is not mistaken for extracted content.
func (e *entryReader) entryError(destPath string, writeErr error) error {
	if e.err == nil {
		return writeErr
	}
	if err := os.Remove(destPath); err != nil && !os.IsNotExist(err) {
		logger.Debugf("unarchive: remove partial file: %v", err)
	}
	return e.err
}

// readSymlinkTarget reads a symlink target stored as entry data.
func readSymlinkTarget(r io.Reader) (string, error) {
	buf, err := io.ReadAll(io.LimitReader(r, maxArchivedSymlinkTarget+1))
	if err != nil {
		return "", err
	}
	if len(buf) > maxArchivedSymlinkTarget {
		return "", fmt.Errorf("symlink target too long")
	}
	return string(buf), nil
}

// rarEntryKind classifies a RAR header for extraction.
type rarEntryKind int

const (
	rarEntrySkip rarEntryKind = iota
	rarEntryDir
	rarEntryFile
	rarEntrySymlink
	rarEntryHardLink
)

func classifyRarEntry(h *rardecode.FileHeader) rarEntryKind {
	switch {
	case h.IsDir:
		return rarEntryDir
	case h.LinkType == rardecode.LinkTypeHardLink || h.LinkType == rardecode.LinkTypeFileCopy:
		return rarEntryHardLink
	case h.LinkType == rardecode.LinkTypeWindowsJunction:
		// Junctions always point at absolute Windows paths.
		return rarEntrySkip
	case h.LinkType == rardecode.LinkTypeUnixSymlink, h.LinkType == rardecode.LinkTypeWindowsSymlink:
		return rarEntrySymlink
	case h.Mode()&fs.ModeSymlink != 0:
		// RAR4 unix symlinks store the target as entry data.
		return rarEntrySymlink
	default:
		return rarEntryFile
	}
}

func rarOptions(archivePath, password string, volumeAllowed func(string) bool) (string, []rardecode.Option) {
	opts := []rardecode.Option{
		rardecode.FileSystem(volumeFS{dir: os.DirFS(filepath.Dir(archivePath)), allowed: volumeAllowed}),
	}
	if password != "" {
		opts = append(opts, rardecode.Password(password))
	}
	return filepath.Base(archivePath), opts
}

// classifyRarError maps rardecode errors to password errors the handler reports to the client.
func classifyRarError(err error, password string) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, rardecode.ErrArchiveEncrypted), errors.Is(err, rardecode.ErrArchivedFileEncrypted):
		if password == "" {
			return errArchivePasswordRequired
		}
		return errArchiveBadPassword
	case errors.Is(err, rardecode.ErrBadPassword):
		return errArchiveBadPassword
	case password != "" && errors.Is(err, rardecode.ErrBadFileChecksum):
		// RAR4 has no password check value; a wrong password shows up as bad file data.
		return errArchiveBadPassword
	}
	return err
}

// extractRar extracts a RAR archive (including multi-volume and solid archives) into destDir.
// volumeAllowed is consulted for every volume file opened next to archivePath.
// It returns the volume file names (relative to the archive directory) that were read.
func extractRar(archivePath, destDir, password string, policy *extractAccessPolicy, volumeAllowed func(string) bool) ([]string, error) {
	name, opts := rarOptions(archivePath, password, volumeAllowed)

	// Validate every entry from the headers before writing anything.
	files, err := rardecode.List(name, opts...)
	if err != nil {
		return nil, classifyRarError(err, password)
	}
	for _, f := range files {
		kind := classifyRarEntry(&f.FileHeader)
		if kind == rarEntrySkip {
			continue
		}
		destPath, pathErr := safeExtractPath(destDir, f.Name)
		if pathErr != nil {
			return nil, pathErr
		}
		if err = preflightExtractEntry(policy, f.Name, destPath, kind == rarEntryDir, kind == rarEntrySymlink); err != nil {
			return nil, err
		}
	}

	rc, err := rardecode.OpenReader(name, opts...)
	if err != nil {
		return nil, classifyRarError(err, password)
	}
	defer rc.Close()

	// Regular files written so far, for hard links to point at.
	extracted := make(map[string]os.FileInfo)
	for {
		h, nextErr := rc.Next()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, classifyRarError(nextErr, password)
		}
		if err = extractRarEntry(destDir, h, password, &rc.Reader, extracted); err != nil {
			return nil, classifyRarError(err, password)
		}
	}
	return rc.Volumes(), nil
}

func extractRarEntry(destDir string, h *rardecode.FileHeader, password string, data io.Reader, extracted map[string]os.FileInfo) error {
	kind := classifyRarEntry(h)
	if kind == rarEntrySkip {
		return nil
	}
	destPath, err := safeExtractPath(destDir, h.Name)
	if err != nil {
		return err
	}
	modTime := h.ModificationTime
	switch kind {
	case rarEntryDir:
		return extractArchivedDir(destPath, h.Mode(), modTime)
	case rarEntrySymlink:
		target := h.LinkTarget
		if target == "" {
			if target, err = readSymlinkTarget(data); err != nil {
				return err
			}
		}
		return extractArchivedSymlink(destDir, destPath, strings.ReplaceAll(target, "\\", "/"), modTime)
	case rarEntryHardLink:
		if err = extractArchivedHardLink(destDir, destPath, h.Name, h.LinkTarget, h.Mode(), modTime, extracted); err != nil {
			return err
		}
	default:
		src := &entryReader{r: data}
		if err = extractArchivedRegularFile(destPath, h.Mode(), modTime, src); err != nil {
			err = src.entryError(destPath, err)
			if src.err != nil && h.Encrypted && password != "" {
				// RAR4 has no password check value; a wrong password surfaces as any decode error.
				return errArchiveBadPassword
			}
			return err
		}
	}
	recordExtractedFile(extracted, destPath)
	return nil
}

// classify7zError maps sevenzip errors to password errors the handler reports to the client.
func classify7zError(err error, password string) error {
	if err == nil {
		return nil
	}
	var readErr *sevenzip.ReadError
	if errors.As(err, &readErr) && readErr.Encrypted {
		if password == "" {
			return errArchivePasswordRequired
		}
		return errArchiveBadPassword
	}
	if errors.Is(err, errArchiveChecksum) {
		// Store-only encrypted entries decrypt to garbage instead of failing to decompress,
		// and an empty password is valid in 7z, so a mismatch is the only signal.
		return errArchiveBadPassword
	}
	return err
}

// extract7z extracts a single-volume 7z archive into destDir.
func extract7z(archivePath, destDir, password string, policy *extractAccessPolicy) error {
	r, err := sevenzip.OpenReaderWithPassword(archivePath, password)
	if err != nil {
		return classify7zError(err, password)
	}
	defer r.Close()

	// Validate every entry before writing anything.
	for _, f := range r.File {
		mode := f.FileInfo().Mode()
		isDir, isSymlink := mode.IsDir(), mode.Type() == fs.ModeSymlink
		if !isDir && !isSymlink && !mode.IsRegular() {
			continue
		}
		destPath, pathErr := safeExtractPath(destDir, f.Name)
		if pathErr != nil {
			return pathErr
		}
		if err = preflightExtractEntry(policy, f.Name, destPath, isDir, isSymlink); err != nil {
			return err
		}
	}

	// Archive order lets the reader reuse decompressors across files in a solid block.
	for _, f := range r.File {
		if err = extract7zEntry(destDir, f); err != nil {
			return classify7zError(err, password)
		}
	}
	return nil
}

func extract7zEntry(destDir string, f *sevenzip.File) error {
	mode := f.FileInfo().Mode()
	if !mode.IsDir() && mode.Type() != fs.ModeSymlink && !mode.IsRegular() {
		return nil
	}
	destPath, err := safeExtractPath(destDir, f.Name)
	if err != nil {
		return err
	}
	modTime := f.Modified
	if mode.IsDir() {
		return extractArchivedDir(destPath, mode, modTime)
	}

	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()
	src := &entryReader{r: rc}
	if f.CRC32 != 0 {
		src.crc, src.wantCRC = crc32.NewIEEE(), f.CRC32
	}
	if mode.Type() == fs.ModeSymlink {
		target, readErr := readSymlinkTarget(src)
		if readErr != nil {
			return readErr
		}
		return extractArchivedSymlink(destDir, destPath, target, modTime)
	}
	if err = extractArchivedRegularFile(destPath, mode, modTime, src); err != nil {
		return src.entryError(destPath, err)
	}
	return nil
}
