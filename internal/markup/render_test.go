package markup_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/markup"
	"github.com/sorokin-vladimir/tele/internal/store"
)

func TestRenderPlainTextUnchanged(t *testing.T) {
	assert.Equal(t, "привет", markup.Render("привет", nil))
}

func TestRenderBold(t *testing.T) {
	ents := []store.MessageEntity{{Type: "bold", Offset: 7, Length: 5}}
	assert.Equal(t, "привет **важно**", markup.Render("привет важно", ents))
}

func TestRenderLink(t *testing.T) {
	ents := []store.MessageEntity{{Type: "text_url", Offset: 0, Length: 3, URL: "https://ya.ru"}}
	assert.Equal(t, "[док](https://ya.ru)", markup.Render("док", ents))
}

func TestRenderPreWithLanguage(t *testing.T) {
	ents := []store.MessageEntity{{Type: "pre", Offset: 0, Length: 4, Language: "go"}}
	assert.Equal(t, "```go\ncode\n```", markup.Render("code", ents))
}

// Escaping is minimal: only sequences Parse would actually read back.
func TestRenderEscapesOnlyRealMarkers(t *testing.T) {
	assert.Equal(t, "hello_world", markup.Render("hello_world", nil))
	assert.Equal(t, `2\**3`, markup.Render("2**3", nil))
	assert.Equal(t, "arr[0](x)", markup.Render("arr[0](x)", nil))
}

// Nesting order at a shared boundary: the inner span must close first.
func TestRenderNestedClosesInnermostFirst(t *testing.T) {
	ents := []store.MessageEntity{
		{Type: "italic", Offset: 7, Length: 8},
		{Type: "bold", Offset: 0, Length: 15},
	}
	assert.Equal(t, "**жирный __и курсив__**", markup.Render("жирный и курсив", ents))
}

func TestRenderSkipsMentionName(t *testing.T) {
	// The composer re-resolves mentions from its pending list, so Render must
	// not invent markup for them.
	ents := []store.MessageEntity{{Type: "mention_name", Offset: 0, Length: 7, UserID: 1}}
	assert.Equal(t, "@Ivan P", markup.Render("@Ivan P", ents))
}

func TestParseRenderRoundTrip(t *testing.T) {
	cases := []string{
		"привет",
		"**важно**",
		"**жирный __и курсив__**",
		"текст `код` текст",
		"[док](https://ya.ru)",
		"```go\nfmt.Println()\n```",
		"**жирный 😀**",
		"hello_world",
		"2**3",
	}
	for _, src := range cases {
		t.Run(src, func(t *testing.T) {
			text, ents := markup.Parse(src)
			text2, ents2 := markup.Parse(markup.Render(text, ents))
			assert.Equal(t, text, text2)
			require.Equal(t, len(ents), len(ents2))
			for i := range ents {
				assert.Equal(t, ents[i], ents2[i])
			}
		})
	}
}
