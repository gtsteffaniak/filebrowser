//go:build !darwin && !linux && !windows

package indexing

import (
	"os"
	"time"
)

func getCreatedTime(_ os.FileInfo, _ string) *time.Time {
	return nil
}
