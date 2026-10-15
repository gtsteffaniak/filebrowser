package transcoding

import (
	"math"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	hlsExtM3UTag          = "#EXTM3U"
	hlsExtInfPrefix       = "#EXTINF:"
	hlsExtVersionPrefix   = "#EXT-X-VERSION:"
	hlsExtTargetDurPrefix = "#EXT-X-TARGETDURATION:"
	hlsExtMediaSeqPrefix  = "#EXT-X-MEDIA-SEQUENCE:"
	hlsEndListTag         = "#EXT-X-ENDLIST"
	hlsSegmentSuffix      = ".m4s"
	hlsSegmentDir         = "seg"
)

type hlsSegmentBlock struct {
	lines    []string // leading tag lines followed by the segment URI line
	name     string   // bare segment file name, e.g. 00042.m4s
	index    int      // numeric segment index parsed from name
	duration float64  // #EXTINF duration
}

type servePlaylistResult struct {
	data       []byte
	readyCount int
	totalCount int
	endList    bool
}

// buildServePlaylist normalizes a raw ffmpeg playlist in memory for serving.
//
// Bare ffmpeg URIs become seg/NNNNN.m4s, #EXT-X-TARGETDURATION and
// #EXT-X-MEDIA-SEQUENCE are recomputed for the segments actually included, and
// segments that are not yet aligned/ready are withheld so a client never
// requests a segment the server would reject with ErrNotReady.
// #EXT-X-ENDLIST is emitted only once every segment in the source playlist is
// ready. The playlist on disk is never modified, so this cannot race with
// ffmpeg rewriting ffmpeg.m3u8.
func buildServePlaylist(raw []byte, isReady func(name string) bool) ([]byte, int) {
	result := parseServePlaylist(raw, isReady)
	return result.data, result.readyCount
}

// playlistIsComplete reports whether raw is a finished playlist (ENDLIST) whose
// segments are all aligned and ready, so the generation can be reused as-is.
func playlistIsComplete(raw []byte, isReady func(name string) bool) bool {
	result := parseServePlaylist(raw, isReady)
	return result.endList && result.totalCount > 0 && result.readyCount == result.totalCount
}

func parseServePlaylist(raw []byte, isReady func(name string) bool) servePlaylistResult {
	lines := strings.Split(strings.ReplaceAll(string(raw), "\r\n", "\n"), "\n")

	var header []string
	var blocks []hlsSegmentBlock
	var pending []string
	inSegments := false
	sawEndList := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			continue
		case trimmed == hlsEndListTag:
			sawEndList = true
		case strings.HasPrefix(trimmed, "#"):
			if strings.HasPrefix(trimmed, hlsExtInfPrefix) {
				inSegments = true
				pending = append(pending, line)
				continue
			}
			if !inSegments {
				// Header tags are preserved verbatim except the ones recomputed below.
				if !strings.HasPrefix(trimmed, hlsExtTargetDurPrefix) &&
					!strings.HasPrefix(trimmed, hlsExtMediaSeqPrefix) &&
					trimmed != hlsExtM3UTag {
					header = append(header, line)
				}
				continue
			}
			pending = append(pending, line)
		default:
			name := segmentNameFromURI(trimmed)
			blocks = append(blocks, hlsSegmentBlock{
				lines:    append(append([]string{}, pending...), hlsSegmentDir+"/"+name),
				name:     name,
				index:    segmentIndex(name),
				duration: extInfDuration(pending),
			})
			pending = nil
		}
	}

	kept := make([]hlsSegmentBlock, 0, len(blocks))
	for _, block := range blocks {
		if isReady != nil && block.name != "" && !isReady(block.name) {
			continue
		}
		kept = append(kept, block)
	}

	out := make([]string, 0, len(header)+len(kept)*2+4)
	out = append(out, hlsExtM3UTag)
	out = appendHeaderWithTargetDuration(out, header, targetDuration(kept))
	if len(kept) > 0 {
		out = append(out, hlsExtMediaSeqPrefix+strconv.Itoa(kept[0].index))
		for _, block := range kept {
			out = append(out, block.lines...)
		}
	}
	if sawEndList && len(blocks) > 0 && len(kept) == len(blocks) {
		out = append(out, hlsEndListTag)
	}
	return servePlaylistResult{
		data:       []byte(strings.Join(out, "\n") + "\n"),
		readyCount: len(kept),
		totalCount: len(blocks),
		endList:    sawEndList,
	}
}

// appendHeaderWithTargetDuration keeps ffmpeg's tag order by placing the
// recomputed #EXT-X-TARGETDURATION directly after #EXT-X-VERSION.
func appendHeaderWithTargetDuration(out, header []string, target int) []string {
	if target <= 0 {
		return append(out, header...)
	}
	tag := hlsExtTargetDurPrefix + strconv.Itoa(target)
	inserted := false
	for _, line := range header {
		out = append(out, line)
		if !inserted && strings.HasPrefix(strings.TrimSpace(line), hlsExtVersionPrefix) {
			out = append(out, tag)
			inserted = true
		}
	}
	if !inserted {
		out = append(out, tag)
	}
	return out
}

func targetDuration(blocks []hlsSegmentBlock) int {
	maxDur := 0.0
	for _, block := range blocks {
		if block.duration > maxDur {
			maxDur = block.duration
		}
	}
	if maxDur <= 0 {
		return 0
	}
	target := int(math.Ceil(maxDur))
	if target < 1 {
		target = 1
	}
	return target
}

func segmentNameFromURI(uri string) string {
	trimmed := strings.TrimSpace(strings.Trim(uri, `"`))
	if trimmed == "" {
		return ""
	}
	return filepath.Base(trimmed)
}

func segmentIndex(name string) int {
	if name == "" {
		return 0
	}
	raw := strings.TrimSuffix(name, hlsSegmentSuffix)
	idx, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return idx
}

func extInfDuration(lines []string) float64 {
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, hlsExtInfPrefix) {
			continue
		}
		raw := strings.TrimPrefix(trimmed, hlsExtInfPrefix)
		if i := strings.Index(raw, ","); i >= 0 {
			raw = raw[:i]
		}
		dur, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		if err != nil {
			return 0
		}
		return dur
	}
	return 0
}
