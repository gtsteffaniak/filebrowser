package sqlitebusy

import (
	"errors"
	"testing"
)

func TestIsBusyOrLocked(t *testing.T) {
	if !IsBusyOrLocked(errors.New("database is locked (5)")) {
		t.Fatal("expected busy")
	}
	if IsBusyOrLocked(errors.New("no such table")) {
		t.Fatal("expected not busy")
	}
}

func TestWrap(t *testing.T) {
	root := errors.New("database is locked (5)")
	wrapped := Wrap(root)
	if !errors.Is(wrapped, ErrBusy) {
		t.Fatalf("expected ErrBusy, got %v", wrapped)
	}
}
