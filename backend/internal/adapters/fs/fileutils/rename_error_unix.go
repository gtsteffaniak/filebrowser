//go:build !windows

package fileutils

import (
	"errors"
	"os"
	"syscall"
)

func isCrossDeviceRenameError(err error) bool {
	if errors.Is(err, syscall.EXDEV) {
		return true
	}
	var linkErr *os.LinkError
	return errors.As(err, &linkErr) && errors.Is(linkErr.Err, syscall.EXDEV)
}
