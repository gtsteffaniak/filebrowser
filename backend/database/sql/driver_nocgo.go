//go:build !cgo
// +build !cgo

package sql

import (
	_ "modernc.org/sqlite"
)

const sqliteDriver = "sqlite"

