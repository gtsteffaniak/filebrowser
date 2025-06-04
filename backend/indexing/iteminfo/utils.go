package iteminfo

import (
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

	base := strings.Split(i.Name, ".")[0]
	for _, f := range parentInfo.Files {
		baseName := strings.Split(f.Name, ".")[0]
		if baseName != base {
			continue
		}

		for _, subtitleExt := range []string{".vtt", ".srt", ".lrc", ".sbv", ".ass", ".ssa", ".sub", ".smi"} {
			if strings.HasSuffix(f.Name, subtitleExt) {
				fullPathBase := strings.Split(i.Path, ".")[0]
				i.Subtitles = append(i.Subtitles, fullPathBase+subtitleExt)
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
