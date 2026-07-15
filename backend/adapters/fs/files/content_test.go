package files

import (
	"os"
	"path/filepath"
	"testing"
	"unicode/utf8"

	"github.com/gtsteffaniak/filebrowser/backend/common/utils"
	"github.com/gtsteffaniak/filebrowser/backend/indexing/iteminfo"
	"github.com/stretchr/testify/require"
)

// Regression: an ASCII-heavy PDF passes utils.IsTextFile's byte heuristic, but
// processContent must NOT return its bytes as text content -- otherwise the
// frontend opens it in the text editor instead of the PDF viewer (#pdf-as-text).
func TestProcessContent_PDFDoesNotReturnTextContent(t *testing.T) {
	pdfBody := "%PDF-1.4\n1 0 obj\n<< /Type /Catalog /Pages 2 0 R >>\nendobj\n" +
		"2 0 obj\n<< /Type /Pages /Kids [3 0 R] /Count 1 >>\nendobj\n" +
		"trailer\n<< /Root 1 0 R >>\n%%EOF\n"
	pdfFile := filepath.Join(t.TempDir(), "doc.pdf")
	require.NoError(t, os.WriteFile(pdfFile, []byte(pdfBody), 0o644))

	// Precondition: the byte heuristic really does misclassify this PDF as text,
	// so without the type guard its raw bytes would be returned as content.
	isText, err := utils.IsTextFile(pdfFile)
	require.NoError(t, err)
	require.True(t, isText, "precondition: ASCII-heavy PDF sniffs as text")

	pdfInfo := &iteminfo.ExtendedFileInfo{
		FileInfo: iteminfo.FileInfo{ItemInfo: iteminfo.ItemInfo{
			Name: "doc.pdf",
			Type: "application/pdf",
			Size: int64(len(pdfBody)),
		}},
		RealPath: pdfFile,
	}
	processContent(pdfInfo, nil, utils.FileOptions{Content: true})
	require.Empty(t, pdfInfo.Content, "PDF must not have text content populated")

	// Control: a genuine text file still gets its content extracted.
	txtBody := "hello text"
	txtFile := filepath.Join(t.TempDir(), "notes.txt")
	require.NoError(t, os.WriteFile(txtFile, []byte(txtBody), 0o644))
	txtInfo := &iteminfo.ExtendedFileInfo{
		FileInfo: iteminfo.FileInfo{ItemInfo: iteminfo.ItemInfo{
			Name: "notes.txt",
			Type: "text/plain",
			Size: int64(len(txtBody)),
		}},
		RealPath: txtFile,
	}
	processContent(txtInfo, nil, utils.FileOptions{Content: true})
	require.Equal(t, txtBody, txtInfo.Content, "text file should still return content")
}

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
	if _, err = os.Stat(testFilePath); os.IsNotExist(err) {
		// Try from project root
		testFilePath = filepath.Join("frontend", "tests", "playwright-files", "utf8-truncated.txt")
		if _, err = os.Stat(testFilePath); os.IsNotExist(err) {
			t.Skipf("Test file not found at %s, skipping test", testFilePath)
			return
		}
	}

	// Get absolute path
	var absPath string
	absPath, err = filepath.Abs(testFilePath)
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
