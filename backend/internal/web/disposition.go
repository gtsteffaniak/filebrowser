package web

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

func toASCIIFilename(fileName string) string {
	var result strings.Builder
	for _, r := range fileName {
		if r > 127 {
			result.WriteRune('_')
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func SetContentDisposition(w http.ResponseWriter, r *http.Request, fileName string, forceInline bool) {
	dispositionType := "attachment"
	if forceInline || r.URL.Query().Get("inline") == "true" {
		dispositionType = "inline"
		w.Header().Set("Content-Security-Policy", "script-src 'none'")
	}
	asciiFileName := toASCIIFilename(fileName)
	encodedFileName := url.PathEscape(fileName)
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename=%q; filename*=utf-8''%s", dispositionType, asciiFileName, encodedFileName))
}

func IsOnlyOfficeCompatibleFile(fileName string) bool {
	return iteminfo.IsOnlyOffice(fileName)
}
