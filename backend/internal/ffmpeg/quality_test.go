package ffmpeg

import (
	"strconv"
	"testing"

	"github.com/gtsteffaniak/go-ffmpeg/encode"
	"github.com/stretchr/testify/require"
)

func TestParseQuality_DefaultAndNewScale(t *testing.T) {
	t.Parallel()
	require.Equal(t, frameQualityForLegacyMJPEGQScale(5), parseQuality(""))
	require.Equal(t, frameQualityForLegacyMJPEGQScale(5), parseQuality("not-a-number"))
	require.Equal(t, 85, parseQuality("85"))
	require.Equal(t, 100, parseQuality("150"))
}

func TestParseQuality_LegacyQScaleMapsToSameMJPEGQScale(t *testing.T) {
	t.Parallel()
	for legacy := 2; legacy <= 10; legacy++ {
		got := parseQuality(itoa(legacy))
		require.Equal(t, encode.MJPEGQualityQScale(got), itoa(legacy),
			"legacy q:v %d", legacy)
	}
	require.Equal(t, 100, parseQuality("1"))
	require.Equal(t, "2", encode.MJPEGQualityQScale(parseQuality("1")))
}

func TestFrameQualityForLegacyMJPEGQScale_KnownPoints(t *testing.T) {
	t.Parallel()
	require.Equal(t, 89, frameQualityForLegacyMJPEGQScale(5))
	require.Equal(t, 74, frameQualityForLegacyMJPEGQScale(10))
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
