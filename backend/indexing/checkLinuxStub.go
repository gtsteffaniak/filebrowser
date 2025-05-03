//go:build !windows
// +build !windows

package indexing

func CheckWindowsHidden(realpath string) bool {
	// Non-Windows platforms don't support hidden attributes in the same way
	return false
}
