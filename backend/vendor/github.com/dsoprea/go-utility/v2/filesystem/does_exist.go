package rifs

import (
	"os"
)

// DoesExist returns true if we can open the given file/path without error. We
// can't simply use `os.IsNotExist()` because we'll get a different error when
// the parent directory doesn't exist, and really the only important thing is if
// it exists *and* it's readable.
func DoesExist(filepath string) bool {
	f, err := os.Open(filepath)
	if err != nil {
		return false
	}

	f.Close()
	return true
}
