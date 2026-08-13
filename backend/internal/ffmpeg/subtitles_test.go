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
	if tracks[0].Language != "" {
		t.Fatalf("tracks[0].Language = %q, want empty", tracks[0].Language)
	}
	if tracks[1].Language != "" {
		t.Fatalf("tracks[1].Language = %q, want empty", tracks[1].Language)
	}
	if tracks[0].Name != "Track" {
		t.Fatalf("tracks[0].Name = %q, want Track", tracks[0].Name)
	}
	if tracks[1].Name != "Track 1" {
		t.Fatalf("tracks[1].Name = %q, want Track 1", tracks[1].Name)
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
	if tracks[1].Language != "chi" {
		t.Fatalf("tracks[1].Language = %q, want chi", tracks[1].Language)
	}
	if tracks[0].Name != "Track" {
		t.Fatalf("tracks[0].Name = %q, want Track", tracks[0].Name)
	}
	if tracks[1].Name != "Track" {
		t.Fatalf("tracks[1].Name = %q, want Track", tracks[1].Name)
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
	if tracks[0].Language != "eng" {
		t.Fatalf("tracks[0].Language = %q, want eng", tracks[0].Language)
	}
	if tracks[1].Language != "eng" {
		t.Fatalf("tracks[1].Language = %q, want eng", tracks[1].Language)
	}
	if tracks[0].Name != "Track" {
		t.Fatalf("tracks[0].Name = %q, want Track", tracks[0].Name)
	}
	if tracks[1].Name != "Track 1" {
		t.Fatalf("tracks[1].Name = %q, want Track 1", tracks[1].Name)
	}
	if tracks[0].Index == nil || *tracks[0].Index != 0 {
		t.Fatalf("tracks[0].Index = %v, want 0", tracks[0].Index)
	}
	if tracks[1].Index == nil || *tracks[1].Index != 1 {
		t.Fatalf("tracks[1].Index = %v, want 1", tracks[1].Index)
	}
}

func TestMapSubtitleTracks_duplicateLanguageNoTitle(t *testing.T) {
	tracks := mapSubtitleTracks([]goffmpeg.SubtitleTrack{
		{Index: 2, Language: "eng"},
		{Index: 3, Language: "eng"},
	})
	if len(tracks) != 2 {
		t.Fatalf("len(tracks) = %d, want 2", len(tracks))
	}
	if tracks[0].Language != "eng" || tracks[1].Language != "eng" {
		t.Fatalf("languages = %q, %q, want eng, eng", tracks[0].Language, tracks[1].Language)
	}
	if tracks[0].Name != "Track" {
		t.Fatalf("tracks[0].Name = %q, want Track", tracks[0].Name)
	}
	if tracks[1].Name != "Track 1" {
		t.Fatalf("tracks[1].Name = %q, want Track 1", tracks[1].Name)
	}
}

func TestMapSubtitleTracks_mixedEngAndUndefined(t *testing.T) {
	tracks := mapSubtitleTracks([]goffmpeg.SubtitleTrack{
		{Index: 2, Language: "eng"},
		{Index: 3},
		{Index: 4},
	})
	if len(tracks) != 3 {
		t.Fatalf("len(tracks) = %d, want 3", len(tracks))
	}
	if tracks[0].Name != "Track" {
		t.Fatalf("tracks[0].Name = %q", tracks[0].Name)
	}
	if tracks[1].Name != "Track" {
		t.Fatalf("tracks[1].Name = %q", tracks[1].Name)
	}
	if tracks[2].Name != "Track 1" {
		t.Fatalf("tracks[2].Name = %q", tracks[2].Name)
	}
}

func TestMapSubtitleTracks_multiLanguageMenu(t *testing.T) {
	// fra, ger, ger, eng, eng — matches typical MKV caption menu.
	tracks := mapSubtitleTracks([]goffmpeg.SubtitleTrack{
		{Index: 4, Language: "fra"},
		{Index: 5, Language: "ger"},
		{Index: 6, Language: "ger"},
		{Index: 7, Language: "eng"},
		{Index: 8, Language: "eng"},
	})
	want := []string{"Track", "Track", "Track 1", "Track", "Track 1"}
	if len(tracks) != len(want) {
		t.Fatalf("len(tracks) = %d, want %d", len(tracks), len(want))
	}
	for i, name := range want {
		if tracks[i].Name != name {
			t.Fatalf("tracks[%d].Name = %q, want %q", i, tracks[i].Name, name)
		}
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
			wantName: "Track",
		},
		{
			name:     "language only",
			stream:   goffmpeg.SubtitleTrack{Index: 5, Language: "deu"},
			wantName: "Track",
		},
		{
			name:     "no title or language",
			stream:   goffmpeg.SubtitleTrack{Index: 7},
			wantName: "Track",
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
