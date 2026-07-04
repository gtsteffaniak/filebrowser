package iteminfo

import "testing"

func TestShouldBubbleUpToFolderPreview(t *testing.T) {
	tests := []struct {
		name string
		item ItemInfo
		want bool
	}{
		{
			name: "directory type never bubbles",
			item: ItemInfo{Name: "sub", Type: "directory"},
			want: false,
		},
		{
			name: "text markdown does not bubble",
			item: ItemInfo{Name: "README.md", Type: "text/markdown"},
			want: false,
		},
		{
			name: "plain text does not bubble",
			item: ItemInfo{Name: "LICENSE", Type: "text/plain; charset=utf-8"},
			want: false,
		},
		{
			name: "image does not bubble without HasPreview",
			item: ItemInfo{Name: "a.png", Type: "image/png", HasPreview: false},
			want: false,
		},
		{
			name: "image bubbles when HasPreview",
			item: ItemInfo{Name: "a.png", Type: "image/png", HasPreview: true},
			want: true,
		},
		{
			name: "video bubbles when HasPreview",
			item: ItemInfo{Name: "clip.mp4", Type: "video/mp4", HasPreview: true},
			want: true,
		},
		{
			name: "audio bubbles when HasPreview (e.g. album art)",
			item: ItemInfo{Name: "track.mp3", Type: "audio/mpeg", HasPreview: true},
			want: true,
		},
		{
			name: "OnlyOffice-backed extension does not bubble",
			item: ItemInfo{Name: "report.docx", Type: "application/vnd.openxmlformats-officedocument.wordprocessingml.document"},
			want: false,
		},
		{
			name: "unknown blob type does not bubble",
			item: ItemInfo{Name: "data.bin", Type: "application/octet-stream"},
			want: false,
		},
		{
			name: "empty type does not bubble",
			item: ItemInfo{Name: "weird", Type: ""},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ShouldBubbleUpToFolderPreview(tt.item); got != tt.want {
				t.Fatalf("ShouldBubbleUpToFolderPreview() = %v, want %v", got, tt.want)
			}
		})
	}
}
