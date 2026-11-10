package web

import (
	"context"
	"math/rand"
	"strings"
	"testing"
	"testing/quick"
	"time"
	"unicode"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

func legacyFilenamesSimilar(name1, name2 string) bool {
	if name1 == name2 {
		return true
	}
	if len(name1) == 0 || len(name2) == 0 {
		return false
	}
	distance := legacyLevenshtein(name1, name2)
	maxLen := max(len(name1), len(name2))
	similarity := 1.0 - float64(distance)/float64(maxLen)
	return similarity >= 0.5
}

func legacyLevenshtein(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
	}
	for i := 0; i <= len(s1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			del := matrix[i-1][j] + 1
			ins := matrix[i][j-1] + 1
			sub := matrix[i-1][j-1] + cost
			v := del
			if ins < v {
				v = ins
			}
			if sub < v {
				v = sub
			}
			matrix[i][j] = v
		}
	}
	return matrix[len(s1)][len(s2)]
}

var filenameCorpus = []string{
	"photo", "photo_1", "photo-copy", "IMG_0042", "img 0042",
	"vacation2023beach", "vacation2023_beach", "user_video", "video",
	"document-final", "document_final_v2", "a", "ab", "abc", "abcd",
	"report Q1 2024", "report-q1-2024", "my@file#name", "myfilename",
	"summer-holiday", "summer holiday", "winter-holiday",
}

func TestFilenameSimilarMatchesLegacyOnCorpus(t *testing.T) {
	names := append([]string{}, filenameCorpus...)
	for i := 0; i < 50; i++ {
		names = append(names, randomNormalizedName(5+corpusRand.Intn(40)))
	}

	var buf levenshteinRowBuf
	for _, a := range names {
		a = normalizeFilename(a + ".jpg")
		for _, b := range names {
			b = normalizeFilename(b + ".png")
			if legacyFilenamesSimilar(a, b) != filenameSimilarWithBudget(a, b, &buf) {
				t.Fatalf("mismatch %q vs %q", a, b)
			}
		}
	}
}

func TestFilenameSimilarMatchesLegacyQuick(t *testing.T) {
	var buf levenshteinRowBuf
	if err := quick.Check(func(a, b string) bool {
		a = trimQuickString(a, 48)
		b = trimQuickString(b, 48)
		return legacyFilenamesSimilar(a, b) == filenameSimilarWithBudget(a, b, &buf)
	}, nil); err != nil {
		t.Fatal(err)
	}
}

func trimQuickString(s string, max int) string {
	if len(s) > max {
		s = s[:max]
	}
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' {
			b.WriteRune(r)
		}
	}
	return strings.ToLower(strings.TrimSpace(b.String()))
}

func randomNormalizedName(n int) string {
	const alphabet = "abcdefghijklmnopqrstuvwxyz0123456789 -"
	b := make([]byte, n)
	for i := range b {
		b[i] = alphabet[corpusRand.Intn(len(alphabet))]
	}
	return string(b)
}

func TestGroupFilesByFilenameOversizedExcluded(t *testing.T) {
	ctx := context.Background()
	stats := &duplicateProcessingStats{startTime: time.Now()}
	files := make([]*iteminfo.FileInfo, 12)
	for i := range files {
		files[i] = &iteminfo.FileInfo{
			ItemInfo: iteminfo.ItemInfo{Name: "photo.jpg", Size: 100},
		}
	}
	if len(groupFilesByFilename(ctx, files, 100, stats)) != 0 {
		t.Fatal("expected no groups for oversized cluster")
	}
}

var corpusRand = rand.New(rand.NewSource(42))
