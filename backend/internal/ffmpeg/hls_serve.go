package ffmpeg

import "github.com/gtsteffaniak/go-ffmpeg/ops"

// NormalizeHLSPlaylist applies canonical continuous-HLS playlist fixes before serving.
func NormalizeHLSPlaylist(outDir string) error {
	return ops.NormalizeContinuousPlaylist(outDir)
}

// HLSInitReady reports whether init.m4s exists and is non-empty.
func HLSInitReady(outDir string) bool {
	return ops.ContinuousInitReady(outDir)
}

// HLSSegmentServeReady reports whether a segment is aligned and safe to serve.
func HLSSegmentServeReady(outDir, name string) bool {
	return ops.ContinuousSegmentServeReady(outDir, name)
}

// ResolveHLSSegmentPath resolves a segment name to a path under outDir.
func ResolveHLSSegmentPath(outDir, name string) (string, error) {
	return ops.ResolveContinuousSegmentPath(outDir, name)
}
