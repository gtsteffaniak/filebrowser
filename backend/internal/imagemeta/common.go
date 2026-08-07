package imagemeta

// IsJPEG reports whether data begins with a JPEG SOI marker.
func IsJPEG(data []byte) bool {
	return len(data) >= 2 && data[0] == 0xff && data[1] == 0xd8
}
