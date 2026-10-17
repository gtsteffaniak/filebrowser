package ffmpeg

import "github.com/gtsteffaniak/go-ffmpeg/ops"

// HLSInitReady reports whether init.m4s exists and is non-empty.
func HLSInitReady(outDir string) bool {
	return ops.ContinuousInitReady(outDir)
}

// ResolveHLSSegmentPath resolves a segment name to a path under outDir.
func ResolveHLSSegmentPath(outDir, name string) (string, error) {
	return ops.ResolveContinuousSegmentPath(outDir, name)
}

// ReadHLSJobStatus loads a generation's job.json — the publisher's record of
// the effective media start and the produced coverage on disk.
func ReadHLSJobStatus(outDir string) (ops.ContinuousJobStatus, error) {
	return ops.ReadContinuousJobStatus(outDir)
}
