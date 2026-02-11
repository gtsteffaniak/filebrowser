package utils

import (
	"errors"
	"os"
)

type SubtitleTrack struct {
	Name     string `json:"name"`            // filename for external, or descriptive name for embedded
	Language string `json:"language"`        // language code
	Title    string `json:"title,omitempty"` // title/description
	Index    *int   `json:"index,omitempty"` // stream index for embedded subtitles (nil for external)
	Codec    string `json:"codec,omitempty"` // codec name for embedded subtitles
	Embedded bool   `json:"embedded"`        // true for embedded subtitles, false for external files
}

func GetSubtitleSidecarContent(subtitlePath string) (string, error) {
	// validate the size is not too large
	info, err := os.Stat(subtitlePath)
	if err != nil {
		return "", err
	}
	if info.Size() > 1024*1024*50 { // 50MB
		return "", errors.New("subtitle file is too large")
	}

	// First check if it's a text file using the consolidated validation logic
	isText, err := IsTextFile(subtitlePath)
	if err != nil {
		return "", err
	}
	if !isText {
		// Not a text file, return empty string (no error, just not text)
		return "", nil
	}

	// read the file content
	content, err := os.ReadFile(subtitlePath)
	if err != nil {
		return "", err
	}
	return string(content), nil
}
