package transcoding

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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

func (s *Store) ReadPlaylist(jobDir string) ([]byte, error) {
	if err := ffmpeg.NormalizeHLSPlaylist(jobDir); err != nil {
		return nil, err
	}
	path := s.PlaylistPath(jobDir)
	if err := s.assertContained(jobDir, path); err != nil {
		return nil, err
	}
	return os.ReadFile(path)
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
