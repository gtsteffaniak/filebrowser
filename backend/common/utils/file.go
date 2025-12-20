package utils

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"hash"
	"io"
	"os"

	"github.com/gtsteffaniak/filebrowser/backend/common/errors"
)

// GetChecksum calculates the checksum of a file using the specified algorithm.
// Returns the checksum as a hex-encoded string.
func GetChecksum(fullPath, algo string) (string, error) {
	reader, err := os.Open(fullPath)
	if err != nil {
		return "", err
	}
	defer reader.Close()

	hashFuncs := map[string]hash.Hash{
		"md5":    md5.New(),
		"sha1":   sha1.New(),
		"sha256": sha256.New(),
		"sha512": sha512.New(),
	}

	h, ok := hashFuncs[algo]
	if !ok {
		return "", errors.ErrInvalidOption
	}

	_, err = io.Copy(h, reader)
	if err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

// FileOptions are the options when getting a file info.
type FileOptions struct {
	Username                 string // username for access control
	Path                     string // realpath
	Source                   string
	IsDir                    bool
	Expand                   bool
	ReadHeader               bool
	Content                  bool
	Recursive                bool // whether to recursively index directories
	Metadata                 bool // whether to get metadata
	ExtractEmbeddedSubtitles bool // whether to extract embedded subtitles from media files
	AlbumArt                 bool // whether to get album art from media files
}
