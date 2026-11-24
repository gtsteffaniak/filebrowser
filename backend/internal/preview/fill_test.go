package preview

import (
	"fmt"
	"image"
	"image/color"
	"runtime"
	"testing"

	"github.com/kovidgoyal/imaging"
	"github.com/stretchr/testify/require"
)

// patternYCbCr is a 4:2:0 image (what a camera JPEG decodes to) with a pattern that differs in
// every row and column, so a crop off by one pixel shows up in the resized output.
func patternYCbCr(w, h int) *image.YCbCr {
	return patternYCbCrRatio(w, h, image.YCbCrSubsampleRatio420)
}

// patternYCbCrRatio is patternYCbCr with the given chroma subsampling.
func patternYCbCrRatio(w, h int, r image.YCbCrSubsampleRatio) *image.YCbCr {
	img := image.NewYCbCr(image.Rect(0, 0, w, h), r)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Y[img.YOffset(x, y)] = uint8((x*7 + y*13) % 251)
		}
	}
	for i := range img.Cb {
		img.Cb[i] = uint8((i*3 + i/img.CStride) % 256)
		img.Cr[i] = uint8((i + 5*(i/img.CStride)) % 256)
	}
	return img
}

// patternNRGBA is an NRGBA image whose pixels differ in every row and column.
func patternNRGBA(w, h int) *image.NRGBA {
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetNRGBA(x, y, color.NRGBA{uint8(x % 256), uint8(y % 256), uint8((x + y) % 256), 255})
		}
	}
	return img
}

// requireSamePixels fails unless want and got are the same size and every pixel is equal,
// comparing from each image's own origin.
func requireSamePixels(t *testing.T, want, got image.Image) {
	t.Helper()
	require.Equal(t, want.Bounds().Size(), got.Bounds().Size())
	wb, gb := want.Bounds().Min, got.Bounds().Min
	for y := 0; y < want.Bounds().Dy(); y++ {
		for x := 0; x < want.Bounds().Dx(); x++ {
			wr, wg, wbl, wa := want.At(wb.X+x, wb.Y+y).RGBA()
			gr, gg, gbl, ga := got.At(gb.X+x, gb.Y+y).RGBA()
			if wr != gr || wg != gg || wbl != gbl || wa != ga {
				t.Fatalf("pixel (%d,%d) differs: imaging.Fill %v, fillCenter %v", x, y,
					[4]uint32{wr, wg, wbl, wa}, [4]uint32{gr, gg, gbl, ga})
			}
		}
	}
}

func matchImagingFill(t *testing.T, src image.Image, w, h int, filter imaging.ResampleFilter) {
	t.Helper()
	want := imaging.Fill(src, w, h, imaging.Center, filter)
	got := fillCenter(src, w, h, filter)
	require.Equal(t, fmt.Sprintf("%T", want), fmt.Sprintf("%T", got), "image type")
	requireSamePixels(t, want, got)
}

// fillCenter must produce exactly what imaging.Fill(…, imaging.Center, …) produces: the same
// pixels and the same image type, so the encoded preview is the same bytes.
func TestFillCenter_MatchesImagingFill(t *testing.T) {
	box := imaging.Box
	type tc struct {
		name string
		src  image.Image
		w, h int
	}
	cases := []tc{
		{"ycbcr landscape", patternYCbCr(1200, 801), 256, 256},
		{"ycbcr portrait", patternYCbCr(601, 1000), 320, 180},
		{"nrgba square", patternNRGBA(700, 700), 256, 256},
		{"nrgba wide", patternNRGBA(1500, 300), 320, 180},
		{"small (falls back to imaging.Fill)", patternNRGBA(90, 60), 256, 256},
		{"exact target size", patternNRGBA(256, 256), 256, 256},
		{"offset origin", patternNRGBA(900, 700).SubImage(image.Rect(37, 11, 837, 611)), 256, 256},
		{"nycbcra 444 600x264", &image.NYCbCrA{YCbCr: *patternYCbCrRatio(600, 264, image.YCbCrSubsampleRatio444), A: make([]uint8, 600*264), AStride: 600}, 256, 256},
		// 420 on tall and odd sizes: crop offset reaches last column/row (stride edges).
		{"ycbcr 420 264x600", patternYCbCrRatio(264, 600, image.YCbCrSubsampleRatio420), 180, 320},
		{"ycbcr 420 601x263", patternYCbCrRatio(601, 263, image.YCbCrSubsampleRatio420), 264, 264},
	}
	// Every subsampling ratio on 600x264: centered crop with x offset through the bottom row.
	for _, r := range []image.YCbCrSubsampleRatio{
		image.YCbCrSubsampleRatio444, image.YCbCrSubsampleRatio422, image.YCbCrSubsampleRatio420,
		image.YCbCrSubsampleRatio440, image.YCbCrSubsampleRatio411, image.YCbCrSubsampleRatio410,
	} {
		src := patternYCbCrRatio(600, 264, r)
		cases = append(cases,
			tc{fmt.Sprintf("ycbcr %v 600x264 scale", r), src, 256, 256},
			tc{fmt.Sprintf("ycbcr %v 600x264 exact crop", r), src, 264, 264},
		)
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			matchImagingFill(t, c.src, c.w, c.h, box)
		})
	}
}

// Resample filters share one code path in fillCenter; verify each against imaging.Fill once.
func TestFillCenter_MatchesImagingFill_Filters(t *testing.T) {
	src := patternYCbCr(1200, 801)
	for name, filter := range map[string]imaging.ResampleFilter{
		"nearest":    imaging.NearestNeighbor,
		"box":        imaging.Box,
		"catmullrom": imaging.CatmullRom,
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			matchImagingFill(t, src, 256, 256, filter)
		})
	}
}

// A square thumbnail of a large photo must not allocate a full-resolution copy of it.
func TestFillCenter_NoFullSizeCopy(t *testing.T) {
	const w, h = 4000, 3000
	src := patternYCbCr(w, h)
	fullCopy := uint64(h * h * 4) // the square crop imaging.Fill materializes as NRGBA

	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	out := fillCenter(src, 256, 256, imaging.Box)
	runtime.ReadMemStats(&after)

	require.Equal(t, image.Pt(256, 256), out.Bounds().Size())
	allocated := after.TotalAlloc - before.TotalAlloc
	require.Less(t, allocated, fullCopy/4,
		"fillCenter allocated %d bytes; a copy of the crop would be %d", allocated, fullCopy)
}