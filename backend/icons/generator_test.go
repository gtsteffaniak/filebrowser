package icons

import (
	"image"
	"image/color"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/common/settings"
)

func TestMaskableBackgroundOpaqueCorner(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 12, 12)) // fully transparent by default
	// an opaque top-left corner is the first sampled point
	img.Set(0, 0, color.RGBA{R: 10, G: 20, B: 30, A: 255})

	got := maskableBackground(img)
	want := color.NRGBA{R: 10, G: 20, B: 30, A: 255}
	if got != want {
		t.Fatalf("opaque corner: got %#v, want %#v", got, want)
	}
}

func TestMaskableBackgroundOpaqueEdge(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 12, 12))
	// corners stay transparent; only the top edge midpoint (cx, 0) is opaque
	img.Set(6, 0, color.RGBA{R: 200, G: 100, B: 50, A: 255})

	got := maskableBackground(img)
	want := color.NRGBA{R: 200, G: 100, B: 50, A: 255}
	if got != want {
		t.Fatalf("opaque edge: got %#v, want %#v", got, want)
	}
}

func TestMaskableBackgroundAllTransparent(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 12, 12)) // every sampled point is transparent
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
