// Package markup converts between the composer's markdown-ish source text and
// Telegram message entities. It owns the input grammar; the composer owns draft
// state. Offsets and lengths are always UTF-16 code units, the unit Telegram
// measures entities in.
package markup

import (
	"sort"
	"strings"
	"unicode"

	"github.com/sorokin-vladimir/tele/internal/store"
)

// escapable lists the characters a backslash may escape. A backslash before any
// other character stays literal, so Windows paths and regexes survive untouched.
const escapable = "*_~+[]()`\\"

// UTF16Len returns the number of UTF-16 code units in s.
func UTF16Len(s string) int {
	n := 0
	for _, r := range s {
		if r >= 0x10000 {
			n += 2
		} else {
			n++
		}
	}
	return n
}

// UTF16ToRuneIndex converts a UTF-16 code unit offset to a rune index in s.
func UTF16ToRuneIndex(s string, utf16Offset int) int {
	runeIdx, u16Pos := 0, 0
	for _, r := range s {
		if u16Pos >= utf16Offset {
			break
		}
		if r >= 0x10000 {
			u16Pos += 2
		} else {
			u16Pos++
		}
		runeIdx++
	}
	return runeIdx
}

// builder accumulates plain text while tracking its UTF-16 length, so entity
// offsets are correct by construction instead of back-patched.
type builder struct {
	sb   strings.Builder
	u16  int
	ents []store.MessageEntity
}

func (b *builder) writeString(s string) {
	b.sb.WriteString(s)
	b.u16 += UTF16Len(s)
}

// Parse converts markdown-ish source into plain text plus Telegram entities.
// It is total: malformed markup degrades to literal text and never errors,
// because a chat composer must not refuse to send over syntax.
func Parse(src string) (string, []store.MessageEntity) {
	b := &builder{}
	parseBlocks(b, src)
	return b.sb.String(), b.ents
}

// parseBlocks peels fenced code blocks off the source. A fence is a line
// starting with ```; everything up to the closing fence is the literal content
// of a pre entity. Every other line goes to the inline parser one line at a
// time, which is what keeps inline markers from spanning newlines.
func parseBlocks(b *builder, src string) {
	lines := strings.Split(src, "\n")
	first := true
	sep := func() {
		if !first {
			b.writeString("\n")
		}
		first = false
	}
	for i := 0; i < len(lines); {
		if strings.HasPrefix(lines[i], "```") {
			if close := findFenceClose(lines, i); close >= 0 {
				sep()
				start := b.u16
				b.writeString(strings.Join(lines[i+1:close], "\n"))
				b.ents = append(b.ents, store.MessageEntity{
					Type:     "pre",
					Offset:   start,
					Length:   b.u16 - start,
					Language: strings.TrimSpace(strings.TrimPrefix(lines[i], "```")),
				})
				i = close + 1
				continue
			}
		}
		sep()
		parseInline(b, []rune(lines[i]))
		i++
	}
}

// findFenceClose returns the index of the line closing the fence opened at
// open, or -1. An unclosed fence is literal, like any unclosed marker.
func findFenceClose(lines []string, open int) int {
	for j := open + 1; j < len(lines); j++ {
		if strings.HasPrefix(lines[j], "```") {
			return j
		}
	}
	return -1
}

// findCodeSpanEnd returns the index of the backtick closing the span opened at
// start, or -1. Content is literal: no escapes, no nested markup.
func findCodeSpanEnd(rs []rune, start int) int {
	for j := start + 1; j < len(rs); j++ {
		if rs[j] == '`' {
			return j
		}
	}
	return -1
}

type marker struct {
	tok string
	typ string
}

var markers = []marker{
	{"**", "bold"},
	{"__", "italic"},
	{"~~", "strike"},
	{"++", "underline"},
}

// markerAt reports the marker opening at i. An opener must be followed by a
// non-space, which is what leaves "i++ and ++j" and "a ** b" alone.
func markerAt(rs []rune, i int) (marker, bool) {
	for _, m := range markers {
		t := []rune(m.tok)
		if i+len(t) >= len(rs) {
			continue
		}
		if string(rs[i:i+len(t)]) != m.tok {
			continue
		}
		if unicode.IsSpace(rs[i+len(t)]) {
			continue
		}
		return m, true
	}
	return marker{}, false
}

// findCloser returns the index of the token closing a span that opened before
// from, or -1. Escaped characters and code spans are skipped, so neither can
// close an outer span. A closer needs a non-space to its left, and content must
// be non-empty.
func findCloser(rs []rune, from int, tok string) int {
	t := []rune(tok)
	for j := from; j+len(t) <= len(rs); {
		if rs[j] == '\\' {
			j += 2
			continue
		}
		if rs[j] == '`' {
			if end := findCodeSpanEnd(rs, j); end > j+1 {
				j = end + 1
				continue
			}
		}
		if string(rs[j:j+len(t)]) == tok && j > from && !unicode.IsSpace(rs[j-1]) {
			return j
		}
		j++
	}
	return -1
}

// linkAt parses "[text](target)" at i, returning the visible text, the
// normalized target, and the index just past the link.
func linkAt(rs []rune, i int) ([]rune, string, int, bool) {
	closeBracket := -1
	for j := i + 1; j < len(rs); j++ {
		if rs[j] == '\\' {
			j++
			continue
		}
		if rs[j] == ']' {
			closeBracket = j
			break
		}
	}
	if closeBracket < 0 || closeBracket == i+1 {
		return nil, "", 0, false
	}
	if closeBracket+1 >= len(rs) || rs[closeBracket+1] != '(' {
		return nil, "", 0, false
	}
	closeParen := -1
	for j := closeBracket + 2; j < len(rs); j++ {
		if rs[j] == ')' {
			closeParen = j
			break
		}
	}
	if closeParen < 0 || closeParen == closeBracket+2 {
		return nil, "", 0, false
	}
	target, ok := normalizeTarget(string(rs[closeBracket+2 : closeParen]))
	if !ok {
		return nil, "", 0, false
	}
	return rs[i+1 : closeBracket], target, closeParen + 1, true
}

// normalizeTarget accepts only URL-shaped targets — a scheme, or a dotted
// domain — so "arr[0](x)" in a dev chat does not silently become a link.
// Scheme-less targets get https://, matching the receive-side
// normalizeLinkTarget in internal/ui/components/render.go.
func normalizeTarget(raw string) (string, bool) {
	s := strings.TrimSpace(raw)
	switch {
	case s == "":
		return "", false
	case strings.Contains(s, "://"), strings.HasPrefix(s, "mailto:"):
		return s, true
	case strings.Contains(s, ".") && !strings.HasPrefix(s, ".") && !strings.HasSuffix(s, "."):
		return "https://" + s, true
	}
	return "", false
}

func parseInline(b *builder, rs []rune) {
	for i := 0; i < len(rs); {
		if rs[i] == '`' {
			if end := findCodeSpanEnd(rs, i); end > i+1 {
				start := b.u16
				b.writeString(string(rs[i+1 : end]))
				b.ents = append(b.ents, store.MessageEntity{
					Type: "code", Offset: start, Length: b.u16 - start,
				})
				i = end + 1
				continue
			}
		}
		if rs[i] == '\\' && i+1 < len(rs) && strings.ContainsRune(escapable, rs[i+1]) {
			b.writeString(string(rs[i+1]))
			i += 2
			continue
		}
		if m, ok := markerAt(rs, i); ok {
			if end := findCloser(rs, i+len(m.tok), m.tok); end >= 0 {
				start := b.u16
				parseInline(b, rs[i+len(m.tok):end])
				b.ents = append(b.ents, store.MessageEntity{
					Type: m.typ, Offset: start, Length: b.u16 - start,
				})
				i = end + len(m.tok)
				continue
			}
		}
		if rs[i] == '[' {
			if linkText, target, next, ok := linkAt(rs, i); ok {
				start := b.u16
				parseInline(b, linkText)
				b.ents = append(b.ents, store.MessageEntity{
					Type: "text_url", Offset: start, Length: b.u16 - start, URL: target,
				})
				i = next
				continue
			}
		}
		b.writeString(string(rs[i]))
		i++
	}
}

type renderSpan struct {
	start, end     int // rune indices
	typ, url, lang string
}

// Two shapes cannot round-trip and are accepted as-is: code content containing
// a backtick (no fence-length negotiation), and a pre entity that starts
// mid-line (Parse only ever produces line-aligned fences).
//
// Render re-inserts markers so an existing message can be edited as source. It
// is the inverse of Parse. mention_name is skipped: the composer re-resolves
// mentions from its pending list, so inventing markup for them would double up.
// Server-detected types (url, email, hashtag, …) are skipped for the same
// reason — they come back on their own.
func Render(text string, entities []store.MessageEntity) string {
	runes := []rune(text)
	n := len(runes)

	var spans []renderSpan
	for _, e := range entities {
		if !renderable(e.Type) {
			continue
		}
		start := UTF16ToRuneIndex(text, e.Offset)
		end := UTF16ToRuneIndex(text, e.Offset+e.Length)
		if start >= n || start >= end {
			continue
		}
		if end > n {
			end = n
		}
		spans = append(spans, renderSpan{start, end, e.Type, e.URL, e.Language})
	}

	opens := make([][]renderSpan, n+1)
	closes := make([][]renderSpan, n+1)
	leaves := make([]*renderSpan, n+1)
	for _, s := range spans {
		if s.typ == "code" || s.typ == "pre" {
			cp := s
			leaves[s.start] = &cp
			continue
		}
		opens[s.start] = append(opens[s.start], s)
		closes[s.end] = append(closes[s.end], s)
	}
	// At a shared boundary the outermost span must open first and the innermost
	// must close first. Without this, "**жирный __и курсив__**" renders as
	// "**жирный __и курсив**__", where the outer bold closes before the inner
	// italic and the round-trip breaks. Entity order from the server is not a
	// reliable proxy for nesting, so derive it from the spans themselves.
	for i := range opens {
		sort.SliceStable(opens[i], func(a, b int) bool { return opens[i][a].end > opens[i][b].end })
	}
	for i := range closes {
		sort.SliceStable(closes[i], func(a, b int) bool { return closes[i][a].start > closes[i][b].start })
	}

	var sb strings.Builder
	for i := 0; i <= n; i++ {
		for _, s := range closes[i] {
			sb.WriteString(closeTok(s))
		}
		if i == n {
			break
		}
		for _, s := range opens[i] {
			sb.WriteString(openTok(s))
		}
		if s := leaves[i]; s != nil {
			sb.WriteString(leafSource(*s, runes))
			i = s.end - 1 // the loop's i++ lands on s.end
			continue
		}
		sb.WriteString(escapeAt(runes, i))
	}
	return sb.String()
}

func renderable(typ string) bool {
	switch typ {
	case "bold", "italic", "strike", "underline", "code", "pre", "text_url":
		return true
	}
	return false
}

func openTok(s renderSpan) string {
	if s.typ == "text_url" {
		return "["
	}
	return tokFor(s.typ)
}

func closeTok(s renderSpan) string {
	if s.typ == "text_url" {
		return "](" + s.url + ")"
	}
	return tokFor(s.typ)
}

func tokFor(typ string) string {
	for _, m := range markers {
		if m.typ == typ {
			return m.tok
		}
	}
	return ""
}

// leafSource emits a code span or a fenced block. Content is written raw:
// Parse does not read markup or escapes inside code.
func leafSource(s renderSpan, runes []rune) string {
	content := string(runes[s.start:s.end])
	if s.typ == "code" {
		return "`" + content + "`"
	}
	return "```" + s.lang + "\n" + content + "\n```"
}

// escapeAt returns the source for runes[i], escaping only where leaving the
// character bare would change how Parse reads it back. Escaping every marker
// character would turn "hello_world" into "hello\_world" in the composer.
func escapeAt(runes []rune, i int) string {
	r := runes[i]
	switch {
	case r == '\\':
		return `\\`
	case r == '`':
		return "\\`"
	case strings.ContainsRune("*_~+", r):
		if i+1 < len(runes) && runes[i+1] == r {
			return "\\" + string(r)
		}
	case r == '[':
		if _, _, _, ok := linkAt(runes, i); ok {
			return `\[`
		}
	}
	return string(r)
}
