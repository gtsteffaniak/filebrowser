package files

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/require"
)

func TestGetContent_UTF8Truncation(t *testing.T) {
	// Get the path to the test file
	// The test file is in frontend/tests/playwright-files/utf8-truncated.txt
	// We need to find it relative to the test execution directory
	cwd, err := os.Getwd()
	require.NoError(t, err)

	// Navigate from backend/adapters/fs/files to the project root
	// backend/adapters/fs/files -> backend/adapters/fs -> backend/adapters -> backend -> root
	testFilePath := filepath.Join(cwd, "..", "..", "..", "..", "frontend", "tests", "playwright-files", "utf8-truncated.txt")

	// Check if file exists, if not try alternative path
	if _, err := os.Stat(testFilePath); os.IsNotExist(err) {
		// Try from project root
		testFilePath = filepath.Join("frontend", "tests", "playwright-files", "utf8-truncated.txt")
		if _, err := os.Stat(testFilePath); os.IsNotExist(err) {
			t.Skipf("Test file not found at %s, skipping test", testFilePath)
			return
		}
	}

	// Get absolute path
	absPath, err := filepath.Abs(testFilePath)
	require.NoError(t, err)

	t.Run("file with UTF-8 truncation at 4096 byte boundary", func(t *testing.T) {
		// The test file is longer than 4096 bytes, with valid UTF-8 throughout.
		// However, when reading exactly 4096 bytes as a header, it cuts off in the
		// middle of a multi-byte UTF-8 sequence (e6 9c, missing the last byte 88 of '月').
		// This should trigger the truncation handling in getContent's header validation.
		// The fix should trim the incomplete sequence from the header, allowing the
		// header check to pass, and then the full file (which is valid UTF-8) should
		// pass the full file validation.
		content, err := getContent(absPath)

		// Should not return an error - the header truncation is handled, and the full file is valid
		require.NoError(t, err)

		// The content should be the full valid UTF-8 file
		require.NotEmpty(t, content, "Content should not be empty - full file is valid UTF-8")

		// Verify it contains the expected text
		require.Contains(t, content, "文件已备份", "Content should contain Chinese characters")
		require.Contains(t, content, "2024年", "Content should contain date information")
	})

	t.Run("regular UTF-8 text file", func(t *testing.T) {
		// Test with a simple text file to ensure normal files still work
		tmpFile, err := os.CreateTemp("", "test-utf8-*.txt")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()

		testText := "Hello, 世界! This is a test file with UTF-8 characters.\n"
		_, err = tmpFile.WriteString(testText)
		require.NoError(t, err)
		tmpFile.Close()

		content, err := getContent(tmpFile.Name())
		require.NoError(t, err)
		require.Equal(t, testText, content)
	})

	t.Run("file smaller than header size", func(t *testing.T) {
		// Test with a file smaller than 4096 bytes
		tmpFile, err := os.CreateTemp("", "test-small-*.txt")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()

		testText := "Small file content"
		_, err = tmpFile.WriteString(testText)
		require.NoError(t, err)
		tmpFile.Close()

		content, err := getContent(tmpFile.Name())
		require.NoError(t, err)
		require.Equal(t, testText, content)
	})

	t.Run("file with Chinese characters at boundary", func(t *testing.T) {
		// Create a file that's exactly 4094 bytes, ending with a complete Chinese character
		tmpFile, err := os.CreateTemp("", "test-chinese-*.txt")
		require.NoError(t, err)
		defer os.Remove(tmpFile.Name())
		defer tmpFile.Close()

		// Create content that's exactly 4094 bytes, ending with complete UTF-8
		baseText := "2024年 06月 17日 星期一 04:05:58 CST 文件已备份\n"
		content := ""
		for len([]byte(content)) < 4094 {
			content += baseText
		}
		// Trim to exactly 4094 bytes
		encoded := []byte(content)
		if len(encoded) > 4094 {
			encoded = encoded[:4094]
		}
		// Ensure it ends with a complete character by finding the last complete rune
		for len(encoded) > 0 {
			lastRune, _ := decodeLastRune(encoded)
			if lastRune != 0xFFFD { // RuneError
				break
			}
			encoded = encoded[:len(encoded)-1]
		}

		_, err = tmpFile.Write(encoded)
		require.NoError(t, err)
		tmpFile.Close()

		result, err := getContent(tmpFile.Name())
		require.NoError(t, err)
		require.NotEmpty(t, result)
		require.Equal(t, string(encoded), result)
	})
}

// Helper function to check if last rune is valid
func decodeLastRune(p []byte) (rune, int) {
	if len(p) == 0 {
		return 0, 0
	}
	r, size := utf8.DecodeLastRune(p)
	return r, size
}
