package components

import (
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
)

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripAnsiSeq(s string) string { return ansiRe.ReplaceAllString(s, "") }

func TestRenderWaveformProgress_PreservesGlyphs(t *testing.T) {
	samples := []byte{31, 16, 0, 8, 24, 31, 4, 12}
	width := 8
	plain := renderWaveform(samples, width)

	// The bar glyphs are unchanged regardless of playback progress.
	assert.Equal(t, plain, stripAnsiSeq(renderWaveformProgress(samples, width, 0)))
	assert.Equal(t, plain, stripAnsiSeq(renderWaveformProgress(samples, width, 0.5)))
	assert.Equal(t, plain, stripAnsiSeq(renderWaveformProgress(samples, width, 1)))
}

func TestRenderWaveformProgress_StylesPlayedPortion(t *testing.T) {
	samples := []byte{31, 16, 0, 8, 24, 31, 4, 12}
	width := 8
	// No progress -> no styling; some progress -> played styling present.
	assert.NotContains(t, renderWaveformProgress(samples, width, 0), "\x1b[")
	assert.Contains(t, renderWaveformProgress(samples, width, 0.5), "\x1b[")
}

func TestDecodeWaveform_Unpacks5BitSamples(t *testing.T) {
	// Samples [1, 2, 3] packed LSB-first: 1 | 2<<5 | 3<<10 = 0x0C41 -> LE {0x41, 0x0C}.
	got := decodeWaveform([]byte{0x41, 0x0C})
	assert.Equal(t, []byte{1, 2, 3}, got)
}

func TestDecodeWaveform_Empty(t *testing.T) {
	assert.Empty(t, decodeWaveform(nil))
}

func TestRenderWaveform_MapsAmplitudeToBlocks(t *testing.T) {
	// Min sample -> lowest block, max (31) -> full block.
	assert.Equal(t, "▁█", renderWaveform([]byte{0, 31}, 2))
}

func TestRenderWaveform_DownsamplesToWidth(t *testing.T) {
	// Four samples bucketed into two bars: avg(0,0)=low, avg(31,31)=full.
	assert.Equal(t, "▁█", renderWaveform([]byte{0, 0, 31, 31}, 2))
}

func TestRenderWaveform_EmptyIsEmpty(t *testing.T) {
	assert.Equal(t, "", renderWaveform(nil, 8))
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		secs int
		want string
	}{
		{0, "0:00"},
		{15, "0:15"},
		{75, "1:15"},
		{200, "3:20"},
		{3661, "1:01:01"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, formatDuration(tc.secs))
	}
}
