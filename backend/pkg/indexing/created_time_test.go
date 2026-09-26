package indexing

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

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

	if runtime.GOOS == "linux" && created == nil {
		t.Skip("filesystem does not report birth time")
	}

	if created == nil {
		t.Fatal("expected non-nil created time")
	}
	if diff := time.Since(*created); diff < -time.Minute || diff > time.Minute {
		t.Errorf("created time %v is not within one minute of now", created)
	}
}

func TestGetCreatedTime_NilInfo(t *testing.T) {
	if got := getCreatedTime(nil, ""); got != nil {
		t.Errorf("expected nil for nil info, got %v", got)
	}
}
