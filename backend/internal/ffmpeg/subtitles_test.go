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
	if tracks[0].Name != "Track 2" {
		t.Fatalf("tracks[0].Name = %q, want Track 2", tracks[0].Name)
	}
	if tracks[1].Name != "Track 3" {
		t.Fatalf("tracks[1].Name = %q, want Track 3", tracks[1].Name)
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
	if tracks[0].Name != "Track 2" {
		t.Fatalf("tracks[0].Name = %q, want Track 2", tracks[0].Name)
	}
	if tracks[1].Name != "Track 3" {
		t.Fatalf("tracks[1].Name = %q, want Track 3", tracks[1].Name)
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
	if tracks[0].Name != "Track 0" {
		t.Fatalf("tracks[0].Name = %q, want Track 0", tracks[0].Name)
	}
	if tracks[1].Name != "Track 1" {
		t.Fatalf("tracks[1].Name = %q, want Track 1", tracks[1].Name)
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
	if tracks[0].Name != "Track 2" {
		t.Fatalf("tracks[0].Name = %q, want Track 2", tracks[0].Name)
	}
	if tracks[1].Name != "Track 3" {
		t.Fatalf("tracks[1].Name = %q, want Track 3", tracks[1].Name)
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
	if tracks[0].Name != "Track 2" {
		t.Fatalf("tracks[0].Name = %q", tracks[0].Name)
	}
	if tracks[1].Name != "Track 3" {
		t.Fatalf("tracks[1].Name = %q", tracks[1].Name)
	}
	if tracks[2].Name != "Track 4" {
		t.Fatalf("tracks[2].Name = %q", tracks[2].Name)
	}
}

func TestMapSubtitleTracks_multiLanguageMenu(t *testing.T) {
	tracks := mapSubtitleTracks([]goffmpeg.SubtitleTrack{
		{Index: 4, Language: "fra"},
		{Index: 5, Language: "ger"},
		{Index: 6, Language: "ger"},
		{Index: 7, Language: "eng"},
		{Index: 8, Language: "eng"},
	})
	wantNames := []string{"Track 4", "Track 5", "Track 6", "Track 7", "Track 8"}
	if len(tracks) != len(wantNames) {
		t.Fatalf("len(tracks) = %d, want %d", len(tracks), len(wantNames))
	}
	for i, name := range wantNames {
		if tracks[i].Name != name {
			t.Fatalf("tracks[%d].Name = %q, want %q", i, tracks[i].Name, name)
		}
	}
	wantSrclang := []string{"fra", "ger", "x-track-6", "eng", "x-track-8"}
	for i, srclang := range wantSrclang {
		if tracks[i].Srclang != srclang {
			t.Fatalf("tracks[%d].Srclang = %q, want %q", i, tracks[i].Srclang, srclang)
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
			wantName: "Track 3",
		},
		{
			name:     "language only",
			stream:   goffmpeg.SubtitleTrack{Index: 5, Language: "deu"},
			wantName: "Track 5",
		},
		{
			name:     "no title or language",
			stream:   goffmpeg.SubtitleTrack{Index: 7},
			wantName: "Track 7",
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
