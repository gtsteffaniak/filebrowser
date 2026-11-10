package web

import (
	"context"
	"strings"
	"unicode"

	"github.com/gtsteffaniak/filebrowser/backend/pkg/indexing/iteminfo"
)

const filenameSimilarityThreshold = 0.5

type levenshteinRowBuf struct {
	prev, cur []int
}

func (b *levenshteinRowBuf) ensure(width int) {
	if cap(b.prev) < width+1 {
		b.prev = make([]int, width+1)
		b.cur = make([]int, width+1)
	} else {
		b.prev = b.prev[:width+1]
		b.cur = b.cur[:width+1]
	}
}

func maxEditDistanceForSimilarity(maxLen int) int {
	return maxLen / 2
}

func filenameSimilarWithBudget(name1, name2 string, buf *levenshteinRowBuf) bool {
	if name1 == name2 {
		return true
	}
	if len(name1) == 0 || len(name2) == 0 {
		return false
	}

	maxLen := max(len(name1), len(name2))
	maxDist := maxEditDistanceForSimilarity(maxLen)
	if abs(len(name1)-len(name2)) > maxDist {
		return false
	}

	distance := boundedLevenshtein(name1, name2, maxDist, buf)
	if distance < 0 {
		return false
	}
	similarity := 1.0 - float64(distance)/float64(maxLen)
	return similarity >= filenameSimilarityThreshold
}

func boundedLevenshtein(s1, s2 string, maxDist int, buf *levenshteinRowBuf) int {
	n, m := len(s1), len(s2)
	if n == 0 {
		if m <= maxDist {
			return m
		}
		return -1
	}
	if m == 0 {
		if n <= maxDist {
			return n
		}
		return -1
	}

	buf.ensure(m)
	prev, cur := buf.prev, buf.cur

	for j := 0; j <= m; j++ {
		prev[j] = j
	}

	for i := 1; i <= n; i++ {
		cur[0] = i
		rowMin := cur[0]
		for j := 1; j <= m; j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			del := prev[j] + 1
			ins := cur[j-1] + 1
			sub := prev[j-1] + cost
			v := del
			if ins < v {
				v = ins
			}
			if sub < v {
				v = sub
			}
			cur[j] = v
			if v < rowMin {
				rowMin = v
			}
		}
		if rowMin > maxDist {
			return -1
		}
		prev, cur = cur, prev
	}

	if prev[m] > maxDist {
		return -1
	}
	return prev[m]
}

func groupFilesByFilename(ctx context.Context, files []*iteminfo.FileInfo, expectedSize int64, stats *duplicateProcessingStats) [][]*iteminfo.FileInfo {
	if len(files) == 0 {
		return nil
	}

	normalized := make([]string, len(files))
	for i, f := range files {
		if f.Name != "" {
			normalized[i] = normalizeFilename(f.Name)
		}
	}

	var rowBuf levenshteinRowBuf
	groups := [][]*iteminfo.FileInfo{}
	used := make(map[int]bool)

	for i := 0; i < len(files); i++ {
		if ctx.Err() != nil {
			return groups
		}
		if shouldStop, _ := stats.shouldStop(); shouldStop {
			return groups
		}

		if used[i] || files[i].Name == "" {
			continue
		}

		group := []*iteminfo.FileInfo{files[i]}
		used[i] = true
		filename1 := normalized[i]
		oversized := false

		for j := i + 1; j < len(files); j++ {
			if ctx.Err() != nil {
				return groups
			}
			if shouldStop, _ := stats.shouldStop(); shouldStop {
				return groups
			}

			if used[j] || files[j].Name == "" {
				continue
			}
			if files[j].Size != expectedSize {
				continue
			}

			if filenameSimilarWithBudget(filename1, normalized[j], &rowBuf) {
				group = append(group, files[j])
				used[j] = true
				if len(group) > maxFuzzyGroupSize {
					oversized = true
					break
				}
			}
		}

		if len(group) >= 2 && !oversized {
			groups = append(groups, group)
		}
	}

	return groups
}

func normalizeFilename(filename string) string {
	filename = strings.ToLower(filename)
	if idx := strings.LastIndex(filename, "."); idx > 0 {
		filename = filename[:idx]
	}

	var result strings.Builder
	for _, r := range filename {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' {
			result.WriteRune(r)
		}
	}

	return strings.TrimSpace(result.String())
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
