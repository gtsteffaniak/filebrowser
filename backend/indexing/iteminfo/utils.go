package iteminfo

import (
	"encoding/json"
	"os/exec"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
)

type SubtitleTrack struct {
	Name     string `json:"name"`                // filename for external, or descriptive name for embedded
	Language string `json:"language,omitempty"`  // language code
	Title    string `json:"title,omitempty"`     // title/description
	Index    *int   `json:"index,omitempty"`     // stream index for embedded subtitles (nil for external)
	Codec    string `json:"codec,omitempty"`     // codec name for embedded subtitles
	IsFile   bool   `json:"isFile"`              // true for external files, false for embedded
}

type FFProbeOutput struct {
	Streams []struct {
		Index         int               `json:"index"`
		CodecType     string           `json:"codec_type"`
		CodecName     string           `json:"codec_name"`
		Tags          map[string]string `json:"tags,omitempty"`
		Disposition   map[string]int    `json:"disposition,omitempty"`
	} `json:"streams"`
}

// detects subtitles for video files.
func (i *ExtendedFileInfo) DetectSubtitles(parentInfo *FileInfo) {
	if !strings.HasPrefix(i.Type, "video") {
		logger.Debug("subtitles are not supported for this file : " + i.Name)
		return
	}
	
	// Detect external subtitle files
	ext := filepath.Ext(i.Name)
	baseWithoutExt := strings.TrimSuffix(i.Name, ext)
	for _, f := range parentInfo.Files {
		baseName := strings.TrimSuffix(f.Name, filepath.Ext(f.Name))
		if baseName != baseWithoutExt {
			continue
		}

		for _, subtitleExt := range SubtitleExts {
			if strings.HasSuffix(f.Name, subtitleExt) {
				i.Subtitles = append(i.Subtitles, SubtitleTrack{
					Name:   f.Name,
					IsFile: true,
				})
			}
		}
	}
	
	embeddedSubs := i.detectEmbeddedSubtitles()
	i.Subtitles = append(i.Subtitles, embeddedSubs...)
}

// detectEmbeddedSubtitles uses ffprobe to find embedded subtitle tracks
func (i *ExtendedFileInfo) detectEmbeddedSubtitles() []SubtitleTrack {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "s",
		i.RealPath)

	output, err := cmd.Output()
	if err != nil {
		logger.Debug("ffprobe failed for file: " + i.Name + ", error: " + err.Error())
		return nil
	}

	var probeOutput FFProbeOutput
	if err := json.Unmarshal(output, &probeOutput); err != nil {
		logger.Debug("failed to parse ffprobe output for file: " + i.Name)
		return nil
	}

	var subtitles []SubtitleTrack
	for _, stream := range probeOutput.Streams {
		if stream.CodecType == "subtitle" {
			track := SubtitleTrack{
				Index:  &stream.Index,
				Codec:  stream.CodecName,
				IsFile: false,
			}

			// Extract language and title from tags
			if stream.Tags != nil {
				if lang, ok := stream.Tags["language"]; ok {
					track.Language = lang
				}
				if title, ok := stream.Tags["title"]; ok {
					track.Title = title
				}
			}

			// Generate a descriptive name
			if track.Title != "" {
				track.Name = track.Title
			} else if track.Language != "" {
				track.Name = "Embedded (" + track.Language + ")"
			} else {
				track.Name = "Embedded Subtitle " + strconv.Itoa(stream.Index)
			}

			subtitles = append(subtitles, track)
		}
	}

	return subtitles
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
