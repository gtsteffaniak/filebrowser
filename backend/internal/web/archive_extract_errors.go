package web

import (
	"errors"
	"fmt"
)

// errExtractAccessDenied marks access-rule denials during extraction (use errors.Is).
var errExtractAccessDenied = errors.New("extract access denied")

type extractAccessDeniedError struct {
	entry string
}

func (e *extractAccessDeniedError) Error() string {
	return fmt.Sprintf("access denied: %s", e.entry)
}

func (e *extractAccessDeniedError) Unwrap() error {
	return errExtractAccessDenied
}

func newExtractAccessDenied(archiveEntryName string) error {
	display := archiveEntryName
	if normalized, err := normalizeArchiveEntryName(archiveEntryName); err == nil && normalized != "" {
		display = normalized
	}
	return &extractAccessDeniedError{entry: display}
}

// errEntryPathInvalid and errEntryPathEscape are returned from path resolution without filesystem paths in Error().
var (
	errEntryPathInvalid = errors.New("archive entry path is invalid")
	errEntryPathEscape  = errors.New("archive entry path escapes destination")
)

// clientExtractError returns a generic extraction failure for HTTP responses (details are logged server-side).
func clientExtractError() error {
	return errors.New("archive extraction failed")
}
