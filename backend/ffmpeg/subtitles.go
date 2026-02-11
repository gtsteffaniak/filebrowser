package ffmpeg

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/go-logger/logger"
)

// FFProbeOutput represents the JSON output from ffprobe
type FFProbeOutput struct {
	Streams []struct {
		Index       int               `json:"index"`
		CodecType   string            `json:"codec_type"`
		CodecName   string            `json:"codec_name"`
		Tags        map[string]string `json:"tags,omitempty"`
		Disposition map[string]int    `json:"disposition,omitempty"`
	} `json:"streams"`
}

// DetectAllSubtitles finds both embedded and external subtitle tracks
func DetectAllSubtitles(videoPath string, parentDir string, modtime time.Time) []utils.SubtitleTrack {
	key := "all_subtitles:" + videoPath + ":" + modtime.Format(time.RFC3339)

	// Check cache first
	if cached, ok := MediaCache.Get(key); ok {
		return cached
	}

	var allSubtitles []utils.SubtitleTrack

	// First, get embedded subtitles
	embeddedSubs := detectEmbeddedSubtitles(videoPath)
	allSubtitles = append(allSubtitles, embeddedSubs...)

	// Then, get external subtitle files
	externalSubs := detectExternalSubtitles(videoPath, parentDir)
	allSubtitles = append(allSubtitles, externalSubs...)

	// Cache the complete list
	MediaCache.Set(key, allSubtitles)

	return allSubtitles
}

// DetectEmbeddedSubtitles detects embedded subtitle streams using ffprobe.
// This is the public API that can be called from other packages.
// Returns empty array if ffprobe is not available or fails.
func DetectEmbeddedSubtitles(videoPath string, modtime time.Time) []utils.SubtitleTrack {
	// Check cache first
	key := "embedded_subtitles:" + videoPath + ":" + modtime.Format(time.RFC3339)
	if cached, ok := MediaCache.Get(key); ok {
		return cached
	}
	// Detect embedded subtitles
	subtitles := detectEmbeddedSubtitles(videoPath)
	// Cache the results
	MediaCache.Set(key, subtitles)
	return subtitles
}

// detectEmbeddedSubtitles uses ffprobe to find embedded subtitle tracks
// Always runs ffprobe - results are cached for performance
func detectEmbeddedSubtitles(realPath string) []utils.SubtitleTrack {
	cmd := exec.Command("ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-select_streams", "s",
		realPath)

	output, err := cmd.Output()
	if err != nil {
		logger.Debug("ffprobe failed for file: " + realPath + ", error: " + err.Error())
		return nil
	}

	var probeOutput FFProbeOutput
	if err := json.Unmarshal(output, &probeOutput); err != nil {
		logger.Debug("failed to parse ffprobe output for file: " + realPath)
		return nil
	}

	var subtitles []utils.SubtitleTrack
	for _, stream := range probeOutput.Streams {
		if stream.CodecType == "subtitle" {
			track := utils.SubtitleTrack{
				Index:    &stream.Index,
				Codec:    stream.CodecName,
				Embedded: true, // This is an embedded subtitle
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

// detectExternalSubtitles finds external subtitle files in the same directory
func detectExternalSubtitles(videoPath string, parentDir string) []utils.SubtitleTrack {
	var subtitles []utils.SubtitleTrack

	// Get the base name of the video (without extension)
	videoBaseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))

	// Common subtitle extensions
	subtitleExts := []string{".srt", ".vtt", ".lrc", ".sbv", ".ass", ".ssa", ".sub", ".smi"}

	// Look for subtitle files with matching base name
	for _, ext := range subtitleExts {
		subtitlePath := filepath.Join(parentDir, videoBaseName+ext)
		if _, err := os.Stat(subtitlePath); err == nil {
			// File exists
			track := utils.SubtitleTrack{
				Name:     filepath.Base(subtitlePath),
				Embedded: false, // This is an external file
			}

			// Try to infer language from filename patterns like "video.en.srt"
			parts := strings.Split(videoBaseName, ".")
			if len(parts) > 1 {
				// Check if the last part before extension looks like a language code
				lastPart := parts[len(parts)-1]
				if len(lastPart) == 2 || len(lastPart) == 3 {
					track.Language = lastPart
				}
			}

			subtitles = append(subtitles, track)
		}
	}

	return subtitles
}

// ExtractSubtitleContent extracts subtitle content without service management
func ExtractSubtitleContent(videoPath string, streamIndex int) (string, error) {
	cmd := exec.Command("ffmpeg",
		"-i", videoPath,
		"-map", fmt.Sprintf("0:%d", streamIndex),
		"-c:s", "webvtt",
		"-f", "webvtt",
		"-") // output to stdout

	output, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("subtitle extraction failed: %v", err)
	}

	return string(output), nil
}

// LoadSubtitleFile loads a subtitle file and returns its raw content
func LoadSubtitleFile(subtitlePath string) (string, error) {
	content, err := os.ReadFile(subtitlePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
