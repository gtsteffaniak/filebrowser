package preview

import (
	"github.com/gtsteffaniak/filebrowser/backend/internal/opensw"
)

// ExtractSolidWorksPreview is the CAD branch of preview generation: the
// embedded image SolidWorks saves inside its documents, read by the opensw
// package (a Go port of OpenSW's container readers).
func ExtractSolidWorksPreview(realPath string) ([]byte, error) {
	return opensw.ExtractSolidWorksPreview(realPath)
}
