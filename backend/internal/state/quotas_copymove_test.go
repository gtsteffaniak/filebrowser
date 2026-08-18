package state

import "testing"

func TestCopyMoveItemDelta(t *testing.T) {
	if got := copyMoveItemDelta("move", "/weddingphotos/sub", "/weddingphotos", 1000, 0); got != 0 {
		t.Fatalf("move within quota root: got %d want 0", got)
	}
	if got := copyMoveItemDelta("move", "/other", "/weddingphotos", 1000, 0); got != 1000 {
		t.Fatalf("move into quota root: got %d want 1000", got)
	}
	if got := copyMoveItemDelta("copy", "/other", "/weddingphotos", 1000, 0); got != 1000 {
		t.Fatalf("copy into quota root: got %d want 1000", got)
	}
	if got := copyMoveItemDelta("copy", "/other", "/weddingphotos", 1000, 200); got != 800 {
		t.Fatalf("copy with overwrite: got %d want 800", got)
	}
}
