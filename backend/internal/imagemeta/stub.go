//go:build 386 || arm

package imagemeta

import "context"

// ExtractEmbeddedPreview is unavailable on 32-bit platforms (imagemeta dependency omitted).
func ExtractEmbeddedPreview(ctx context.Context, path string) ([]byte, error) {
	return nil, nil
}

// GetOrientation is unavailable on 32-bit platforms (imagemeta dependency omitted).
func GetOrientation(ctx context.Context, path string) string {
	return ""
}
