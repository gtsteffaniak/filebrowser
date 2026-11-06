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

// fillCenter must produce exactly what imaging.Fill(…, imaging.Center, …) produces: the same
// pixels and the same image type, so the encoded preview is the same bytes.
func TestFillCenter_MatchesImagingFill(t *testing.T) {
	sources := map[string]image.Image{
		"ycbcr landscape":                    patternYCbCr(1200, 801),
		"ycbcr portrait":                     patternYCbCr(601, 1000),
		"nrgba square":                       patternNRGBA(700, 700),
		"nrgba wide":                         patternNRGBA(1500, 300),
		"small (falls back to imaging.Fill)": patternNRGBA(90, 60),
		"exact target size":                  patternNRGBA(256, 256),
		"offset origin":                      patternNRGBA(900, 700).SubImage(image.Rect(37, 11, 837, 611)),
	}
	// Every subsampling ratio, in shapes whose centered crop has an x (or y) offset and reaches
	// the source's last row (or column) -- where a SubImage's buffer ends short of a full stride.
	for _, r := range []image.YCbCrSubsampleRatio{
		image.YCbCrSubsampleRatio444, image.YCbCrSubsampleRatio422, image.YCbCrSubsampleRatio420,
		image.YCbCrSubsampleRatio440, image.YCbCrSubsampleRatio411, image.YCbCrSubsampleRatio410,
	} {
		for _, sz := range [][2]int{{600, 264}, {264, 600}, {601, 263}} {
			sources[fmt.Sprintf("ycbcr %v %dx%d", r, sz[0], sz[1])] = patternYCbCrRatio(sz[0], sz[1], r)
		}
	}
	sources["nycbcra 444 600x264"] = &image.NYCbCrA{YCbCr: *patternYCbCrRatio(600, 264, image.YCbCrSubsampleRatio444), A: make([]uint8, 600*264), AStride: 600}
	// 264x264 is the crop of the 600x264 and 264x600 sources itself: nothing to scale.
	targets := [][2]int{{256, 256}, {320, 180}, {180, 320}, {264, 264}}
	filters := map[string]imaging.ResampleFilter{
		"nearest": imaging.NearestNeighbor, "box": imaging.Box, "catmullrom": imaging.CatmullRom,
	}
	for name, src := range sources {
		for _, tg := range targets {
			for fname, f := range filters {
				t.Run(fmt.Sprintf("%s/%dx%d/%s", name, tg[0], tg[1], fname), func(t *testing.T) {
					want := imaging.Fill(src, tg[0], tg[1], imaging.Center, f)
					got := fillCenter(src, tg[0], tg[1], f)
					require.Equal(t, fmt.Sprintf("%T", want), fmt.Sprintf("%T", got), "image type")
					requireSamePixels(t, want, got)
				})
			}
		}
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
