package http

import "testing"

func TestStreamPlaybackWindowClipRange(t *testing.T) {
	t.Parallel()

	w := &streamPlaybackWindow{}
	w.update(streamWindowUpdate{
		CurrentTime: 1800,
		Duration:    3600,
	})

	fileSize := int64(3600 * 1000)

	ok, end := w.clipRange(1780*1000, 1820*1000, fileSize)
	if !ok || end != 1820*1000 {
		t.Fatalf("expected in-window range, got ok=%v end=%d", ok, end)
	}

	ok, _ = w.clipRange(3000*1000, 3100*1000, fileSize)
	if ok {
		t.Fatal("expected far-ahead range to be blocked")
	}

	ok, end = w.clipRange(1850*1000, fileSize-1, fileSize)
	if !ok {
		t.Fatal("expected clipped open-ended range to remain allowed")
	}
	maxByte := timeToByte(1800+45, 3600, fileSize)
	if end > maxByte {
		t.Fatalf("expected end clipped to %d, got %d", maxByte, end)
	}
}

func TestStreamPlaybackWindowMetadataAndSuffix(t *testing.T) {
	t.Parallel()

	w := &streamPlaybackWindow{}
	w.update(streamWindowUpdate{
		CurrentTime: 1800,
		Duration:    3600,
	})

	fileSize := int64(100 * 1024 * 1024)

	ok, end := w.clipRange(0, 1024, fileSize)
	if !ok || end != 1024 {
		t.Fatalf("expected metadata range at start to pass, got ok=%v end=%d", ok, end)
	}

	tailStart := fileSize - streamSuffixBytes + 1024
	ok, end = w.clipRange(tailStart, fileSize-1, fileSize)
	if !ok || end != fileSize-1 {
		t.Fatalf("expected tail index range to pass, got ok=%v end=%d", ok, end)
	}

	ok, end = w.clipRange(50*1024*1024, fileSize-1, fileSize)
	if !ok {
		t.Fatal("expected mid-file open-ended to be clipped, not rejected outright")
	}
	maxByte := timeToByte(1800+45, 3600, fileSize)
	if end > maxByte {
		t.Fatalf("expected open-ended mid-file clipped to %d, got %d", maxByte, end)
	}
}

func TestStreamPlaybackWindowDeniesMidFileWithoutWindow(t *testing.T) {
	t.Parallel()

	w := &streamPlaybackWindow{}
	fileSize := int64(100 * 1024 * 1024)

	ok, _ := w.clipRange(50*1024*1024, 51*1024*1024, fileSize)
	if ok {
		t.Fatal("expected mid-file range denied before window is registered")
	}

	ok, end := w.clipRange(0, 1024, fileSize)
	if !ok || end != 1024 {
		t.Fatalf("expected header probe allowed, got ok=%v end=%d", ok, end)
	}

	tailStart := fileSize - streamSuffixBytes + 1024
	ok, end = w.clipRange(tailStart, fileSize-1, fileSize)
	if !ok || end != fileSize-1 {
		t.Fatalf("expected tail index probe allowed, got ok=%v end=%d", ok, end)
	}
}

func TestStreamPlaybackWindowSeekingGrace(t *testing.T) {
	t.Parallel()

	w := &streamPlaybackWindow{}
	w.update(streamWindowUpdate{
		CurrentTime: 1800,
		Duration:    3600,
		Seeking:     true,
	})

	fileSize := int64(3600 * 1000)
	ok, _ := w.clipRange(1600*1000, 1650*1000, fileSize)
	if !ok {
		t.Fatal("expected seeking grace to allow wider lookback")
	}
}
