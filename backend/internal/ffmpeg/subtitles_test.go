package ffmpeg

import (
	"testing"

	goffmpeg "github.com/gtsteffaniak/go-ffmpeg"
)

func TestMapSubtitleTracks_uniqueUndefinedLanguage(t *testing.T) {
	tracks := mapSubtitleTracks([]goffmpeg.SubtitleTrack{
		{Index: 2},
		{Index: 3},
	})
	if len(tracks) != 2 {
		t.Fatalf("len(tracks) = %d, want 2", len(tracks))
	}
	if tracks[0].Language != "und" {
		t.Fatalf("tracks[0].Language = %q, want und", tracks[0].Language)
	}
	if tracks[1].Language != "und-3" {
		t.Fatalf("tracks[1].Language = %q, want und-3", tracks[1].Language)
	}
	if tracks[0].Language == tracks[1].Language {
		t.Fatalf("duplicate subtitle languages: %q", tracks[0].Language)
	}
}

func TestMapSubtitleTracks_duplicateLanguage(t *testing.T) {
	tracks := mapSubtitleTracks([]goffmpeg.SubtitleTrack{
		{Index: 2, Language: "chi", Title: "Simplified Chinese"},
		{Index: 3, Language: "chi", Title: "Traditional Chinese"},
	})
	if len(tracks) != 2 {
		t.Fatalf("len(tracks) = %d, want 2", len(tracks))
	}
	if tracks[0].Language != "chi" {
		t.Fatalf("tracks[0].Language = %q, want chi", tracks[0].Language)
	}
	if tracks[1].Language != "chi-3" {
		t.Fatalf("tracks[1].Language = %q, want chi-3", tracks[1].Language)
	}
}

func TestMapSubtitleTracks_uniqueNames(t *testing.T) {
	tracks := mapSubtitleTracks([]goffmpeg.SubtitleTrack{
		{Index: 0, Title: "English", Language: "eng"},
		{Index: 1, Title: "English", Language: "eng"},
	})
	if len(tracks) != 2 {
		t.Fatalf("len(tracks) = %d, want 2", len(tracks))
	}
	if tracks[0].Name == tracks[1].Name {
		t.Fatalf("duplicate subtitle names: %q", tracks[0].Name)
	}
	if tracks[0].Language == tracks[1].Language {
		t.Fatalf("duplicate subtitle languages: %q", tracks[0].Language)
	}
	if tracks[0].Language != "eng" {
		t.Fatalf("tracks[0].Language = %q, want eng", tracks[0].Language)
	}
	if tracks[1].Language != "eng-1" {
		t.Fatalf("tracks[1].Language = %q, want eng-1", tracks[1].Language)
	}
	if tracks[0].Index == nil || *tracks[0].Index != 0 {
		t.Fatalf("tracks[0].Index = %v, want 0", tracks[0].Index)
	}
	if tracks[1].Index == nil || *tracks[1].Index != 1 {
		t.Fatalf("tracks[1].Index = %v, want 1", tracks[1].Index)
	}
}

func TestMapSubtitleTracks_nameFormats(t *testing.T) {
	tests := []struct {
		name     string
		stream   goffmpeg.SubtitleTrack
		wantName string
	}{
		{
			name:     "title only",
			stream:   goffmpeg.SubtitleTrack{Index: 3, Title: "Director's Commentary"},
			wantName: "Embedded Subtitle 3 (Director's Commentary)",
		},
		{
			name:     "language only",
			stream:   goffmpeg.SubtitleTrack{Index: 5, Language: "deu"},
			wantName: "Embedded Subtitle 5 (deu)",
		},
		{
			name:     "no title or language",
			stream:   goffmpeg.SubtitleTrack{Index: 7},
			wantName: "Embedded Subtitle 7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := mapSubtitleTracks([]goffmpeg.SubtitleTrack{tt.stream})
			if len(got) != 1 {
				t.Fatalf("len = %d, want 1", len(got))
			}
			if got[0].Name != tt.wantName {
				t.Fatalf("Name = %q, want %q", got[0].Name, tt.wantName)
			}
		})
	}
}
