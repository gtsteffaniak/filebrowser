package preview

import (
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/ops"
	"github.com/stretchr/testify/require"
)

func TestFfmpegPreviewParamsForSize(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		size      string
		wantW     int
		wantH     int
		wantMode  ops.ScaleMode
		wantQual  int
		wantError bool
	}{
		{
			name:     "small fill",
			size:     "small",
			wantW:    256,
			wantH:    256,
			wantMode: ops.ScaleFill,
			wantQual: 10,
		},
		{
			name:     "large fit",
			size:     "large",
			wantW:    640,
			wantH:    640,
			wantMode: ops.ScaleFit,
			wantQual: 10,
		},
		{
			name:     "xlarge fit",
			size:     "xlarge",
			wantW:    1024,
			wantH:    1024,
			wantMode: ops.ScaleFit,
			wantQual: 10,
		},
		{
			name:     "original no scale",
			size:     "original",
			wantQual: 10,
		},
		{
			name:      "unsupported",
			size:      "huge",
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := ffmpegPreviewParamsForSize(tt.size)
			if tt.wantError {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.wantW, got.width)
			require.Equal(t, tt.wantH, got.height)
			require.Equal(t, tt.wantMode, got.scaleMode)
			require.Equal(t, tt.wantQual, got.quality)
		})
	}
}
