//go:build !unix && !windows

package settings

func unixUmask(_ int) int {
	return 0
}
