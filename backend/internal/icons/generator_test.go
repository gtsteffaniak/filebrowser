package icons

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
	"github.com/kovidgoyal/imaging"
)

func TestGenerateMaskableIconKeepsCanvasOpaque(t *testing.T) {
	originalCacheDir := settings.Config.Server.CacheDir
	settings.Config.Server.CacheDir = t.TempDir()
	t.Cleanup(func() { settings.Config.Server.CacheDir = originalCacheDir })
	if err := os.MkdirAll(settings.PWAIconsCacheDir(), 0o755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name       string
		background color.Color
		source     func(*image.RGBA)
	}{
		{
			name:       "transparent padding",
			background: color.White,
			source: func(img *image.RGBA) {
				img.Set(5, 5, color.RGBA{R: 20, G: 40, B: 200, A: 255})
			},
		},
		{
			name:       "opaque edge",
			background: color.NRGBA{R: 200, G: 100, B: 50, A: 255},
			source: func(img *image.RGBA) {
				for x := 0; x < 12; x++ {
					img.Set(x, 0, color.RGBA{R: 200, G: 100, B: 50, A: 255})
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := image.NewRGBA(image.Rect(0, 0, 12, 12))
			tt.source(source)
			var sourceData bytes.Buffer
			if err := png.Encode(&sourceData, source); err != nil {
				t.Fatal(err)
			}

			const size = 96
			icon := iconSize{size: size, name: "maskable.png", maskable: true}
			if err := generateMaskableIcon(sourceData.Bytes(), icon); err != nil {
				t.Fatal(err)
			}
			output, err := imaging.Open(filepath.Join(settings.PWAIconsCacheDir(), icon.name))
			if err != nil {
				t.Fatal(err)
			}

			if output.Bounds().Dx() != size || output.Bounds().Dy() != size {
				t.Fatalf("got bounds %v, want %dx%d", output.Bounds(), size, size)
			}
			want, ok := color.NRGBAModel.Convert(tt.background).(color.NRGBA)
			if !ok {
				t.Fatalf("background color %#v is not NRGBA", tt.background)
			}
			for y := 0; y < size; y++ {
				for x := 0; x < size; x++ {
					outsideArt := x < 16 || x >= 80 || y < 16 || y >= 80
					got, ok := color.NRGBAModel.Convert(output.At(x, y)).(color.NRGBA)
					if !ok {
						t.Fatalf("pixel (%d,%d) is not NRGBA", x, y)
					}
					if got.A != 255 {
						t.Fatalf("pixel (%d,%d) has alpha %d, want opaque", x, y, got.A)
					}
					if outsideArt && got != want {
						t.Fatalf("pixel (%d,%d) got %#v, want background %#v", x, y, got, want)
					}
				}
			}
		})
	}
}

func TestMaskableBackgroundOpaqueCorner(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 12, 12))
	img.Set(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})

	got := maskableBackground(img)
	want := color.NRGBA{R: 10, G: 20, B: 30, A: 255}
	if got != want {
		t.Fatalf("opaque corner: got %#v, want %#v", got, want)
	}
}

func TestMaskableBackgroundOpaqueEdge(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 12, 12))
	img.Set(6, 0, color.RGBA{R: 200, G: 100, B: 50, A: 255})

	got := maskableBackground(img)
	want := color.NRGBA{R: 200, G: 100, B: 50, A: 255}
	if got != want {
		t.Fatalf("opaque edge: got %#v, want %#v", got, want)
	}
}

func TestMaskableBackgroundAllTransparent(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 12, 12))
	if got := maskableBackground(img); got != color.White {
		t.Fatalf("all transparent: got %#v, want white", got)
	}
}

func TestDefaultBackgroundColor(t *testing.T) {
	saved := settings.Config
	defer func() { settings.Config = saved }()

	dark := true
	light := false

	settings.Config.Frontend.Styling.DarkBackground = "#111111"
	settings.Config.Frontend.Styling.LightBackground = "#ffffff"

	settings.Config.UserDefaults.UI.DarkMode = &dark
	if got := settings.DefaultBackgroundColor(); got != "#111111" {
		t.Fatalf("dark branch: got %q, want %q", got, "#111111")
	}

	settings.Config.UserDefaults.UI.DarkMode = &light
	if got := settings.DefaultBackgroundColor(); got != "#ffffff" {
		t.Fatalf("light branch (dark disabled): got %q, want %q", got, "#ffffff")
	}

	settings.Config.UserDefaults.UI.DarkMode = nil
	if got := settings.DefaultBackgroundColor(); got != "#ffffff" {
		t.Fatalf("light branch (unset): got %q, want %q", got, "#ffffff")
	}
}
