package utils

import (
	"path"

	"golang.org/x/text/unicode/norm"
)

// NormalizeFinalSegment applies a Unicode normalization form to the FINAL
// path segment — the name being created or renamed — per the configured
// server.filesystem.filenameNormalization (#2823).
//
// Only the final segment is touched: parent directories already exist with
// whatever form they were created in, and re-normalizing a full path would
// stop it from matching an existing NFD-named directory on disk. macOS
// uploads arrive as NFD while the macOS SMB client sends NFC, which makes
// NFD-named files openable in Finder listings but unreadable over SMB; an
// explicit form lets deployments align new names with their consumers.
//
// form is one of "none" (default — no change), "nfc", or "nfd"; anything
// else is treated as "none".
func NormalizeFinalSegment(p, form string) string {
	if form != "nfc" && form != "nfd" {
		return p
	}
	dir, base := path.Split(p)
	if base == "" || base == "/" {
		return p
	}
	normalized := base
	if form == "nfc" {
		normalized = norm.NFC.String(base)
	} else {
		normalized = norm.NFD.String(base)
	}
	if normalized == base {
		return p
	}
	return dir + normalized
}
