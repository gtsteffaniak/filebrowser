package transcoding

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

// Store manages disk-backed HLS cache directories.
type Store struct {
	root string
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

var vodSegmentFilePattern = regexp.MustCompile(`^[0-9]{5}\.m4s$`)

func (s *Store) SegmentPath(jobDir, name string) (string, error) {
	name = filepath.Base(strings.TrimSpace(name))
	if !vodSegmentFilePattern.MatchString(name) {
		return "", ErrInvalidSegment
	}
	path := filepath.Join(jobDir, "seg", name)
	if err := s.assertContained(jobDir, path); err != nil {
		return "", ErrInvalidSegment
	}
	return path, nil
}

func (s *Store) InitReady(jobDir string) bool {
	st, err := os.Stat(s.InitPath(jobDir))
	return err == nil && st.Size() > 0
}

// OpenSegment opens a published segment. The publisher only lists a segment in
// index.m3u8 after it lands in seg/, so a listed segment always exists —
// a missing file here means the generation was cleared or evicted.
func (s *Store) OpenSegment(jobDir, name string) (*os.File, error) {
	path, err := s.SegmentPath(jobDir, name)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, ErrNotReady
		}
		return nil, err
	}
	return f, nil
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

// ReadServePlaylist returns the publisher's index.m3u8 verbatim plus whether
// it lists at least one segment. The publisher atomically rewrites it as
// segments are validated, so serving never races with ffmpeg and needs no
// per-request readiness filtering.
func (s *Store) ReadServePlaylist(jobDir string) ([]byte, bool, error) {
	path := s.PlaylistPath(jobDir)
	if err := s.assertContained(jobDir, path); err != nil {
		return nil, false, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		// The publisher has not written the playlist yet; callers poll on
		// ErrNotReady instead of treating a missing file as a hard 500.
		if os.IsNotExist(err) {
			return nil, false, ErrNotReady
		}
		return nil, false, err
	}
	body := string(data)
	if strings.Contains(body, "#EXT-X-PLAYLIST-TYPE:VOD") {
		return data, true, nil
	}
	return data, strings.Contains(body, "seg/"), nil
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
	return os.WriteFile(path, []byte(fmt.Sprintf("%.6f\n", durationSec)), 0o644)
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
	FirstIndex  int     // first produced segment index; -1 when nothing was produced
	StartSec    float64 // unused — the VOD grid makes coverage playlist-local
	ProducedSec float64 // servable media duration from the vod.complete marker
	Complete    bool    // the generation ran to EOF (vod.complete marker exists)
	HasPlaylist bool    // the serve playlist exists
}

// GenerationCoverage reads the vod.complete marker plus the produced segment
// files — the only authoritative record under the synthetic playlist model.
func (s *Store) GenerationCoverage(jobDir string) GenerationCoverage {
	cov := GenerationCoverage{FirstIndex: -1}
	if jobDir == "" {
		return cov
	}
	if _, err := os.Stat(s.PlaylistPath(jobDir)); err == nil {
		cov.HasPlaylist = true
	}
	if dur, ok := s.vodCompleteDuration(jobDir); ok {
		cov.FirstIndex = s.firstProducedSegmentIndex(jobDir)
		cov.ProducedSec = dur
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

// ClearGenerationOutput wipes a generation's produced files — segments,
// readiness markers, playlists, init — while keeping the job directory itself.
// Used when a running producer must restart: coverage is computed from files
// on disk, so stale output from the failed run would otherwise be served or
// inflate the produced edge.
func (s *Store) ClearGenerationOutput(jobDir string) error {
	if jobDir == "" {
		return nil
	}
	if err := s.assertContained(s.root, jobDir); err != nil {
		return err
	}
	segDir := filepath.Join(jobDir, "seg")
	entries, err := os.ReadDir(segDir)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
	} else {
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if err := os.Remove(filepath.Join(segDir, entry.Name())); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	for _, name := range []string{"init.m4s", vodCompleteMarker, ".ffmpeg-vod-segments.csv"} {
		if err := os.Remove(filepath.Join(jobDir, name)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}
	return nil
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
