package icons

import (
	"image"
	"image/color"
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/database/users"
	"github.com/gtsteffaniak/filebrowser/backend/pkg/settings"
)

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


	settings.Config.Frontend.Styling.DarkBackground = "#111111"
	settings.Config.Frontend.Styling.LightBackground = "#ffffff"

	settings.Config.UserDefaults.UI.DarkMode = users.DarkModeDark
	if got := settings.DefaultBackgroundColor(); got != "#111111" {
		t.Fatalf("dark branch: got %q, want %q", got, "#111111")
	}

	settings.Config.UserDefaults.UI.DarkMode = users.DarkModeLight
	if got := settings.DefaultBackgroundColor(); got != "#ffffff" {
		t.Fatalf("light branch (dark disabled): got %q, want %q", got, "#ffffff")
	}

	settings.Config.UserDefaults.UI.DarkMode = ""
	if got := settings.DefaultBackgroundColor(); got != "#ffffff" {
		t.Fatalf("light branch (unset): got %q, want %q", got, "#ffffff")
	}
}
