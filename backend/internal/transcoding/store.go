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

// PlaylistPath is the publisher's canonical serve playlist: every segment it
// lists has already been validated, aligned, and renamed into seg/.
func (s *Store) PlaylistPath(jobDir string) string {
	return filepath.Join(jobDir, "index.m3u8")
}

func (s *Store) SegmentPath(jobDir, name string) (string, error) {
	path, err := ffmpeg.ResolveHLSSegmentPath(jobDir, name)
	if err != nil {
		return "", ErrInvalidSegment
	}
	return path, nil
}

func (s *Store) InitReady(jobDir string) bool {
	return ffmpeg.HLSInitReady(jobDir)
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
	return data, strings.Contains(string(data), "seg/"), nil
}

// GenerationCoverage describes the contiguous produced range of a generation
// on the shared segment grid. Segments before FirstIndex were never produced;
// LastReadyIndex is the produced edge a session can serve right now.
type GenerationCoverage struct {
	FirstIndex     int     // first listed segment index (the job's start index); -1 when no playlist segments exist
	LastReadyIndex int     // highest published segment index; -1 when nothing is ready
	StartSec       float64 // effective media start — the publisher's snapped/real anchor
	ProducedSec    float64 // servable media duration (sum of published #EXTINF)
	EndList        bool    // ffmpeg wrote #EXT-X-ENDLIST — the job ran to EOF
	Complete       bool    // EndList && every listed segment published && at least one segment
	HasPlaylist    bool    // the publisher's job.json existed
}

// GenerationCoverage reads the generation's job.json — the publisher's
// authoritative record of produced coverage. A running job rewrites it as
// segments publish, so the coverage reflects the produced edge at read time.
func (s *Store) GenerationCoverage(jobDir string) GenerationCoverage {
	cov := GenerationCoverage{FirstIndex: -1, LastReadyIndex: -1}
	if jobDir == "" {
		return cov
	}
	st, err := ffmpeg.ReadHLSJobStatus(jobDir)
	if err != nil {
		return cov
	}
	cov.HasPlaylist = true
	cov.FirstIndex = st.FirstIndex
	cov.LastReadyIndex = st.LastReadyIndex
	cov.StartSec = st.StartSec
	cov.ProducedSec = st.ProducedSec
	cov.EndList = st.EndList
	cov.Complete = st.Complete
	return cov
}

// IsCompleteGeneration reports whether a generation directory holds a finished,
// fully aligned HLS output that can be served without running ffmpeg again.
func (s *Store) IsCompleteGeneration(jobDir string) bool {
	return jobDir != "" && s.InitReady(jobDir) && s.GenerationCoverage(jobDir).Complete
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
	for _, sub := range []string{"seg", "raw"} {
		segDir := filepath.Join(jobDir, sub)
		entries, err := os.ReadDir(segDir)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return err
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			if err := os.Remove(filepath.Join(segDir, entry.Name())); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	for _, name := range []string{"ffmpeg.m3u8", "ffmpeg.m3u8.tmp", "index.m3u8", "job.json", "init.m4s"} {
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
