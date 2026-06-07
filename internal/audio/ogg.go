package audio

import "bytes"

var oggCapture = []byte("OggS")

// oggPackets extracts logical packets from an Ogg bitstream, reassembling
// packets across the segment lacing table (and across page boundaries when a
// page ends on a 255-length segment). CRC and granule fields are ignored.
func oggPackets(data []byte) [][]byte {
	var packets [][]byte
	var cur []byte

	i := 0
	for i+27 <= len(data) {
		if !bytes.Equal(data[i:i+4], oggCapture) {
			break
		}
		segCount := int(data[i+26])
		segStart := i + 27
		bodyStart := segStart + segCount
		if bodyStart > len(data) {
			break
		}
		segTable := data[segStart:bodyStart]

		off := bodyStart
		for _, lace := range segTable {
			end := off + int(lace)
			if end > len(data) {
				return packets
			}
			cur = append(cur, data[off:end]...)
			off = end
			if lace < 255 {
				packets = append(packets, cur)
				cur = nil
			}
		}
		i = off
	}
	return packets
}
