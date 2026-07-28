package media

import (
	"testing"

	"github.com/sorokin-vladimir/tele/internal/domain"
	"github.com/stretchr/testify/assert"
)

func TestNormalizeMIME(t *testing.T) {
	assert.Equal(t, "image/jpeg", NormalizeMIME("Image/JPEG"))
	assert.Equal(t, "text/plain", NormalizeMIME("text/plain; charset=utf-8"))
	assert.Equal(t, "image/png", NormalizeMIME("  image/png  "))
	assert.Equal(t, "", NormalizeMIME(""))
}

func TestDefaultMediaType(t *testing.T) {
	cases := []struct {
		mime string
		want domain.MediaKind
	}{
		{"image/jpeg", domain.MediaPhoto},
		{"image/png", domain.MediaPhoto},
		{"video/mp4", domain.MediaVideo},
		{"video/quicktime", domain.MediaVideo},
		{"audio/ogg", domain.MediaVoice},
		{"audio/mpeg", domain.MediaAudio},
		{"application/pdf", domain.MediaFile},
		{"", domain.MediaFile},
		{"IMAGE/JPEG; foo=bar", domain.MediaPhoto},
	}
	for _, c := range cases {
		assert.Equalf(t, c.want, DefaultMediaType(c.mime), "mime=%q", c.mime)
	}
}
