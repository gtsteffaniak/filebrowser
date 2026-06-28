package http

import (
	"fmt"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/ffmpeg"
	"github.com/gtsteffaniak/go-logger/logger"
)

func hlsFormatMs(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	return fmt.Sprintf("%.0fms", float64(d)/float64(time.Millisecond))
}

func hlsLogSession(entry *transcodeSessionEntry) string {
	if entry == nil {
		return "session=?"
	}
	return fmt.Sprintf("session=%s file=%q", entry.ID, entry.FileName)
}

func hlsLogParams(params ffmpeg.HLSSegmentParams, profileMode string) string {
	mode := "transcode"
	switch {
	case params.Remux:
		mode = "remux"
	case params.VideoCopy:
		mode = "video-copy"
	}
	return fmt.Sprintf("profile=%s mode=%s gop=%d maxH=%d", profileMode, mode, params.GOP, params.MaxHeight)
}

func hlsLogInfo(entry *transcodeSessionEntry, msg string, args ...interface{}) {
	prefix := hlsLogSession(entry)
	logger.Infof("hls "+prefix+": "+msg, args...)
}

func hlsLogError(entry *transcodeSessionEntry, msg string, args ...interface{}) {
	prefix := hlsLogSession(entry)
	logger.Errorf("hls "+prefix+": "+msg, args...)
}
