//go:build windows

package fileutils

import (
	"errors"
	"os"
	"syscall"
)

func isCrossDeviceRenameError(err error) bool {
	const errNotSameDevice = syscall.Errno(17)
	if errors.Is(err, errNotSameDevice) {
		return true
	}
	var linkErr *os.LinkError
	return errors.As(err, &linkErr) && errors.Is(linkErr.Err, errNotSameDevice)
}
