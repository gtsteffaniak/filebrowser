package iteminfo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
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
	ext := filepath.Ext(i.Name)
	baseWithoutExt := strings.TrimSuffix(i.Name, ext)

	// Collect external subtitle files
	var externalSubs []utils.SubtitleTrack
	if parentInfo != nil && parentInfo.Files != nil {
		for _, f := range parentInfo.Files {
			fileExt := filepath.Ext(f.Name)
			fileBase := strings.TrimSuffix(f.Name, fileExt)

			// Check if this file has the same base name and a subtitle extension
			if fileBase == baseWithoutExt && SubtitleExts[strings.ToLower(fileExt)] {
				track := utils.SubtitleTrack{
					Name:   f.Name,
					IsFile: true,
				}

				// Try to infer language from filename patterns like "video.en.srt"
				parts := strings.Split(fileBase, ".")
				if len(parts) > 1 {
					lastPart := parts[len(parts)-1]
					if len(lastPart) == 2 || len(lastPart) == 3 {
						track.Language = lastPart
					}
				}

				externalSubs = append(externalSubs, track)
			}
		}
	}

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

// ResolveSymlinks resolves symlinks in the given path and returns
// the final resolved path, whether it's a directory (considering bundle logic), and any error.
func ResolveSymlinks(path string) (string, bool, error) {
	for {
		// Get the file info using os.Lstat to handle symlinks
		info, err := os.Lstat(path)
		if err != nil {
			return path, false, fmt.Errorf("could not stat path: %s, %v", path, err)
		}

		// Check if the path is a symlink
		if info.Mode()&os.ModeSymlink != 0 {
			// Read the symlink target
			target, err := os.Readlink(path)
			if err != nil {
				return path, false, fmt.Errorf("could not read symlink: %s, %v", path, err)
			}

			// Resolve the symlink's target relative to its directory
			path = filepath.Join(filepath.Dir(path), target)
		} else {
			// Not a symlink, check with bundle-aware directory logic
			isDir := IsDirectory(info)
			return path, isDir, nil
		}
	}
}
