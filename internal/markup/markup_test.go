package markup_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/markup"
)

func TestUTF16LenCountsSurrogatePairsAsTwo(t *testing.T) {
	assert.Equal(t, 3, markup.UTF16Len("абв"))
	assert.Equal(t, 2, markup.UTF16Len("😀"))
	assert.Equal(t, 3, markup.UTF16Len("a😀"))
}

func TestUTF16ToRuneIndexSkipsSurrogatePairs(t *testing.T) {
	// "😀" is one rune but two UTF-16 code units, so offset 2 is rune 1.
	assert.Equal(t, 1, markup.UTF16ToRuneIndex("😀x", 2))
	assert.Equal(t, 2, markup.UTF16ToRuneIndex("😀x", 3))
}

func TestParsePlainTextProducesNoEntities(t *testing.T) {
	text, ents := markup.Parse("привет, мир")
	assert.Equal(t, "привет, мир", text)
	assert.Empty(t, ents)
}

func TestParseEscapedMarkersAreLiteral(t *testing.T) {
	text, ents := markup.Parse(`2\*\*3\*\*4`)
	assert.Equal(t, "2**3**4", text)
	assert.Empty(t, ents)
}

func TestParseBackslashBeforeNonMarkerStaysLiteral(t *testing.T) {
	text, ents := markup.Parse(`C:\path\to`)
	assert.Equal(t, `C:\path\to`, text)
	assert.Empty(t, ents)
}

func TestParseCodeSpan(t *testing.T) {
	text, ents := markup.Parse("вот `x := 1` тут")
	assert.Equal(t, "вот x := 1 тут", text)
	require.Len(t, ents, 1)
	assert.Equal(t, "code", ents[0].Type)
	assert.Equal(t, 4, ents[0].Offset)
	assert.Equal(t, 6, ents[0].Length)
}

func TestParseCodeSpanIsLeaf(t *testing.T) {
	text, ents := markup.Parse("`2**3**4`")
	assert.Equal(t, "2**3**4", text)
	require.Len(t, ents, 1)
	assert.Equal(t, "code", ents[0].Type)
}

func TestParseCodeSpanDoesNotProcessEscapes(t *testing.T) {
	text, _ := markup.Parse(`\*` + "`" + `\*` + "`")
	assert.Equal(t, `*\*`, text)
}

func TestParseUnclosedCodeSpanIsLiteral(t *testing.T) {
	text, ents := markup.Parse("`oops")
	assert.Equal(t, "`oops", text)
	assert.Empty(t, ents)
}

func TestParseFencedPreWithLanguage(t *testing.T) {
	text, ents := markup.Parse("```go\nfmt.Println()\n```")
	assert.Equal(t, "fmt.Println()", text)
	require.Len(t, ents, 1)
	assert.Equal(t, "pre", ents[0].Type)
	assert.Equal(t, "go", ents[0].Language)
	assert.Equal(t, 0, ents[0].Offset)
	assert.Equal(t, 13, ents[0].Length)
}

func TestParseFencedPreWithoutLanguage(t *testing.T) {
	_, ents := markup.Parse("```\ncode\n```")
	require.Len(t, ents, 1)
	assert.Equal(t, "pre", ents[0].Type)
	assert.Empty(t, ents[0].Language)
}

func TestParseTextAroundFence(t *testing.T) {
	text, ents := markup.Parse("до\n```\ncode\n```\nпосле")
	assert.Equal(t, "до\ncode\nпосле", text)
	require.Len(t, ents, 1)
	assert.Equal(t, 3, ents[0].Offset)
	assert.Equal(t, 4, ents[0].Length)
}

func TestParseUnclosedFenceIsLiteral(t *testing.T) {
	text, ents := markup.Parse("```go\nfmt")
	assert.Equal(t, "```go\nfmt", text)
	assert.Empty(t, ents)
}

func TestParseAttributeMarkers(t *testing.T) {
	cases := []struct{ src, want, typ string }{
		{"**важно**", "важно", "bold"},
		{"__курсив__", "курсив", "italic"},
		{"~~зачёркнуто~~", "зачёркнуто", "strike"},
		{"++подчёркнуто++", "подчёркнуто", "underline"},
	}
	for _, c := range cases {
		t.Run(c.typ, func(t *testing.T) {
			text, ents := markup.Parse(c.src)
			assert.Equal(t, c.want, text)
			require.Len(t, ents, 1)
			assert.Equal(t, c.typ, ents[0].Type)
			assert.Equal(t, 0, ents[0].Offset)
			assert.Equal(t, markup.UTF16Len(c.want), ents[0].Length)
		})
	}
}

func TestParseNestedMarkersOverlap(t *testing.T) {
	text, ents := markup.Parse("**жирный __и курсив__**")
	assert.Equal(t, "жирный и курсив", text)
	require.Len(t, ents, 2)
	// Inner closes first, so italic is recorded before bold.
	assert.Equal(t, "italic", ents[0].Type)
	assert.Equal(t, 7, ents[0].Offset)
	assert.Equal(t, 8, ents[0].Length)
	assert.Equal(t, "bold", ents[1].Type)
	assert.Equal(t, 0, ents[1].Offset)
	assert.Equal(t, 15, ents[1].Length)
}

func TestParseFlankingLeavesIncrementOperators(t *testing.T) {
	text, ents := markup.Parse("i++ and ++j")
	assert.Equal(t, "i++ and ++j", text)
	assert.Empty(t, ents)
}

func TestParseFlankingLeavesSpacedMarkers(t *testing.T) {
	text, ents := markup.Parse("a ** b")
	assert.Equal(t, "a ** b", text)
	assert.Empty(t, ents)
}

// The documented cost of the Telegram Desktop dialect: Python's power operator
// needs escaping or a code span. Pinned so the behaviour is deliberate.
func TestParsePowerOperatorBecomesBold(t *testing.T) {
	text, ents := markup.Parse("2**3**4")
	assert.Equal(t, "234", text)
	require.Len(t, ents, 1)
	assert.Equal(t, "bold", ents[0].Type)
	assert.Equal(t, 1, ents[0].Offset)
	assert.Equal(t, 1, ents[0].Length)
}

func TestParseUnclosedMarkerIsLiteral(t *testing.T) {
	text, ents := markup.Parse("**bold")
	assert.Equal(t, "**bold", text)
	assert.Empty(t, ents)
}

func TestParseMarkerInsideCodeSpanCannotClose(t *testing.T) {
	text, ents := markup.Parse("**a `b**c` d**")
	assert.Equal(t, "a b**c d", text)
	require.Len(t, ents, 2)
	assert.Equal(t, "code", ents[0].Type)
	assert.Equal(t, "bold", ents[1].Type)
	assert.Equal(t, 0, ents[1].Offset)
	assert.Equal(t, 8, ents[1].Length)
}

func TestParseEscapedMarkerCannotClose(t *testing.T) {
	text, ents := markup.Parse(`**a\**b**`)
	assert.Equal(t, "a**b", text)
	require.Len(t, ents, 1)
	assert.Equal(t, "bold", ents[0].Type)
	assert.Equal(t, 4, ents[0].Length)
}

func TestParseBoldWithEmojiOffsets(t *testing.T) {
	text, ents := markup.Parse("**жирный 😀** хвост")
	assert.Equal(t, "жирный 😀 хвост", text)
	require.Len(t, ents, 1)
	assert.Equal(t, 0, ents[0].Offset)
	// 6 letters + space + surrogate pair = 9 UTF-16 code units.
	assert.Equal(t, 9, ents[0].Length)
}

func TestParseLink(t *testing.T) {
	text, ents := markup.Parse("см. [док](https://ya.ru) тут")
	assert.Equal(t, "см. док тут", text)
	require.Len(t, ents, 1)
	assert.Equal(t, "text_url", ents[0].Type)
	assert.Equal(t, "https://ya.ru", ents[0].URL)
	assert.Equal(t, 4, ents[0].Offset)
	assert.Equal(t, 3, ents[0].Length)
}

func TestParseLinkSchemelessTargetGetsHTTPS(t *testing.T) {
	_, ents := markup.Parse("[док](example.com)")
	require.Len(t, ents, 1)
	assert.Equal(t, "https://example.com", ents[0].URL)
}

func TestParseLinkMailtoTarget(t *testing.T) {
	_, ents := markup.Parse("[почта](mailto:a@b.com)")
	require.Len(t, ents, 1)
	assert.Equal(t, "mailto:a@b.com", ents[0].URL)
}

// Bracket-paren sequences are ordinary code, not links.
func TestParseLinkRejectsNonURLTarget(t *testing.T) {
	text, ents := markup.Parse("arr[0](x)")
	assert.Equal(t, "arr[0](x)", text)
	assert.Empty(t, ents)
}

func TestParseLinkTextIsParsed(t *testing.T) {
	text, ents := markup.Parse("[**жирная** ссылка](https://ya.ru)")
	assert.Equal(t, "жирная ссылка", text)
	require.Len(t, ents, 2)
	assert.Equal(t, "bold", ents[0].Type)
	assert.Equal(t, "text_url", ents[1].Type)
	assert.Equal(t, 13, ents[1].Length)
}
