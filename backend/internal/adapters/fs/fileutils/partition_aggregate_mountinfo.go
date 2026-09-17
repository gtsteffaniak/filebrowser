package fileutils

import (
	"bufio"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type mountEntry struct {
	deviceID   string
	mountpoint string
}

// distinctMountPathsFromMountinfo selects one path per distinct filesystem that
// contributes capacity for root: the FS that owns root, plus nested mounts under
// root. Deduped by major:minor from mountinfo.
func distinctMountPathsFromMountinfo(mountinfo string, root string) ([]string, error) {
	root = filepath.Clean(root)
	entries, err := parseAllMounts(mountinfo)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]string)

	var covering *mountEntry
	for i := range entries {
		e := &entries[i]
		mp := e.mountpoint
		if mp == root || strings.HasPrefix(root, mp+string(os.PathSeparator)) || mp == string(os.PathSeparator) {
			if covering == nil || len(mp) > len(covering.mountpoint) {
				covering = e
			}
		}
	}
	if covering != nil {
		seen[covering.deviceID] = root
	}

	for _, e := range entries {
		mp := e.mountpoint
		if mp != root && !strings.HasPrefix(mp, root+string(os.PathSeparator)) {
			continue
		}
		if _, ok := seen[e.deviceID]; ok {
			continue
		}
		seen[e.deviceID] = mp
	}

	out := make([]string, 0, len(seen))
	for _, p := range seen {
		out = append(out, p)
	}
	return out, nil
}

func parseAllMounts(mountinfo string) ([]mountEntry, error) {
	var out []mountEntry
	scanner := bufio.NewScanner(strings.NewReader(mountinfo))
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)
	for scanner.Scan() {
		entry, ok := parseMountinfoLine(scanner.Text())
		if !ok {
			continue
		}
		entry.mountpoint = filepath.Clean(entry.mountpoint)
		out = append(out, entry)
	}
	return out, scanner.Err()
}

func parseMountinfoLine(line string) (mountEntry, bool) {
	parts := strings.SplitN(line, " - ", 2)
	if len(parts) != 2 {
		return mountEntry{}, false
	}
	fields := strings.Fields(parts[0])
	if len(fields) < 5 {
		return mountEntry{}, false
	}
	return mountEntry{
		deviceID:   fields[2],
		mountpoint: unescapeFstab(fields[4]),
	}, true
}

func unescapeFstab(path string) string {
	var b strings.Builder
	b.Grow(len(path))
	for i := 0; i < len(path); i++ {
		if path[i] == '\\' && i+3 < len(path) {
			if n, err := strconv.ParseUint(path[i+1:i+4], 8, 8); err == nil {
				b.WriteByte(byte(n))
				i += 3
				continue
			}
		}
		b.WriteByte(path[i])
	}
	return b.String()
}
