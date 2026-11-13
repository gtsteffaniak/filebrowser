package ffmpeg

import "testing"

func TestParseDisplayMatrixRotation(t *testing.T) {
	stderr := `Input #0, mov,mp4,m4a,3gp,3g2,mj2, from 'test.heic':
  Stream group #0:0[0x31]: Tile Grid: hevc
    Side data:
      Display Matrix: rotation of -90.00 degrees
  Stream #0:48[0x32]: Video: hevc
    Side data:
      Display Matrix: rotation of -0.00 degrees`

	got, ok := parseDisplayMatrixRotation(stderr)
	if !ok {
		t.Fatal("parseDisplayMatrixRotation() ok = false, want true")
	}
	if got != -90 {
		t.Fatalf("parseDisplayMatrixRotation() = %v, want -90", got)
	}
}

func TestParseDisplayMatrixRotation_malformedLine(t *testing.T) {
	_, ok := parseDisplayMatrixRotation("Display Matrix: rotation of\n")
	if ok {
		t.Fatal("expected ok=false for malformed stderr")
	}
}

func TestOrientationNeedsPostCorrection(t *testing.T) {
	tests := []struct {
		orientation     string
		displayRotation float64
		displayKnown    bool
		want            bool
	}{
		{"Horizontal (normal)", 0, true, false},
		{"Rotate 90 CW", -90, true, false},
		{"Rotate 90 CW", 0, true, true},
		{"Rotate 90 CW", 0, false, false},
		{"Mirror vertical", 0, true, true},
		{"Mirror vertical", -90, true, true},
	}

	for _, tt := range tests {
		got := orientationNeedsPostCorrection(tt.orientation, tt.displayRotation, tt.displayKnown)
		if got != tt.want {
			t.Errorf("orientationNeedsPostCorrection(%q, %v, %v) = %v, want %v",
				tt.orientation, tt.displayRotation, tt.displayKnown, got, tt.want)
		}
	}
}

func TestHEICDisplayRotationPrefersReader(t *testing.T) {
	prev := heicDisplayRotationReader.Load()
	t.Cleanup(func() { heicDisplayRotationReader.Store(prev) })

	var calls int
	SetHEICDisplayRotationReader(func(path string) (float64, bool) {
		calls++
		if path == "known.heic" {
			return -90, true
		}
		return 0, false
	})
	var svc *Service // nil: the ffmpeg probe cannot run
	got, ok := svc.HEICDisplayRotation(t.Context(), "known.heic")
	if !ok || got != -90 {
		t.Fatalf("HEICDisplayRotation(known) = %v, %v; want -90, true", got, ok)
	}
	// The reader cannot tell: the ffmpeg probe runs, and with no service it
	// reports unknown rather than "no rotation".
	if _, ok := svc.HEICDisplayRotation(t.Context(), "other.heic"); ok {
		t.Fatal("HEICDisplayRotation(other) ok = true, want the ffmpeg probe's false")
	}
	if calls != 2 {
		t.Fatalf("reader calls = %d, want 2", calls)
	}

	SetHEICDisplayRotationReader(nil)
	if _, ok := svc.HEICDisplayRotation(t.Context(), "known.heic"); ok {
		t.Fatal("with no reader, HEICDisplayRotation must use the ffmpeg probe")
	}
}
