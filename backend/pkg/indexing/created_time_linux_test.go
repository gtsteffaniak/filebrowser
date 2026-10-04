//go:build linux

package indexing

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestGetCreatedTime_Symlink checks that the reported birth time follows the supplied
// FileInfo: the link's own time for Lstat info, the target's time for Stat info.
func TestGetCreatedTime_Symlink(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.txt")
	link := filepath.Join(dir, "link")

	if err := os.WriteFile(target, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	// Ensure the link's birth time differs from the target's.
	time.Sleep(50 * time.Millisecond)
	if err := os.Symlink(target, link); err != nil {
		t.Fatal(err)
	}

	targetInfo, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	linkInfo, err := os.Lstat(link)
	if err != nil {
		t.Fatal(err)
	}
	followedInfo, err := os.Stat(link)
	if err != nil {
		t.Fatal(err)
	}

	targetCreated := getCreatedTime(targetInfo, target)
	linkCreated := getCreatedTime(linkInfo, link)
	if targetCreated == nil || linkCreated == nil {
		t.Skip("filesystem does not report birth time")
	}
	if !linkCreated.After(*targetCreated) {
		t.Skipf("filesystem birth times are too coarse to distinguish link (%v) from target (%v)", linkCreated, targetCreated)
	}

	if got := getCreatedTime(linkInfo, link); got == nil || !got.Equal(*linkCreated) {
		t.Errorf("Lstat info: got %v, want link birth time %v", got, linkCreated)
	}
	if got := getCreatedTime(followedInfo, link); got == nil || !got.Equal(*targetCreated) {
		t.Errorf("Stat info: got %v, want target birth time %v", got, targetCreated)
	}
}
