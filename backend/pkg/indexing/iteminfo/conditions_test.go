package iteminfo

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// Helper function to create error messages dynamically
func errorMsg(extension, expectedType string, expectedMatch bool) string {
	matchStatus := "to match"
	if !expectedMatch {
		matchStatus = "to not match"
	}
	return fmt.Sprintf("Expected %s %s type '%s'", extension, matchStatus, expectedType)
}

func TestIsMatchingType(t *testing.T) {
	// Test cases where IsMatchingType should return true
	trueTestCases := []struct {
		extension    string
		expectedType string
	}{
		{".pdf", "doc"},
		{".doc", "doc"},
		{".docx", "doc"},
		{".json", "text"},
		{".sh", "text"},
		{".zip", "archive"},
		{".rar", "archive"},
	}

	for _, tc := range trueTestCases {
		assert.True(t, IsMatchingType(tc.extension, tc.expectedType), errorMsg(tc.extension, tc.expectedType, true))
	}

	// Test cases where IsMatchingType should return false
	falseTestCases := []struct {
		extension    string
		expectedType string
	}{
		{".mp4", "doc"},
		{".mp4", "text"},
		{".mp4", "archive"},
	}

	for _, tc := range falseTestCases {
		assert.False(t, IsMatchingType(tc.extension, tc.expectedType), errorMsg(tc.extension, tc.expectedType, false))
	}
}

func TestUpdateSize(t *testing.T) {
	// Helper function for size error messages
	sizeErrorMsg := func(input string, expected, actual int) string {
		return fmt.Sprintf("Expected size for input '%s' to be %d, got %d", input, expected, actual)
	}

	// Test cases for updateSize
	testCases := []struct {
		input    string
		expected int
	}{
		{"150", 150},
		{"invalid", 100},
		{"", 100},
	}

	for _, tc := range testCases {
		actual := UpdateSize(tc.input)
		assert.Equal(t, tc.expected, actual, sizeErrorMsg(tc.input, tc.expected, actual))
	}
}

func TestIsDoc(t *testing.T) {
	// Test cases where IsMatchingType should return true for document types
	docTrueTestCases := []struct {
		extension    string
		expectedType string
	}{
		{".doc", "doc"},
		{".pdf", "doc"},
	}

	for _, tc := range docTrueTestCases {
		assert.True(t, IsMatchingType(tc.extension, tc.expectedType), errorMsg(tc.extension, tc.expectedType, true))
	}

	// Test case where IsMatchingType should return false for document types
	docFalseTestCases := []struct {
		extension    string
		expectedType string
	}{
		{".mp4", "doc"},
	}

	for _, tc := range docFalseTestCases {
		assert.False(t, IsMatchingType(tc.extension, tc.expectedType), errorMsg(tc.extension, tc.expectedType, false))
	}
}

func TestIsText(t *testing.T) {
	// Test cases where IsMatchingType should return true for text types
	textTrueTestCases := []struct {
		extension    string
		expectedType string
	}{
		{".json", "text"},
		{".sh", "text"},
	}

	for _, tc := range textTrueTestCases {
		assert.True(t, IsMatchingType(tc.extension, tc.expectedType), errorMsg(tc.extension, tc.expectedType, true))
	}

	// Test case where IsMatchingType should return false for text types
	textFalseTestCases := []struct {
		extension    string
		expectedType string
	}{
		{".mp4", "text"},
	}

	for _, tc := range textFalseTestCases {
		assert.False(t, IsMatchingType(tc.extension, tc.expectedType), errorMsg(tc.extension, tc.expectedType, false))
	}
}

func TestIsArchive(t *testing.T) {
	// Test cases where IsMatchingType should return true for archive types
	archiveTrueTestCases := []struct {
		extension    string
		expectedType string
	}{
		{".zip", "archive"},
		{".rar", "archive"},
	}

	for _, tc := range archiveTrueTestCases {
		assert.True(t, IsMatchingType(tc.extension, tc.expectedType), errorMsg(tc.extension, tc.expectedType, true))
	}

	// Test case where IsMatchingType should return false for archive types
	archiveFalseTestCases := []struct {
		extension    string
		expectedType string
	}{
		{".mp4", "archive"},
	}

	for _, tc := range archiveFalseTestCases {
		assert.False(t, IsMatchingType(tc.extension, tc.expectedType), errorMsg(tc.extension, tc.expectedType, false))
	}
}
