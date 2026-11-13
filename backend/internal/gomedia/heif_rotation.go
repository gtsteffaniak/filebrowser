package gomedia

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/gtsteffaniak/filebrowser/backend/internal/ffmpeg"
	"github.com/gtsteffaniak/go-media/pkg/media"
)

// heicRotationShadowTimeout bounds the dev-mode ffmpeg comparison probe.
const heicRotationShadowTimeout = time.Minute

// HEICDisplayRotation returns the display rotation that a HEIC/HEIF/AVIF
// file's irot property asks for, read from the item properties without
// decoding. The value is degrees counter-clockwise in (-180, 180], the same
// number ffmpeg prints as "Display Matrix: rotation of ..." for the file. A
// file with no irot returns 0 and ok=true. ok is false when go-media is
// disabled or cannot read the item properties; the caller then runs the ffmpeg
// probe.
//
// It is installed as ffmpeg's HEIC display rotation reader. In shadow mode the
// ffmpeg stderr probe runs for comparison.
func HEICDisplayRotation(path string) (float64, bool) {
	return timed("heic rotation", path, heicRotationLegacy(path), heicRotationDiff, func() (float64, bool) {
		return heicDisplayRotationFast(path)
	})
}

// heicRotationLegacy returns the ffmpeg stderr probe for shadow mode, or nil
// when ffmpeg is not available.
func heicRotationLegacy(path string) Legacy[float64] {
	if !ShadowEnabled() {
		return nil
	}
	svc := ffmpeg.Get()
	if svc == nil {
		return nil
	}
	return func() (float64, bool) {
		ctx, cancel := context.WithTimeout(context.Background(), heicRotationShadowTimeout)
		defer cancel()
		return svc.GetHEICDisplayMatrixRotation(ctx, path)
	}
}

// heicDisplayRotationFast reads the primary item's irot with go-media and
// converts it to ffmpeg's display rotation.
func heicDisplayRotationFast(path string) (float64, bool) {
	if !Enabled() {
		return 0, false
	}
	// No extension check: the resolved path can have another name (symlink
	// target). go-media sniffs the content and reports other files unsupported.
	f, size, err := openFile(path)
	if err != nil {
		logFallback("heic rotation", path, err)
		return 0, false
	}
	defer f.Close()

	t, err := media.HEIFTransform(f, size)
	if err != nil {
		logFallback("heic rotation", path, err)
		return 0, false
	}
	return displayRotation(t), true
}

// displayRotation converts go-media's counter-clockwise irot angle to ffmpeg's
// display matrix convention: counter-clockwise degrees normalized to
// (-180, 180]. ffmpeg builds the matrix with av_display_rotation_set(-irot)
// and prints av_display_rotation_get, so irot 270 is reported as -90. A mirror
// does not change the rotation value.
func displayRotation(t media.ImageTransform) float64 {
	r := ((t.Rotation % 360) + 360) % 360
	if r > 180 {
		r -= 360
	}
	return float64(r)
}

// heicRotationDiff compares go-media's display rotation with ffmpeg's. 180 and
// -180 are the same rotation.
func heicRotationDiff(fast, legacy float64) []string {
	norm := func(v float64) float64 {
		v = math.Mod(v, 360)
		if v <= -180 {
			v += 360
		} else if v > 180 {
			v -= 360
		}
		return v
	}
	if math.Abs(norm(fast)-norm(legacy)) > 0.5 {
		return []string{fmt.Sprintf("display rotation go-media=%.2f ffmpeg=%.2f", fast, legacy)}
	}
	return nil
}
