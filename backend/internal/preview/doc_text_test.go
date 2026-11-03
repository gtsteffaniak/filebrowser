package preview

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUtf8PrefixMax(t *testing.T) {
	// Three-byte UTF-8 rune (©) split at max=2 leaves only "a"
	b := []byte{'a', 0xc2, 0xa9, 'b'}
	got := utf8PrefixMax(b, 2)
	require.Equal(t, []byte("a"), got)

	prefix := utf8PrefixMax([]byte("hello"), 4096)
	require.Equal(t, []byte("hello"), prefix)
}
