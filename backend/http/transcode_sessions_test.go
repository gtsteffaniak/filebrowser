package http

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
)

func newTestSessionStore() *transcodeSessionStore {
	return &transcodeSessionStore{
		sessions: make(map[string]*transcodeSessionEntry),
		byUser:   make(map[uint64]string),
	}
}

func TestTranscodeSessionAcquireUserLimit(t *testing.T) {
	store := newTestSessionStore()
	settings.Config.Integrations.Media.Transcode.MaxConcurrent = 2

	first := store.acquire(1, "alice", "default", "/a.mkv", "a.mkv")
	if !first.OK || first.Session == nil {
		t.Fatalf("expected first acquire to succeed, got %+v", first)
	}

	second := store.acquire(1, "alice", "default", "/b.mkv", "b.mkv")
	if second.OK {
		t.Fatal("expected user limit to block second session for different file")
	}
	if second.Reason != "user_limit" {
		t.Fatalf("expected user_limit, got %q", second.Reason)
	}

	dup := store.acquire(1, "alice", "default", "/a.mkv", "a.mkv")
	if dup.OK {
		t.Fatal("expected user limit to block duplicate stream for same file")
	}

	store.releaseStream(first.Session.ID)
	third := store.acquire(1, "alice", "default", "/b.mkv", "b.mkv")
	if !third.OK {
		t.Fatalf("expected acquire after release to succeed, got %+v", third)
	}
}

func TestTranscodeSessionConcurrentStreamsBlocked(t *testing.T) {
	store := newTestSessionStore()
	settings.Config.Integrations.Media.Transcode.MaxConcurrent = 2

	first := store.acquire(1, "alice", "default", "/a.mkv", "a.mkv")
	if !first.OK {
		t.Fatal("expected first acquire to succeed")
	}

	// Simulate concurrent second connection before first stream ends.
	dup := store.acquire(1, "alice", "default", "/a.mkv", "a.mkv")
	if dup.OK {
		t.Fatal("expected concurrent duplicate acquire to be blocked")
	}

	store.releaseStream(first.Session.ID)
	retry := store.acquire(1, "alice", "default", "/a.mkv", "a.mkv")
	if !retry.OK {
		t.Fatal("expected acquire after stream release to succeed")
	}
}

func TestTranscodeSessionAcquireSystemLimit(t *testing.T) {
	store := newTestSessionStore()
	settings.Config.Integrations.Media.Transcode.MaxConcurrent = 1

	a := store.acquire(1, "alice", "default", "/a.mkv", "a.mkv")
	if !a.OK {
		t.Fatal("expected alice acquire to succeed")
	}
	b := store.acquire(2, "bob", "default", "/b.mkv", "b.mkv")
	if b.OK {
		t.Fatal("expected system limit to block second user")
	}
	if b.Reason != "system_limit" {
		t.Fatalf("expected system_limit, got %q", b.Reason)
	}
}

func TestTranscodeSessionEvaluateBlocksWhenActive(t *testing.T) {
	store := newTestSessionStore()
	settings.Config.Integrations.Media.Transcode.MaxConcurrent = 2

	store.acquire(1, "alice", "default", "/a.mkv", "a.mkv")

	sameFile := store.evaluate(1, "default", "/a.mkv")
	if sameFile.CanStart {
		t.Fatal("expected evaluate to block while stream active (same file)")
	}
	if sameFile.BlockReason != "user_limit" {
		t.Fatalf("expected user_limit, got %q", sameFile.BlockReason)
	}

	otherFile := store.evaluate(1, "default", "/b.mkv")
	if otherFile.CanStart {
		t.Fatal("expected evaluate to block while stream active (other file)")
	}
}

func TestTranscodeSessionReleaseStreamRefcount(t *testing.T) {
	store := newTestSessionStore()
	settings.Config.Integrations.Media.Transcode.MaxConcurrent = 2

	first := store.acquire(1, "alice", "default", "/a.mkv", "a.mkv")
	second := store.acquire(1, "alice", "default", "/a.mkv", "a.mkv")
	if second.OK {
		t.Fatal("expected second acquire while first stream active to fail")
	}

	store.releaseStream(first.Session.ID)
	if _, blocked := store.userHasLiveStream(1); blocked {
		t.Fatal("expected no live stream after release")
	}
}
