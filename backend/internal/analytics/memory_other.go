//go:build !linux && !darwin && !windows

package analytics

func systemTotalMemoryMB() int {
	return 0
}
