package transcoding

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type cacheGeneration struct {
	fingerprint string
	generation  string
	dir         string
	modTime     time.Time
	size        int64
}

// incompleteGenerationGrace keeps a brand-new generation from being evicted
// before its producer has had a chance to write the init segment.
const incompleteGenerationGrace = time.Minute

// EvictInactive removes inactive cache generations by retention age and total size.
// Generations that never completed (cancelled, failed, or abandoned mid-transcode)
// are removed immediately: they are never reusable, so holding them for the whole
// retention window only wastes disk.
func (s *Store) EvictInactive(active map[string]struct{}, maxBytes int64, maxAge time.Duration) ([]string, error) {
	gens, err := s.listGenerations()
	if err != nil {
		return nil, err
	}
	now := time.Now()
	var inactive []cacheGeneration
	var totalSize int64
	var evicted []string
	for _, gen := range gens {
		if active != nil {
			if _, ok := active[gen.dir]; ok {
				totalSize += gen.size
				continue
			}
		}
		complete := s.IsCompleteGeneration(gen.dir)
		if !complete {
			// Too new to judge: a producer may be about to write into it.
			if now.Sub(gen.modTime) < incompleteGenerationGrace {
				totalSize += gen.size
				continue
			}
			if err := s.removeGenerationLogged(gen.dir, "incomplete"); err == nil {
				evicted = append(evicted, gen.dir)
			}
			continue
		}
		totalSize += gen.size
		if maxAge > 0 && now.Sub(gen.modTime) >= maxAge {
			if err := s.removeGenerationLogged(gen.dir, "retention"); err == nil {
				evicted = append(evicted, gen.dir)
				totalSize -= gen.size
			}
			continue
		}
		inactive = append(inactive, gen)
	}
	if maxBytes <= 0 || totalSize <= maxBytes {
		s.pruneEmptyFingerprintDirs()
		return evicted, nil
	}
	sort.Slice(inactive, func(i, j int) bool {
		return inactive[i].modTime.Before(inactive[j].modTime)
	})
	for _, gen := range inactive {
		if totalSize <= maxBytes {
			break
		}
		if err := s.removeGenerationLogged(gen.dir, "size"); err != nil {
			continue
		}
		evicted = append(evicted, gen.dir)
		totalSize -= gen.size
	}
	s.pruneEmptyFingerprintDirs()
	return evicted, nil
}

// pruneEmptyFingerprintDirs removes fingerprint directories that no longer hold
// any generation, so an emptied cache does not leave a tree of empty dirs behind.
func (s *Store) pruneEmptyFingerprintDirs() {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		fpDir := filepath.Join(s.root, entry.Name())
		children, readErr := os.ReadDir(fpDir)
		if readErr != nil {
			continue
		}
		hasGeneration := false
		for _, child := range children {
			if child.IsDir() {
				hasGeneration = true
				break
			}
		}
		if !hasGeneration {
			_ = os.RemoveAll(fpDir)
		}
	}
}

func (s *Store) removeGenerationLogged(dir, reason string) error {
	if err := s.RemoveGeneration(dir); err != nil {
		return err
	}
	return nil
}

func (s *Store) listGenerations() ([]cacheGeneration, error) {
	entries, err := os.ReadDir(s.root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var gens []cacheGeneration
	for _, fpEntry := range entries {
		if !fpEntry.IsDir() {
			continue
		}
		fpDir := filepath.Join(s.root, fpEntry.Name())
		genEntries, readErr := os.ReadDir(fpDir)
		if readErr != nil {
			continue
		}
		for _, genEntry := range genEntries {
			if !genEntry.IsDir() {
				continue
			}
			dir := filepath.Join(fpDir, genEntry.Name())
			info, statErr := genEntry.Info()
			if statErr != nil {
				continue
			}
			size, sizeErr := dirSize(dir)
			if sizeErr != nil {
				continue
			}
			gens = append(gens, cacheGeneration{
				fingerprint: fpEntry.Name(),
				generation:  genEntry.Name(),
				dir:         dir,
				modTime:     info.ModTime(),
				size:        size,
			})
		}
	}
	return gens, nil
}

func dirSize(root string) (int64, error) {
	var total int64
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		info, statErr := d.Info()
		if statErr != nil {
			return statErr
		}
		total += info.Size()
		return nil
	})
	return total, err
}
