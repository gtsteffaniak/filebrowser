// Package exif parses raw EXIF information given a block of raw EXIF data. It
// can also construct new EXIF information, and provides tools for doing so.
// This package is not involved with the parsing of particular file-formats.
//
// The EXIF data must first be extracted and then provided to us. Conversely,
// when constructing new EXIF data, the caller is responsible for packaging
// this in whichever format they require.
package exif
