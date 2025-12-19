//go:build cgo
// +build cgo

package sql

import (
	_ "github.com/mattn/go-sqlite3"
)

const sqliteDriver = "sqlite3"
