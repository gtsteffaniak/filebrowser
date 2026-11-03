package textpreview

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRTFToHTML(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{"paragraphs", `{\rtf1\ansi Hello\par World}`, "<p>Hello</p><p>World</p>"},
		{"bold then reset", `{\rtf1 \b bold\b0  plain}`, "<p><strong>bold</strong> plain</p>"},
		{"line break", `{\rtf1 a\line b}`, "<p>a<br>b</p>"},
		{"cp1252 escape", `{\rtf1 caf\'e9}`, "<p>café</p>"},
		{"unicode with fallback", `{\rtf1 \u8364?}`, "<p>€</p>"},
		{"escaped braces", `{\rtf1 a\{b\}c}`, "<p>a{b}c</p>"},
		{"font table skipped", `{\rtf1{\fonttbl{\f0 Arial;}}Visible}`, "<p>Visible</p>"},
		{"ignorable destination skipped", `{\rtf1{\*\generator Word;}Body}`, "<p>Body</p>"},
		{"html is escaped", `{\rtf1 <b>x</b>}`, "<p>&lt;b&gt;x&lt;/b&gt;</p>"},
		{"truncated group", `{\rtf1 open {\b unclosed`, "<p>open <strong>unclosed</strong></p>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, RTFToHTML(tt.in))
		})
	}
}

func TestTrimIncompleteRune(t *testing.T) {
	full := []byte("a\u00e9") // "a" + 2-byte rune
	require.Equal(t, "a", string(trimIncompleteRune(full[:2])))
	require.Equal(t, "a\u00e9", string(trimIncompleteRune(full)))
}

func TestReadTruncatesAndKeepsUTF8Valid(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "big.txt")
	// Multibyte characters so the 16KB cut is likely to land inside one.
	content := strings.Repeat("\u00e9\n", MaxBytes)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))

	res, err := Read(path, "plain")
	require.NoError(t, err)
	require.True(t, res.Truncated)
	require.Equal(t, "plain", res.Kind)
	require.Equal(t, "text", res.Format)
	require.LessOrEqual(t, len(res.Content), MaxBytes)
	require.True(t, strings.HasSuffix(res.Content, "\u00e9"), "cut must land on a whole line")
	require.Equal(t, int64(len(content)), res.Size)
}

func TestReadSmallFileNotTruncated(t *testing.T) {
	path := filepath.Join(t.TempDir(), "small.md")
	require.NoError(t, os.WriteFile(path, []byte("# Title\n"), 0o600))

	res, err := Read(path, "markdown")
	require.NoError(t, err)
	require.False(t, res.Truncated)
	require.Equal(t, "# Title\n", res.Content)
}

func TestReadRTFReturnsHTML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "doc.rtf")
	require.NoError(t, os.WriteFile(path, []byte(`{\rtf1 Hi\par There}`), 0o600))

	res, err := Read(path, "rtf")
	require.NoError(t, err)
	require.Equal(t, "html", res.Format)
	require.Equal(t, "<p>Hi</p><p>There</p>", res.Content)
}

func TestReadRejectsBinary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data.txt")
	require.NoError(t, os.WriteFile(path, []byte("text\x00binary"), 0o600))

	_, err := Read(path, "plain")
	require.ErrorIs(t, err, ErrNotText)
}

func TestReadReplacesInvalidUTF8(t *testing.T) {
	path := filepath.Join(t.TempDir(), "latin1.txt")
	require.NoError(t, os.WriteFile(path, []byte("caf\xe9"), 0o600))

	res, err := Read(path, "plain")
	require.NoError(t, err)
	require.Equal(t, "caf\uFFFD", res.Content)
}
