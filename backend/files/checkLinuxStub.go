//go:build !windows
// +build !windows

package files

func checkWindowsHidden(realpath string) bool {
	// Non-Windows platforms don't support hidden attributes in the same way
	return false
}
