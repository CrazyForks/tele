package audio

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// makeOggPage builds a single Ogg page with the given lacing/segment table and
// body. CRC and granule are left zero (the demuxer does not validate them).
func makeOggPage(segTable []byte, body []byte) []byte {
	page := []byte("OggS")
	page = append(page, 0)                   // version
	page = append(page, 0)                   // header type
	page = append(page, make([]byte, 8)...)  // granule position
	page = append(page, make([]byte, 4)...)  // bitstream serial
	page = append(page, make([]byte, 4)...)  // page sequence
	page = append(page, make([]byte, 4)...)  // crc
	page = append(page, byte(len(segTable))) // page segments
	page = append(page, segTable...)
	page = append(page, body...)
	return page
}

func TestOggOpusPackets_SplitsByLacing(t *testing.T) {
	// One page, two packets of 3 and 5 bytes.
	body := []byte{1, 2, 3, 4, 5, 6, 7, 8}
	page := makeOggPage([]byte{3, 5}, body)

	packets := oggPackets(page)
	require.Len(t, packets, 2)
	assert.Equal(t, []byte{1, 2, 3}, packets[0])
	assert.Equal(t, []byte{4, 5, 6, 7, 8}, packets[1])
}

func TestOggOpusPackets_LacingContinuationWithinPage(t *testing.T) {
	// A 257-byte packet is laced as [255, 2] and must reassemble to one packet.
	body := make([]byte, 257)
	for i := range body {
		body[i] = byte(i)
	}
	page := makeOggPage([]byte{255, 2}, body)

	packets := oggPackets(page)
	require.Len(t, packets, 1)
	assert.Len(t, packets[0], 257)
}

func TestOggOpusPackets_PacketSpanningTwoPages(t *testing.T) {
	// Packet split across pages: page1 ends with a 255 segment (continues),
	// page2 begins with the remainder.
	p1body := make([]byte, 255)
	p2body := []byte{0xAA, 0xBB}
	page1 := makeOggPage([]byte{255}, p1body)
	page2 := makeOggPage([]byte{2}, p2body)

	packets := oggPackets(append(page1, page2...))
	require.Len(t, packets, 1)
	assert.Len(t, packets[0], 257)
}
