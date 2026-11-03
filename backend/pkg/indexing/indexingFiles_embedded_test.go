package indexing

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
	"github.com/stretchr/testify/require"
)

func TestSetFilePreviewFlags_embeddedFormats(t *testing.T) {
	for _, name := range []string{"art.kra", "comic.cbz", "doc.xps", "letter.odt", "report.docx", "paint.ora"} {
		fi := iteminfo.ItemInfo{Name: name, Type: "application/octet-stream", Size: 1024}
		setFilePreviewFlags(&fi, "/nonexistent/"+name)
		require.True(t, fi.HasPreview, name)
	}
}
