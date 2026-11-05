package preview

import (
	"html"
	"strconv"
	"strings"
	"unicode/utf8"
)

// rtfDestinations are groups whose text is not part of the document body.
// A group whose first control word is one of these is skipped entirely.
var rtfDestinations = map[string]bool{
	"fonttbl": true, "colortbl": true, "stylesheet": true, "info": true,
	"pict": true, "header": true, "headerl": true, "headerr": true, "headerf": true,
	"footer": true, "footerl": true, "footerr": true, "footerf": true,
	"footnote": true, "object": true, "fldinst": true, "themedata": true,
	"colorschememapping": true, "listtable": true, "listoverridetable": true,
	"generator": true, "rsidtbl": true, "xmlnstbl": true, "pgdsctbl": true,
	"latentstyles": true, "datastore": true, "filetbl": true, "revtbl": true,
}

// cp1252High maps bytes 0x80-0x9F to Windows-1252 code points. Zero means undefined,
// which falls back to the Latin-1 value.
var cp1252High = [32]rune{
	0x20AC, 0, 0x201A, 0x0192, 0x201E, 0x2026, 0x2020, 0x2021,
	0x02C6, 0x2030, 0x0160, 0x2039, 0x0152, 0, 0x017D, 0,
	0, 0x2018, 0x2019, 0x201C, 0x201D, 0x2022, 0x2013, 0x2014,
	0x02DC, 0x2122, 0x0161, 0x203A, 0x0153, 0, 0x017E, 0x0178,
}

// rtfSymbols are control words that produce fixed text.
var rtfSymbols = map[string]string{
	"emdash": "\u2014", "endash": "\u2013", "lquote": "\u2018", "rquote": "\u2019",
	"ldblquote": "\u201C", "rdblquote": "\u201D", "bullet": "\u2022", "tab": "\u00a0\u00a0\u00a0\u00a0",
	"cell": "\u00a0\u00a0",
}

type rtfFormat struct {
	bold, italic, underline bool
}

type rtfGroup struct {
	format rtfFormat
	skip   bool
	// first is true until the group has produced its first token.
	first bool
}

type rtfConverter struct {
	out         strings.Builder
	cur         rtfGroup
	stack       []rtfGroup
	open        rtfFormat // formatting tags currently open in out
	uc          int       // fallback characters that follow a \uN escape
	skipN       int       // pending fallback characters to drop
	paraHasText bool
}

// RTFToHTML converts RTF to a simple HTML fragment with paragraphs, line breaks, and
// bold/italic/underline. Output is NOT sanitized; the frontend must sanitize it.
// It tolerates truncated input (unclosed groups are fine).
func RTFToHTML(src string) string {
	c := &rtfConverter{uc: 1}
	c.out.WriteString("<p>")
	for i := 0; i < len(src); {
		switch src[i] {
		case '{':
			c.cur.first = false
			c.stack = append(c.stack, c.cur)
			c.cur.first = true
			i++
		case '}':
			if n := len(c.stack); n > 0 {
				c.cur = c.stack[n-1]
				c.stack = c.stack[:n-1]
			}
			i++
		case '\\':
			i = c.escape(src, i+1)
		case '\r', '\n':
			// Raw line breaks are not significant in RTF.
			i++
		default:
			j := i
			for j < len(src) && !strings.ContainsRune("{}\\\r\n", rune(src[j])) {
				j++
			}
			c.text(src[i:j])
			i = j
		}
	}
	c.closeFormat()
	c.out.WriteString("</p>")
	return c.out.String()
}

// escape handles the text after a backslash. i points at the first character after it.
// It returns the index of the next unread character.
func (c *rtfConverter) escape(src string, i int) int {
	if i >= len(src) {
		return i
	}
	ch := src[i]
	switch {
	case ch == '\'':
		end := min(i+3, len(src))
		if v, err := strconv.ParseUint(src[i+1:end], 16, 8); err == nil {
			c.hexByte(byte(v))
		}
		return end
	case isLetter(ch):
		j := i
		for j < len(src) && isLetter(src[j]) {
			j++
		}
		name := src[i:j]
		k := j
		param, hasParam := 0, false
		if k < len(src) && (src[k] == '-' || isDigit(src[k])) {
			start := k
			if src[k] == '-' {
				k++
			}
			for k < len(src) && isDigit(src[k]) {
				k++
			}
			if p, err := strconv.Atoi(src[start:k]); err == nil {
				param, hasParam = p, true
			}
		}
		if k < len(src) && src[k] == ' ' {
			k++ // a single space after a control word is its delimiter
		}
		c.controlWord(name, param, hasParam)
		return k
	case ch == '*':
		// \* marks an ignorable destination; skip the group if it is the first token.
		if c.cur.first {
			c.cur.skip = true
		}
		c.cur.first = false
		return i + 1
	case ch == '\\' || ch == '{' || ch == '}':
		c.text(string(ch))
		return i + 1
	case ch == '~':
		c.text("\u00a0")
		return i + 1
	case ch == '_':
		c.text("-")
		return i + 1
	case ch == '\r' || ch == '\n':
		// A backslash before a line break is \par.
		c.paragraph()
		if ch == '\r' && i+1 < len(src) && src[i+1] == '\n' {
			return i + 2
		}
		return i + 1
	default:
		// Other control symbols (\-, \|, \:, ...) have no visible output.
		c.cur.first = false
		return i + 1
	}
}

func (c *rtfConverter) controlWord(name string, param int, hasParam bool) {
	if c.cur.first && rtfDestinations[name] {
		c.cur.skip = true
	}
	c.cur.first = false
	if c.cur.skip {
		return
	}
	if sym, ok := rtfSymbols[name]; ok {
		c.emit(sym)
		return
	}
	on := !hasParam || param != 0
	switch name {
	case "par", "sect":
		c.paragraph()
	case "line":
		c.line()
	case "plain":
		c.cur.format = rtfFormat{}
	case "b":
		c.cur.format.bold = on
	case "i":
		c.cur.format.italic = on
	case "ul":
		c.cur.format.underline = on
	case "ulnone":
		c.cur.format.underline = false
	case "uc":
		if hasParam && param >= 0 {
			c.uc = param
		}
	case "u":
		// \uN is a signed 16-bit code point; the following uc characters are its fallback.
		r := param
		if r < 0 {
			r += 65536
		}
		c.emit(string(rune(r)))
		c.skipN = c.uc
	}
}

// text emits literal text, dropping any pending \uN fallback characters.
func (c *rtfConverter) text(s string) {
	c.cur.first = false
	if c.cur.skip || s == "" {
		return
	}
	for c.skipN > 0 && s != "" {
		_, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		c.skipN--
	}
	if s != "" {
		c.emit(s)
	}
}

// hexByte emits a \'hh byte decoded as Windows-1252.
func (c *rtfConverter) hexByte(b byte) {
	c.cur.first = false
	if c.cur.skip {
		return
	}
	if c.skipN > 0 {
		c.skipN--
		return
	}
	r := rune(b)
	if b >= 0x80 && b < 0xA0 && cp1252High[b-0x80] != 0 {
		r = cp1252High[b-0x80]
	}
	c.emit(string(r))
}

func (c *rtfConverter) emit(s string) {
	c.syncFormat()
	c.out.WriteString(html.EscapeString(s))
	if strings.TrimSpace(s) != "" {
		c.paraHasText = true
	}
}

func (c *rtfConverter) paragraph() {
	if c.cur.skip {
		return
	}
	c.closeFormat()
	if !c.paraHasText {
		c.out.WriteString("<br>")
	}
	c.out.WriteString("</p><p>")
	c.paraHasText = false
}

func (c *rtfConverter) line() {
	if c.cur.skip {
		return
	}
	c.out.WriteString("<br>")
	c.paraHasText = true
}

// syncFormat opens and closes tags so the open formatting matches the current group.
func (c *rtfConverter) syncFormat() {
	c.setFormat(c.cur.format)
}

func (c *rtfConverter) closeFormat() {
	c.setFormat(rtfFormat{})
}

// setFormat closes all open formatting tags and opens the wanted ones. Closing everything
// first keeps the tags properly nested.
func (c *rtfConverter) setFormat(want rtfFormat) {
	if want == c.open {
		return
	}
	if c.open.underline {
		c.out.WriteString("</u>")
	}
	if c.open.italic {
		c.out.WriteString("</em>")
	}
	if c.open.bold {
		c.out.WriteString("</strong>")
	}
	if want.bold {
		c.out.WriteString("<strong>")
	}
	if want.italic {
		c.out.WriteString("<em>")
	}
	if want.underline {
		c.out.WriteString("<u>")
	}
	c.open = want
}

func isLetter(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z')
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}
