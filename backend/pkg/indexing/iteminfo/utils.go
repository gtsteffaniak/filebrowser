package iteminfo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/internal/gomedia"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
)

type FFProbeOutput struct {
	Streams []struct {
		Index       int               `json:"index"`
		CodecType   string            `json:"codec_type"`
		CodecName   string            `json:"codec_name"`
		Tags        map[string]string `json:"tags,omitempty"`
		Disposition map[string]int    `json:"disposition,omitempty"`
	} `json:"streams"`
}

// GetSubtitles detects external subtitle files for video files.
// Embedded subtitles should be detected by ffmpeg and passed as embeddedSubs parameter.
func (i *ExtendedFileInfo) GetSubtitles(parentInfo *FileInfo) {
	if !strings.HasPrefix(i.Type, "video") {
		return
	}

	// Sidecar subtitles in formats go-media lists come from go-media. The other
	// SubtitleExts (.sbv, .sub, .smi) keep the directory scan.
	var names []string
	if parentInfo != nil && parentInfo.Files != nil {
		for _, f := range parentInfo.Files {
			names = append(names, f.Name)
		}
	}
	scanGoMedia := func(lowerExt string) bool { return gomedia.SidecarSubtitleExts[lowerExt] }
	legacy := func() ([]utils.SubtitleTrack, bool) {
		return scanSidecarSubtitles(i.Name, names, scanGoMedia), true
	}
	externalSubs, ok := gomedia.ExternalSubtitles(i.Name, names, legacy)
	if !ok {
		externalSubs, _ = legacy()
	}
	externalSubs = append(externalSubs, scanSidecarSubtitles(i.Name, names, func(lowerExt string) bool {
		return !gomedia.SidecarSubtitleExts[lowerExt]
	})...)

	// Sort external subtitles alphabetically for consistent ordering
	sort.Slice(externalSubs, func(i, j int) bool {
		return externalSubs[i].Name < externalSubs[j].Name
	})

	i.Subtitles = append(i.Subtitles, externalSubs...)

	// Sort all subtitles for consistent ordering
	sort.Slice(i.Subtitles, func(a, b int) bool {
		return strings.ToLower(i.Subtitles[a].Name) < strings.ToLower(i.Subtitles[b].Name)
	})
	// Note: Content is NOT loaded here. Frontend will call /api/media/subtitles for each track
}

func (info *FileInfo) SortItems() {
	sort.Slice(info.Folders, func(i, j int) bool {
		nameWithoutExt := strings.Split(info.Folders[i].Name, ".")[0]
		nameWithoutExt2 := strings.Split(info.Folders[j].Name, ".")[0]
		// Convert strings to integers for numeric sorting if both are numeric
		numI, errI := strconv.Atoi(nameWithoutExt)
		numJ, errJ := strconv.Atoi(nameWithoutExt2)
		if errI == nil && errJ == nil {
			return numI < numJ
		}
		// Fallback to case-insensitive lexicographical sorting
		return strings.ToLower(info.Folders[i].Name) < strings.ToLower(info.Folders[j].Name)
	})
	sort.Slice(info.Files, func(i, j int) bool {
		nameWithoutExt := strings.Split(info.Files[i].Name, ".")[0]
		nameWithoutExt2 := strings.Split(info.Files[j].Name, ".")[0]
		// Convert strings to integers for numeric sorting if both are numeric
		numI, errI := strconv.Atoi(nameWithoutExt)
		numJ, errJ := strconv.Atoi(nameWithoutExt2)
		if errI == nil && errJ == nil {
			return numI < numJ
		}
		// Fallback to case-insensitive lexicographical sorting
		return strings.ToLower(info.Files[i].Name) < strings.ToLower(info.Files[j].Name)
	})
}

// PathWithinRoot reports whether path is equal to or under root after resolving symlinks.
func PathWithinRoot(root, path string) error {
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return fmt.Errorf("invalid root path: %w", err)
	}
	rootReal, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return fmt.Errorf("could not resolve root path: %w", err)
	}
	pathAbs, err := filepath.Abs(path)
	if err != nil {
		return fmt.Errorf("invalid path: %w", err)
	}
	pathReal, err := filepath.EvalSymlinks(pathAbs)
	if err != nil {
		return fmt.Errorf("could not resolve path: %w", err)
	}
	rel, err := filepath.Rel(rootReal, pathReal)
	if err != nil {
		return fmt.Errorf("path outside root: %w", err)
	}
	sep := string(filepath.Separator)
	if rel == ".." || strings.HasPrefix(rel, ".."+sep) {
		return fmt.Errorf("path escapes root")
	}
	return nil
}

// ResolveSymlinks resolves symlinks in the given path and returns
// Uses Go's filepath.EvalSymlinks which properly detects circular symlinks.
func ResolveSymlinks(path string) (string, bool, error) {
	resolvedPath, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path, false, fmt.Errorf("could not resolve symlinks for %s: %w", path, err)
	}
	info, err := os.Lstat(resolvedPath)
	if err != nil {
		return resolvedPath, false, fmt.Errorf("could not stat resolved path %s: %w", resolvedPath, err)
	}
	return resolvedPath, IsDirectory(info), nil
}

// scanSidecarSubtitles returns the sidecar subtitle files in names that belong to
// videoName (same base name, optionally with a language suffix such as .en) and
// whose extension is accepted by want.
func scanSidecarSubtitles(videoName string, names []string, want func(lowerExt string) bool) []utils.SubtitleTrack {
	ext := filepath.Ext(videoName)
	baseWithoutExt := strings.TrimSuffix(videoName, ext)
	var out []utils.SubtitleTrack
	for _, name := range names {
		fileExt := filepath.Ext(name)
		lowerExt := strings.ToLower(fileExt)
		if !SubtitleExts[lowerExt] || !want(lowerExt) {
			continue
		}
		fileBaseWithoutExt := strings.TrimSuffix(name, fileExt)
		langExt := filepath.Ext(fileBaseWithoutExt)
		fileBaseWithoutExt2 := strings.TrimSuffix(fileBaseWithoutExt, langExt)
		if fileBaseWithoutExt != baseWithoutExt && fileBaseWithoutExt2 != baseWithoutExt {
			continue
		}
		track := utils.SubtitleTrack{Name: name, Codec: strings.TrimPrefix(lowerExt, "."), Embedded: false}
		// Language from names like "video.en.srt"
		parts := strings.Split(fileBaseWithoutExt, ".")
		if len(parts) > 1 {
			if last := parts[len(parts)-1]; len(last) == 2 || len(last) == 3 {
				track.Language = last
			}
		}
		out = append(out, track)
	}
	return out
}
