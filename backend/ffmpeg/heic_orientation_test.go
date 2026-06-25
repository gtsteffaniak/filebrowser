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

	got := parseDisplayMatrixRotation(stderr)
	if got != -90 {
		t.Fatalf("parseDisplayMatrixRotation() = %v, want -90", got)
	}
}

func TestOrientationNeedsPostCorrection(t *testing.T) {
	tests := []struct {
		orientation     string
		displayRotation float64
		want            bool
	}{
		{"Horizontal (normal)", 0, false},
		{"Rotate 90 CW", -90, false},
		{"Rotate 90 CW", 0, true},
		{"Mirror vertical", 0, true},
		{"Mirror vertical", -90, true},
	}

	for _, tt := range tests {
		got := orientationNeedsPostCorrection(tt.orientation, tt.displayRotation)
		if got != tt.want {
			t.Errorf("orientationNeedsPostCorrection(%q, %v) = %v, want %v", tt.orientation, tt.displayRotation, got, tt.want)
		}
	}
}
