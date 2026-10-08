// Package archiveencoding decodes ZIP entry names before path normalization.
package archiveencoding

import (
	"archive/zip"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/charmap"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/encoding/korean"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/encoding/traditionalchinese"
)

// ErrEncoding indicates a filename encoding that needs a different user selection.
var ErrEncoding = errors.New("invalid ZIP filename encoding")

type codec struct {
	name     string
	encoding encoding.Encoding
}

var codecs = []codec{
	{"utf-8", nil},
	{"cp932", japanese.ShiftJIS},
	{"gb18030", simplifiedchinese.GB18030},
	{"big5", traditionalchinese.Big5},
	{"euc-kr", korean.EUCKR},
	{"cp437", charmap.CodePage437},
	{"windows-1252", charmap.Windows1252},
}

// Candidate contains a bounded preview, decoded exactly as during extraction.
type Candidate struct {
	Encoding string   `json:"encoding"`
	Names    []string `json:"names"`
}

// Preview describes compatible encodings. An empty suggestion requires a choice;
// a heuristic suggestion is never a guarantee that the names are correct.
type Preview struct {
	Suggested  string      `json:"suggested"`
	Candidates []Candidate `json:"candidates"`
}

// unicodeName prefers the UTF-8 flag, then a verified Info-ZIP Unicode Path field.
// A Unicode Path field is usable only when its version and original-name CRC match.
func unicodeName(f *zip.File) (string, bool) {
	if f.Flags&0x800 != 0 {
		return f.Name, true
	}
	extra := f.Extra
	for len(extra) >= 4 {
		tag := binary.LittleEndian.Uint16(extra)
		size := int(binary.LittleEndian.Uint16(extra[2:]))
		extra = extra[4:]
		if size > len(extra) {
			break
		}
		field := extra[:size]
		extra = extra[size:]
		if tag == 0x7075 && len(field) > 5 && field[0] == 1 &&
			binary.LittleEndian.Uint32(field[1:5]) == crc32.ChecksumIEEE([]byte(f.Name)) && utf8.Valid(field[5:]) {
			return string(field[5:]), true
		}
	}
	return "", false
}

func decode(f *zip.File, c codec) (string, error) {
	name, declared := unicodeName(f)
	if !declared {
		name = f.Name
		if c.encoding != nil {
			var err error
			name, err = c.encoding.NewDecoder().String(name)
			if err != nil || strings.ContainsRune(name, utf8.RuneError) {
				return "", fmt.Errorf("%w: cannot decode filename as %s", ErrEncoding, c.name)
			}
		}
	}
	if !utf8.ValidString(name) || strings.ContainsRune(name, 0) {
		return "", fmt.Errorf("%w: invalid filename for %s", ErrEncoding, c.name)
	}
	return name, nil
}

// Inspect considers all entry names without decompressing file contents. Candidate
// previews prefer names lacking Unicode metadata so that users can compare them.
func Inspect(files []*zip.File) Preview {
	result := Preview{Candidates: []Candidate{}}
	bestScore, tied := 0, false
	for _, c := range codecs {
		candidate := Candidate{Encoding: c.name, Names: []string{}}
		fallback := []string{}
		score, valid := 0, true
		for _, f := range files {
			name, err := decode(f, c)
			if err != nil {
				valid = false
				break
			}
			if len(fallback) < 5 {
				fallback = append(fallback, name)
			}
			if _, declared := unicodeName(f); !declared && !isASCII(f.Name) {
				if len(candidate.Names) < 5 {
					candidate.Names = append(candidate.Names, name)
				}
				// The EUC-KR decoder also accepts CP949 extension bytes, including
				// CP932 kana pairs. Do not use those bytes as Korean evidence.
				if c.name != "euc-kr" || isEUCKR(f.Name) {
					score += scriptEvidence(name, c.name)
				}
			}
		}
		if !valid {
			continue
		}
		if len(candidate.Names) == 0 {
			candidate.Names = fallback
		}
		result.Candidates = append(result.Candidates, candidate)
		// Valid UTF-8 (including metadata-backed names) needs no legacy guess.
		if c.name == "utf-8" {
			result.Suggested = c.name
		}
		if score > bestScore {
			bestScore, tied = score, false
			if result.Suggested != "utf-8" {
				result.Suggested = c.name
			}
		} else if score == bestScore && score > 0 {
			tied = true
		}
	}
	if result.Suggested != "utf-8" && (bestScore < 3 || tied) {
		result.Suggested = ""
	}
	return result
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= utf8.RuneSelf {
			return false
		}
	}
	return true
}

// isEUCKR excludes CP949 extension sequences from automatic Korean suggestions.
func isEUCKR(raw string) bool {
	for i := 0; i < len(raw); i++ {
		if raw[i] < utf8.RuneSelf {
			continue
		}
		if raw[i] < 0xa1 || raw[i] > 0xfe || i+1 >= len(raw) || raw[i+1] < 0xa1 || raw[i+1] > 0xfe {
			return false
		}
		i++
	}
	return true
}

// Kana and Hangul give useful script evidence; Han alone does not distinguish
// Japanese, simplified Chinese and traditional Chinese. Half-width kana are not
// evidence: unrelated legacy bytes frequently decode to half-width kana.
func scriptEvidence(name, charset string) int {
	score := 0
	for _, r := range name {
		if charset == "cp932" && r < 0xff00 && (unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r)) {
			score++
		}
		if charset == "euc-kr" && unicode.Is(unicode.Hangul, r) {
			score++
		}
	}
	return score
}

// DecodeNames resolves the selection once for the entire archive. Explicit
// Unicode metadata always wins over the selected fallback encoding.
func DecodeNames(files []*zip.File, charset string) ([]string, error) {
	if charset == "" {
		charset = Inspect(files).Suggested
		if charset == "" {
			return nil, fmt.Errorf("%w: choose an encoding before extracting", ErrEncoding)
		}
	}
	for _, c := range codecs {
		if c.name != charset {
			continue
		}
		names := make([]string, 0, len(files))
		for _, f := range files {
			name, err := decode(f, c)
			if err != nil {
				return nil, err
			}
			names = append(names, name)
		}
		return names, nil
	}
	return nil, fmt.Errorf("%w: unsupported encoding %s", ErrEncoding, charset)
}
