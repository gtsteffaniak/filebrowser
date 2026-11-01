package analytics

import "testing"

func TestMapSystemOS(t *testing.T) {
	tests := []struct {
		name string
		goos string
		want string
	}{
		{name: "linux", goos: "linux", want: "linux"},
		{name: "windows", goos: "windows", want: "windows"},
		{name: "darwin", goos: "darwin", want: "darwin"},
		{name: "freebsd release build", goos: "freebsd", want: "freebsd"},
		{name: "unsupported goos", goos: "plan9", want: "other"},
		{name: "empty", goos: "", want: "other"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := mapSystemOS(tt.goos); got != tt.want {
				t.Errorf("mapSystemOS(%q) = %q, want %q", tt.goos, got, tt.want)
			}
		})
	}
}
