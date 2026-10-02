package indexing

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

// TestGetCreatedTime checks that a freshly created file reports a recent birth time.
func TestGetCreatedTime(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "created_time_test.txt")
	if err := os.WriteFile(path, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	created := getCreatedTime(info, path)

	switch runtime.GOOS {
	case "darwin", "windows":
	case "linux":
		if created == nil {
			t.Skip("filesystem does not report birth time")
		}
	default:
		if created != nil {
			t.Errorf("expected nil created time on %s, got %v", runtime.GOOS, created)
		}
		return
	}

	if created == nil {
		t.Fatal("expected non-nil created time")
	}
	if diff := time.Since(*created); diff < -time.Minute || diff > time.Minute {
		t.Errorf("created time %v is not within one minute of now", created)
	}
}

// TestGetCreatedTime_NilInfo checks that a nil FileInfo yields no creation time.
func TestGetCreatedTime_NilInfo(t *testing.T) {
	if got := getCreatedTime(nil, ""); got != nil {
		t.Errorf("expected nil for nil info, got %v", got)
	}
}
