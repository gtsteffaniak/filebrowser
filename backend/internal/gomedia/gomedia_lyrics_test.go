package gomedia

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestAudioSidecarLyricsSetsFlag checks that a sidecar .lrc alone sets HasLyrics.
func TestAudioSidecarLyricsSetsFlag(t *testing.T) {
	withGate(t, false)
	mp3 := makeMP3(t)
	require.NoError(t, os.WriteFile(filepath.Join(filepath.Dir(mp3), "tone.lrc"), []byte("[00:01.00]hello"), 0o644))

	info, ok := Audio(mp3, false, nil)
	require.True(t, ok)
	require.True(t, info.HasLyrics)

	text, format, ok := Lyrics(mp3, nil)
	require.True(t, ok)
	require.Equal(t, "lrc", format)
	require.Contains(t, text, "hello")
}

// TestAudioNoLyricsNoFlag checks that a plain file reports no lyrics.
func TestAudioNoLyricsNoFlag(t *testing.T) {
	withGate(t, false)
	mp3 := makeMP3(t)

	info, ok := Audio(mp3, false, nil)
	require.True(t, ok)
	require.False(t, info.HasLyrics)

	_, _, ok = Lyrics(mp3, nil)
	require.False(t, ok)
}
