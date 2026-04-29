package iteminfo

import "testing"

func TestFileContributesToFolderPreviewThumbnail(t *testing.T) {
	tests := []struct {
		name  string
		item  ItemInfo
		want  bool
	}{
		{
			name: "video with preview contributes",
			item: ItemInfo{
				Name:       "clip.mp4",
				Type:       "video/mp4",
				HasPreview: true,
			},
			want: true,
		},
		{
			name: "markdown with in-file preview does not contribute to folder strip",
			item: ItemInfo{
				Name:       "README.md",
				Type:       "text/markdown",
				HasPreview: true,
			},
			want: false,
		},
		{
			name: "image with preview contributes",
			item: ItemInfo{
				Name:       "a.png",
				Type:       "image/png",
				HasPreview: true,
			},
			want: true,
		},
		{
			name: "image without preview flag does not contribute",
			item: ItemInfo{
				Name:       "b.png",
				Type:       "image/png",
				HasPreview: false,
			},
			want: false,
		},
		{
			name: "audio type with preview false does not contribute",
			item: ItemInfo{
				Name:       "track.mp3",
				Type:       "audio/mpeg",
				HasPreview: false,
			},
			want: false,
		},
		{
			name: "directory never contributes as file child",
			item: ItemInfo{
				Name:       "sub",
				Type:       "directory",
				HasPreview: true,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FileContributesToFolderPreviewThumbnail(tt.item); got != tt.want {
				t.Fatalf("FileContributesToFolderPreviewThumbnail() = %v, want %v", got, tt.want)
			}
		})
	}
}
