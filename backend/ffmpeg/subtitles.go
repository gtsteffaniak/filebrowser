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

	"github.com/gtsteffaniak/go-cache/cache"
	"github.com/gtsteffaniak/go-logger/logger"
)

// SubtitleTrack represents a subtitle track (embedded or external file)
type SubtitleTrack struct {
	Name     string `json:"name"`               // filename for external, or descriptive name for embedded
	Language string `json:"language,omitempty"` // language code
	Title    string `json:"title,omitempty"`    // title/description
	Index    *int   `json:"index,omitempty"`    // stream index for embedded subtitles (nil for external)
	Codec    string `json:"codec,omitempty"`    // codec name for embedded subtitles
	Content  string `json:"content,omitempty"`  // subtitle content
	IsFile   bool   `json:"isFile"`             // true for external files, false for embedded streams
}

var (
	MediaCache           = cache.NewCache[[]SubtitleTrack](24 * time.Hour) // subtitles get cached for 24 hours
	SubtitleContentCache = cache.NewCache[string](24 * time.Hour)          // subtitle content gets cached for 24 hours
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
func DetectAllSubtitles(videoPath string, parentDir string, modtime time.Time) []SubtitleTrack {
	key := "all_subtitles:" + videoPath + ":" + modtime.Format(time.RFC3339)

	// Check cache first
	if cached, ok := MediaCache.Get(key); ok {
		return cached
	}

	var allSubtitles []SubtitleTrack

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

// hasSubtitleStreams performs a fast header check to detect if subtitle streams likely exist
// This avoids expensive ffprobe calls on files without subtitles
func hasSubtitleStreams(realPath string) bool {
	file, err := os.Open(realPath)
	if err != nil {
		return false // If we can't open, assume no subtitles
	}
	defer file.Close()

	// Read first 64KB of file - enough to check container headers
	buffer := make([]byte, 65536)
	n, err := file.Read(buffer)
	if err != nil || n < 100 {
		return false
	}

	// Check file extension for quick filtering
	ext := strings.ToLower(filepath.Ext(realPath))

	// Check container-specific subtitle markers
	switch ext {
	case ".mkv", ".webm":
		// MKV/WebM: Look for subtitle track type markers
		// "S_TEXT" = SRT subtitles, "S_VOBSUB" = VobSub, etc.
		return strings.Contains(string(buffer[:n]), "S_TEXT") ||
			strings.Contains(string(buffer[:n]), "S_VOBSUB") ||
			strings.Contains(string(buffer[:n]), "S_HDMV") ||
			strings.Contains(string(buffer[:n]), "S_SSA") ||
			strings.Contains(string(buffer[:n]), "S_ASS")
	case ".mp4", ".m4v", ".mov":
		// MP4: Look for subtitle track atoms (tx3g, c608, etc.)
		return strings.Contains(string(buffer[:n]), "tx3g") || // Timed text
			strings.Contains(string(buffer[:n]), "c608") || // CEA-608 captions
			strings.Contains(string(buffer[:n]), "c708") || // CEA-708 captions
			strings.Contains(string(buffer[:n]), "sbtl") // Subtitle track
	case ".avi":
		// AVI: Check for subtitle stream headers
		return strings.Contains(string(buffer[:n]), "txts") || // Text subtitle stream
			strings.Contains(string(buffer[:n]), "ISFT") // Subtitle chunk
	default:
		// For unknown formats, assume subtitles might exist (safe fallback)
		return true
	}
}

// detectEmbeddedSubtitles uses ffprobe to find embedded subtitle tracks
func detectEmbeddedSubtitles(realPath string) []SubtitleTrack {
	// Fast pre-check: skip ffprobe if no subtitle streams detected in header
	if !hasSubtitleStreams(realPath) {
		return nil
	}

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

	var subtitles []SubtitleTrack
	for _, stream := range probeOutput.Streams {
		if stream.CodecType == "subtitle" {
			track := SubtitleTrack{
				Index:  &stream.Index,
				Codec:  stream.CodecName,
				IsFile: false, // This is an embedded subtitle
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
func detectExternalSubtitles(videoPath string, parentDir string) []SubtitleTrack {
	var subtitles []SubtitleTrack

	// Get the base name of the video (without extension)
	videoBaseName := strings.TrimSuffix(filepath.Base(videoPath), filepath.Ext(videoPath))

	// Common subtitle extensions
	subtitleExts := []string{".srt", ".vtt", ".lrc", ".sbv", ".ass", ".ssa", ".sub", ".smi"}

	// Look for subtitle files with matching base name
	for _, ext := range subtitleExts {
		subtitlePath := filepath.Join(parentDir, videoBaseName+ext)
		if _, err := os.Stat(subtitlePath); err == nil {
			// File exists
			track := SubtitleTrack{
				Name:   filepath.Base(subtitlePath),
				IsFile: true, // This is an external file
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

// LoadAndConvertSubtitleFile loads a subtitle file and converts it to WebVTT format
func LoadAndConvertSubtitleFile(subtitlePath string) (string, error) {
	ext := strings.ToLower(filepath.Ext(subtitlePath))

	if ext == ".vtt" {
		// Already WebVTT, just read it
		content, err := os.ReadFile(subtitlePath)
		if err != nil {
			return "", err
		}
		return string(content), nil
	}

	if ext == ".srt" {
		// Convert SRT to WebVTT using ffmpeg
		cmd := exec.Command("ffmpeg",
			"-i", subtitlePath,
			"-c:s", "webvtt",
			"-f", "webvtt",
			"-") // output to stdout

		output, err := cmd.Output()
		if err != nil {
			return "", fmt.Errorf("failed to convert SRT to WebVTT: %v", err)
		}
		return string(output), nil
	}

	// For other formats, try to read as plain text and hope for the best
	// This is a fallback - ideally you'd want format-specific converters
	content, err := os.ReadFile(subtitlePath)
	if err != nil {
		return "", err
	}

	// Basic conversion wrapper for non-VTT formats
	vttHeader := "WEBVTT\n\n"
	return vttHeader + string(content), nil
}

// ExtractSingleSubtitle extracts content for a specific subtitle track by array index
func ExtractSingleSubtitle(videoPath string, parentDir string, trackIndex int, modtime time.Time) (SubtitleTrack, error) {
	// Get all subtitle tracks
	allTracks := DetectAllSubtitles(videoPath, parentDir, modtime)

	if trackIndex >= len(allTracks) {
		return SubtitleTrack{}, fmt.Errorf("subtitle track %d not found (only %d tracks available)", trackIndex, len(allTracks))
	}

	track := allTracks[trackIndex]

	// Load content based on type
	if track.IsFile {
		// Load external subtitle file
		subtitlePath := filepath.Join(parentDir, track.Name)
		content, err := LoadAndConvertSubtitleFile(subtitlePath)
		if err != nil {
			return SubtitleTrack{}, fmt.Errorf("failed to load external subtitle: %v", err)
		}
		track.Content = content
	} else {
		// Extract embedded subtitle content
		if track.Index == nil {
			return SubtitleTrack{}, fmt.Errorf("embedded subtitle track has no stream index")
		}
		content, err := ExtractSubtitleContent(videoPath, *track.Index)
		if err != nil {
			return SubtitleTrack{}, fmt.Errorf("failed to extract embedded subtitle: %v", err)
		}
		track.Content = content
	}

	return track, nil
}

// LoadAllSubtitleContent loads the actual content for all detected subtitle tracks
func LoadAllSubtitleContent(videoPath string, subtitles []SubtitleTrack, modtime time.Time) error {
	for idx := range subtitles {
		subtitle := &subtitles[idx]

		// Check if content is already cached
		contentKey := fmt.Sprintf("subtitle_content:%s:%d:%s", videoPath, idx, modtime.Format(time.RFC3339))
		if cached, ok := SubtitleContentCache.Get(contentKey); ok {
			subtitle.Content = cached
			continue
		}

		var content string
		var err error

		if subtitle.IsFile {
			// Load external subtitle file content and convert to WebVTT
			subtitlePath := filepath.Join(filepath.Dir(videoPath), subtitle.Name)
			content, err = LoadAndConvertSubtitleFile(subtitlePath)
			if err != nil {
				logger.Debug("failed to read/convert subtitle file " + subtitlePath + ": " + err.Error())
				continue
			}
		} else {
			// Load embedded subtitle content
			if subtitle.Index == nil {
				logger.Debug("embedded subtitle has no stream index")
				continue
			}
			content, err = ExtractSubtitleContent(videoPath, *subtitle.Index)
			if err != nil {
				logger.Debug("failed to extract embedded subtitle: " + err.Error())
				continue
			}
		}

		subtitle.Content = content
		// Cache the content for future requests
		SubtitleContentCache.Set(contentKey, content)
	}
	return nil
}
