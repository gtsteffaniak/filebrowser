package iteminfo

import (
	"testing"

	"github.com/gtsteffaniak/filebrowser/backend/internal/gomedia"
	"github.com/gtsteffaniak/filebrowser/backend/internal/utils"
	"github.com/stretchr/testify/require"
)

func TestScanSidecarSubtitlesSplitsByOwner(t *testing.T) {
	names := []string{"movie.srt", "movie.fr.vtt", "movie.sbv", "movie.lrc", "other.srt", "movie.mkv.srt"}

	goMedia := scanSidecarSubtitles("movie.mkv", names, func(ext string) bool {
		return gomedia.SidecarSubtitleExts[ext]
	})
	require.Equal(t, []utils.SubtitleTrack{
		{Name: "movie.srt", Codec: "srt"},
		{Name: "movie.fr.vtt", Language: "fr", Codec: "vtt"},
	}, goMedia[:2])
	require.Len(t, goMedia, 3, "movie.mkv.srt has the video name as a stem and is included")

	extra := scanSidecarSubtitles("movie.mkv", names, func(ext string) bool {
		return !gomedia.SidecarSubtitleExts[ext]
	})
	require.Len(t, extra, 2, "sbv and lrc stay on the directory scan")
	require.Equal(t, "movie.sbv", extra[0].Name)
	require.Equal(t, "movie.lrc", extra[1].Name)
}
