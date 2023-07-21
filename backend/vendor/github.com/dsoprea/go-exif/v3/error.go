package exif

import (
	"errors"
)

var (
	// ErrTagNotFound indicates that the tag was not found.
	ErrTagNotFound = errors.New("tag not found")

	// ErrTagNotKnown indicates that the tag is not registered with us as a
	// known tag.
	ErrTagNotKnown = errors.New("tag is not known")
)
