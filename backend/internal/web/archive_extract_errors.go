package web

import "errors"

// errExtractSkip means the archive entry is omitted because access rules deny it (not an extraction failure).
var errExtractSkip = errors.New("extract entry skipped by access rules")

// errEntryPathInvalid and errEntryPathEscape are returned from path resolution without filesystem paths in Error().
var (
	errEntryPathInvalid = errors.New("archive entry path is invalid")
	errEntryPathEscape  = errors.New("archive entry path escapes destination")
)

// clientExtractError returns a generic extraction failure for HTTP responses (details are logged server-side).
func clientExtractError() error {
	return errors.New("archive extraction failed")
}
