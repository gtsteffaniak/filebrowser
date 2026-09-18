package quota

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
)

func TestUploadCommitDelta_shrinkReturnsNegative(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "file.bin")
	if err := os.WriteFile(dest, make([]byte, 100), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := UploadContext{
		Principal:    &users.User{},
		DestRealPath: dest,
		TotalSize:    50,
		HasKnownSize: true,
	}
	if got := UploadCommitDelta(ctx); got != -50 {
		t.Fatalf("UploadCommitDelta shrink: got %d want -50", got)
	}
	if got := uploadReserveDelta(ctx); got != 0 {
		t.Fatalf("uploadReserveDelta shrink: got %d want 0", got)
	}
}

func TestUploadCommitDelta_growthPositive(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "file.bin")
	if err := os.WriteFile(dest, make([]byte, 40), 0644); err != nil {
		t.Fatal(err)
	}
	ctx := UploadContext{
		Principal:    &users.User{},
		DestRealPath: dest,
		TotalSize:    100,
		HasKnownSize: true,
	}
	if got := UploadCommitDelta(ctx); got != 60 {
		t.Fatalf("UploadCommitDelta growth: got %d want 60", got)
	}
	if got := uploadReserveDelta(ctx); got != 60 {
		t.Fatalf("uploadReserveDelta growth: got %d want 60", got)
	}
}
