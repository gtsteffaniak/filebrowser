package iteminfo

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
	"github.com/gtsteffaniak/go-logger/logger"
)

type SubtitleTrack struct {
	Name     string `json:"name"`               // filename for external, or descriptive name for embedded
	Language string `json:"language,omitempty"` // language code
	Title    string `json:"title,omitempty"`    // title/description
	Index    *int   `json:"index,omitempty"`    // stream index for embedded subtitles (nil for external)
	Codec    string `json:"codec,omitempty"`    // codec name for embedded subtitles
	IsFile   bool   `json:"isFile"`             // true for external files, false for embedded
}

type FFProbeOutput struct {
	Streams []struct {
		Index       int               `json:"index"`
		CodecType   string            `json:"codec_type"`
		CodecName   string            `json:"codec_name"`
		Tags        map[string]string `json:"tags,omitempty"`
		Disposition map[string]int    `json:"disposition,omitempty"`
	} `json:"streams"`
}

// detects subtitles for video files.
func (i *ExtendedFileInfo) GetSubtitles(parentInfo *FileInfo, extractEmbeddedSubtitles bool) {
	if !strings.HasPrefix(i.Type, "video") {
		return
	}
	// First, detect embedded subtitles using ffmpeg (if enabled)
	if extractEmbeddedSubtitles {
		embeddedSubs := ffmpeg.DetectEmbeddedSubtitles(i.RealPath, i.ModTime)
		i.Subtitles = append(i.Subtitles, embeddedSubs...)
	}

	ext := filepath.Ext(i.Name)
	baseWithoutExt := strings.TrimSuffix(i.Name, ext)
	if parentInfo != nil && parentInfo.Files != nil {
		for _, f := range parentInfo.Files {
			fileExt := filepath.Ext(f.Name)
			fileBase := strings.TrimSuffix(f.Name, fileExt)

			// Check if this file has the same base name and a subtitle extension
			if fileBase == baseWithoutExt && SubtitleExts[strings.ToLower(fileExt)] {
				track := ffmpeg.SubtitleTrack{
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

				i.Subtitles = append(i.Subtitles, track)
			}
		}
	}

	// Load content for ALL detected subtitles (both embedded and external)
	if len(i.Subtitles) > 0 {
		err := i.LoadSubtitleContent()
		if err != nil {
			logger.Debug("failed to load subtitle content: " + err.Error())
		}
	}
}

// LoadSubtitleContent loads the actual content for all detected subtitle tracks
func (i *ExtendedFileInfo) LoadSubtitleContent() error {
	return ffmpeg.LoadAllSubtitleContent(i.RealPath, i.Subtitles, i.ModTime)
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
