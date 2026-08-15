//go:build mupdf

package preview

// Link MuPDF static libraries through go-fitz so preview CGO can call fz_* without
// duplicating platform-specific LDFLAGS in this package.
import _ "github.com/gen2brain/go-fitz"
