package transcoding

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/gtsteffaniak/go-ffmpeg/mp4"
)

// Store manages disk-backed HLS cache directories.
type Store struct {
	root      string
	initMu    sync.Mutex
	initValid map[string]initStamp
}

// initStamp records the size/mtime of an init.m4s that already passed
// structural validation so polling does not re-read the file.
type initStamp struct {
	size int64
	mod  time.Time
}

// NewStore returns a store rooted at root (or the configured transcode cache
// dir). The directory is created lazily on first transcode so wiring never
// requires the cache tree to exist; every read path tolerates a missing root.
func NewStore(root string) (*Store, error) {
	if root == "" {
		root = settings.TranscodeCacheDir()
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

// PlaylistPath is the synthetic VOD media playlist served to clients. It lists
// the full planned segment grid; per-segment readiness is checked at request
// time, not from the playlist.
func (s *Store) PlaylistPath(jobDir string) string {
	return filepath.Join(jobDir, "index.m3u8")
}

func (s *Store) InitReady(jobDir string) bool {
	path := s.InitPath(jobDir)
	st, err := os.Stat(path)
	if err != nil || st.Size() == 0 {
		return false
	}
	s.initMu.Lock()
	stamp, ok := s.initValid[path]
	s.initMu.Unlock()
	if ok && stamp.size == st.Size() && stamp.mod.Equal(st.ModTime()) {
		return true
	}
	data, err := os.ReadFile(path)
	if err != nil || !mp4.InitSegmentComplete(data) {
		// Only successful validations are cached: a partial init keeps
		// reporting not-ready until ffmpeg finishes writing it.
		return false
	}
	s.initMu.Lock()
	if s.initValid == nil {
		s.initValid = map[string]initStamp{}
	}
	s.initValid[path] = initStamp{size: st.Size(), mod: st.ModTime()}
	s.initMu.Unlock()
	return true
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

// ReadServePlaylist returns the synthetic VOD index.m3u8 verbatim. It is
// written once at generation start and lists the full planned segment grid.
func (s *Store) ReadServePlaylist(jobDir string) ([]byte, error) {
	path := s.PlaylistPath(jobDir)
	if err := s.assertContained(jobDir, path); err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// Callers poll on ErrNotReady instead of treating a missing file as a hard 500.
		if os.IsNotExist(err) {
			return nil, ErrNotReady
		}
		return nil, err
	}
	return data, nil
}

// WriteServePlaylist atomically publishes the host-facing media playlist.
func (s *Store) WriteServePlaylist(jobDir, body string) error {
	if jobDir == "" {
		return ErrInvalidPath
	}
	path := s.PlaylistPath(jobDir)
	if err := s.assertContained(jobDir, path); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(body), 0o644)
}

// WriteVODComplete records that a generation finished encoding all segments.
func (s *Store) WriteVODComplete(jobDir string, durationSec float64) error {
	if jobDir == "" {
		return nil
	}
	path := filepath.Join(jobDir, vodCompleteMarker)
	if err := s.assertContained(s.root, path); err != nil {
		return err
	}
	return os.WriteFile(path, fmt.Appendf(nil, "%.6f\n", durationSec), 0o644)
}

func (s *Store) vodCompleteDuration(jobDir string) (float64, bool) {
	path := filepath.Join(jobDir, vodCompleteMarker)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil || v <= 0 {
		return 0, false
	}
	return v, true
}

// GenerationCoverage describes the produced range of a completed generation
// on the shared segment grid. Segments before FirstIndex were never produced
// (generations that began mid-file).
type GenerationCoverage struct {
	FirstIndex int  // first produced segment index; -1 when nothing was produced
	Complete   bool // the generation ran to EOF (vod.complete marker exists)
}

// GenerationCoverage reads the vod.complete marker plus the produced segment
// files — the only authoritative record under the synthetic playlist model.
func (s *Store) GenerationCoverage(jobDir string) GenerationCoverage {
	cov := GenerationCoverage{FirstIndex: -1}
	if jobDir == "" {
		return cov
	}
	if _, ok := s.vodCompleteDuration(jobDir); ok {
		cov.FirstIndex = s.firstProducedSegmentIndex(jobDir)
		cov.Complete = true
	}
	return cov
}

// firstProducedSegmentIndex returns the lowest segment index on disk, or -1.
func (s *Store) firstProducedSegmentIndex(jobDir string) int {
	entries, err := os.ReadDir(filepath.Join(jobDir, "seg"))
	if err != nil {
		return -1
	}
	first := -1
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		idx, err := parseSegmentIndex(entry.Name())
		if err != nil {
			continue
		}
		if first < 0 || idx < first {
			first = idx
		}
	}
	return first
}

// IsCompleteGeneration reports whether a generation directory holds a finished
// VOD output that can be served without running ffmpeg again.
func (s *Store) IsCompleteGeneration(jobDir string) bool {
	if jobDir == "" || !s.InitReady(jobDir) {
		return false
	}
	_, ok := s.vodCompleteDuration(jobDir)
	return ok
}

// FindReusableGeneration returns the complete generation under fingerprint
// with the widest coverage (lowest listed segment index, newest wins ties).
// Generations are keyed by file identity + transcode params, so any user
// playing the same file/profile shares the cached output. The caller must
// check the returned coverage reaches the requested playhead.
func (s *Store) FindReusableGeneration(fingerprint string) (string, GenerationCoverage, bool) {
	if fingerprint == "" {
		return "", GenerationCoverage{}, false
	}
	fpDir := filepath.Join(s.root, fingerprint)
	entries, err := os.ReadDir(fpDir)
	if err != nil {
		return "", GenerationCoverage{}, false
	}
	var best string
	var bestCov GenerationCoverage
	var bestMod time.Time
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		genDir := filepath.Join(fpDir, entry.Name())
		if !s.IsCompleteGeneration(genDir) {
			continue
		}
		cov := s.GenerationCoverage(genDir)
		info, statErr := entry.Info()
		if statErr != nil {
			continue
		}
		if best == "" || cov.FirstIndex < bestCov.FirstIndex || (cov.FirstIndex == bestCov.FirstIndex && info.ModTime().After(bestMod)) {
			best = genDir
			bestCov = cov
			bestMod = info.ModTime()
		}
	}
	return best, bestCov, best != ""
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
