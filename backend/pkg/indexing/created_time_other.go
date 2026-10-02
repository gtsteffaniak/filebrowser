//go:build !darwin && !linux && !windows

package indexing

import (
	"os"
	"time"
)

// getCreatedTime always returns nil: birth time is not supported on this platform.
func getCreatedTime(_ os.FileInfo, _ string) *time.Time {
	return nil
}
