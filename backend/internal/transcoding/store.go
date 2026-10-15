package transcoding

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// Store manages disk-backed HLS cache directories.
type Store struct {
	root string
}

func NewStore(root string) (*Store, error) {
	if root == "" {
		root = settings.TranscodeCacheDir()
	}
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	return &Store{root: root}, nil
}

func (s *Store) Root() string {
	return s.root
}

func (s *Store) JobDir(fingerprint, generation string) string {
	return filepath.Join(s.root, fingerprint, generation)
}

func (s *Store) EnsureJobDir(fingerprint, generation string) (string, error) {
	dir := s.JobDir(fingerprint, generation)
	if err := os.MkdirAll(filepath.Join(dir, "seg"), 0o755); err != nil {
		return "", err
	}
	marker := filepath.Join(s.root, fingerprint, ".owner")
	if err := os.MkdirAll(filepath.Dir(marker), 0o755); err != nil {
		return "", err
	}
	if _, err := os.Stat(marker); os.IsNotExist(err) {
		if writeErr := os.WriteFile(marker, []byte("filebrowser"), 0o644); writeErr != nil {
			return "", writeErr
		}
	}
	return dir, nil
}

func (s *Store) InitPath(jobDir string) string {
	return filepath.Join(jobDir, "init.m4s")
}

func (s *Store) PlaylistPath(jobDir string) string {
	return filepath.Join(jobDir, "ffmpeg.m3u8")
}

func (s *Store) SegmentPath(jobDir, name string) (string, error) {
	path, err := ffmpeg.ResolveHLSSegmentPath(jobDir, name)
	if err != nil {
		return "", ErrInvalidSegment
	}
	return path, nil
}

func (s *Store) SegmentReady(path string) bool {
	name := filepath.Base(path)
	jobDir := filepath.Dir(filepath.Dir(path))
	return ffmpeg.HLSSegmentServeReady(jobDir, name)
}

// SegmentNameReady reports whether a segment has been aligned and is safe to serve.
func (s *Store) SegmentNameReady(jobDir, name string) bool {
	path, err := s.SegmentPath(jobDir, name)
	if err != nil {
		return false
	}
	return s.SegmentReady(path)
}

func (s *Store) InitReady(jobDir string) bool {
	return ffmpeg.HLSInitReady(jobDir)
}

func (s *Store) OpenReadySegment(jobDir, name string) (*os.File, error) {
	path, err := s.SegmentPath(jobDir, name)
	if err != nil {
		return nil, err
	}
	if !s.SegmentReady(path) {
		return nil, ErrNotReady
	}
	return os.Open(path)
}

func (s *Store) OpenInit(jobDir string) (*os.File, error) {
	path := s.InitPath(jobDir)
	if !s.InitReady(jobDir) {
		return nil, ErrNotReady
	}
	if err := s.assertContained(jobDir, path); err != nil {
		return nil, err
	}
	return os.Open(path)
}

// ReadServePlaylist returns the playlist normalized in memory with only the
// segments that are aligned and ready included, plus how many segments it
// exposes. The playlist on disk is left untouched so serving never races with
// ffmpeg rewriting ffmpeg.m3u8.
func (s *Store) ReadServePlaylist(jobDir string) ([]byte, int, error) {
	path := s.PlaylistPath(jobDir)
	if err := s.assertContained(jobDir, path); err != nil {
		return nil, 0, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, err
	}
	out, ready := buildServePlaylist(raw, func(name string) bool {
		return s.SegmentNameReady(jobDir, name)
	})
	return out, ready, nil
}

// IsCompleteGeneration reports whether a generation directory holds a finished,
// fully aligned HLS output that can be served without running ffmpeg again.
func (s *Store) IsCompleteGeneration(jobDir string) bool {
	if jobDir == "" || !s.InitReady(jobDir) {
		return false
	}
	raw, err := os.ReadFile(s.PlaylistPath(jobDir))
	if err != nil {
		return false
	}
	return playlistIsComplete(raw, func(name string) bool {
		return s.SegmentNameReady(jobDir, name)
	})
}

// FindReusableGeneration returns the most recently modified complete generation
// under fingerprint. Generations are keyed only by file identity and transcode
// parameters, so any user playing the same file with the same profile reuses the
// same cached playlist and segments.
func (s *Store) FindReusableGeneration(fingerprint string) (string, bool) {
	if fingerprint == "" {
		return "", false
	}
	fpDir := filepath.Join(s.root, fingerprint)
	entries, err := os.ReadDir(fpDir)
	if err != nil {
		return "", false
	}
	var best string
	var bestMod time.Time
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		genDir := filepath.Join(fpDir, entry.Name())
		if !s.IsCompleteGeneration(genDir) {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		if best == "" || info.ModTime().After(bestMod) {
			best = genDir
			bestMod = info.ModTime()
		}
	}
	return best, best != ""
}

// TouchGeneration refreshes a generation's modification time so active cache
// entries are not evicted by retention while they are still being watched.
func (s *Store) TouchGeneration(jobDir string) {
	if jobDir == "" {
		return
	}
	now := time.Now()
	_ = os.Chtimes(jobDir, now, now)
}

func (s *Store) RemoveGeneration(jobDir string) error {
	if err := s.assertContained(s.root, jobDir); err != nil {
		return err
	}
	return os.RemoveAll(jobDir)
}

func (s *Store) assertContained(base, target string) error {
	baseAbs, err := filepath.Abs(base)
	if err != nil {
		return ErrInvalidPath
	}
	targetAbs, err := filepath.Abs(target)
	if err != nil {
		return ErrInvalidPath
	}
	rel, err := filepath.Rel(baseAbs, targetAbs)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return ErrInvalidPath
	}
	if !strings.HasPrefix(targetAbs, baseAbs+string(os.PathSeparator)) && targetAbs != baseAbs {
		return ErrInvalidPath
	}
	return nil
}

func (s *Store) ReconcileStartup() error {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fpDir := filepath.Join(s.root, entry.Name())
		genEntries, readErr := os.ReadDir(fpDir)
		if readErr != nil {
			continue
		}
		for _, gen := range genEntries {
			if !gen.IsDir() {
				continue
			}
			genPath := filepath.Join(fpDir, gen.Name())
			if !s.InitReady(genPath) {
				_ = os.RemoveAll(genPath)
			}
		}
	}
	return nil
}

func (s *Store) String() string {
	return fmt.Sprintf("transcode-store(%s)", s.root)
}
