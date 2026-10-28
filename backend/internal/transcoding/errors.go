package transcoding

import "errors"

var (
	ErrDisabled         = errors.New("transcoding disabled")
	ErrUnavailable      = errors.New("ffmpeg unavailable")
	ErrNotFound         = errors.New("session not found")
	ErrNotOwner         = errors.New("session not owned by user")
	ErrUserLimit        = errors.New("user transcode limit reached")
	ErrGlobalLimit      = errors.New("global transcode limit reached")
	ErrInvalidProfile   = errors.New("unsupported transcode profile")
	ErrUnsupportedMedia = errors.New("unsupported media for transcoding")
	ErrInvalidSegment   = errors.New("invalid segment name")
	ErrNotReady         = errors.New("segment not ready")
	// ErrNeedsRestart is returned when the segment is before the generation's
	// produced range and no live producer can reposition — the client should
	// start a replacement session at the requested playhead.
	ErrNeedsRestart = errors.New("segment needs session restart")
	ErrInvalidPath      = errors.New("invalid cache path")
)
