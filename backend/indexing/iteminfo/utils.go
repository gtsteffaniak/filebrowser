package iteminfo

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/gtsteffaniak/go-logger/logger"
)

// detects subtitles for video files.
func (i *ExtendedFileInfo) DetectSubtitles(parentInfo *FileInfo) {
	if !strings.HasPrefix(i.Type, "video") {
		logger.Debug("subtitles are not supported for this file : " + i.Name)
		return
	}
	ext := filepath.Ext(i.Name)
	baseWithoutExt := strings.TrimSuffix(i.Name, ext)
	for _, f := range parentInfo.Files {
		baseName := strings.TrimSuffix(i.Name, ext)
		if baseName != baseWithoutExt {
			continue
		}

		for _, subtitleExt := range SubtitleExts {
			if strings.HasSuffix(f.Name, subtitleExt) {
				i.Subtitles = append(i.Subtitles, parentInfo.Path+f.Name)
			}
		}
	}
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
