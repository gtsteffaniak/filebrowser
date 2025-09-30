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
func (i *ExtendedFileInfo) DetectSubtitles(parentInfo *FileInfo) {
	logger.Debugf("[DETECT_SUBTITLES] Called for: %s (type: %s)", i.Name, i.Type)
	if !strings.HasPrefix(i.Type, "video") {
		logger.Debugf("[DETECT_SUBTITLES] Not a video file: %s", i.Name)
		return
	}
	// Use unified subtitle detection that finds both embedded and external
	parentDir := filepath.Dir(i.RealPath)
	logger.Debugf("[DETECT_SUBTITLES] Scanning for subtitles: file=%s, realPath=%s, parentDir=%s", i.Name, i.RealPath, parentDir)
	i.Subtitles = ffmpeg.DetectAllSubtitles(i.RealPath, parentDir, i.ModTime)
	logger.Debugf("[DETECT_SUBTITLES] Found %d subtitle tracks for: %s", len(i.Subtitles), i.Name)
	for idx, sub := range i.Subtitles {
		logger.Debugf("[DETECT_SUBTITLES]   Track %d: name=%s, lang=%s, isFile=%v, index=%v", idx, sub.Name, sub.Language, sub.IsFile, sub.Index)
	}
}

// LoadSubtitleContent loads the actual content for all detected subtitle tracks
func (i *ExtendedFileInfo) LoadSubtitleContent() error {
	logger.Debugf("[LOAD_SUBTITLES] Loading content for %d subtitle tracks: %s", len(i.Subtitles), i.Name)
	err := ffmpeg.LoadAllSubtitleContent(i.RealPath, i.Subtitles, i.ModTime)
	if err != nil {
		logger.Errorf("[LOAD_SUBTITLES] Error loading subtitles: %v", err)
	} else {
		logger.Debugf("[LOAD_SUBTITLES] Successfully loaded subtitle content for: %s", i.Name)
	}
	return err
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
