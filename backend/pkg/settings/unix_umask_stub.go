//go:build !unix

package settings

func unixUmask(mask int) int {
	return 0
}
