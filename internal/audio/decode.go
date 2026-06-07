package audio

import "github.com/pion/opus"

const (
	sampleRate     = 48000
	channels       = 1
	bytesPerSecond = sampleRate * channels * 2 // signed 16-bit
)

// decodeVoicePCM decodes a Telegram voice message (Opus in an Ogg container)
// into 48 kHz mono signed-16-bit little-endian PCM. Undecodable packets are
// skipped rather than failing the whole stream.
func decodeVoicePCM(ogg []byte) ([]byte, error) {
	packets := oggPackets(ogg)
	// The first two packets are the Opus headers (OpusHead, OpusTags).
	if len(packets) > 2 {
		packets = packets[2:]
	} else {
		packets = nil
	}

	dec, err := opus.NewDecoderWithOutput(sampleRate, channels)
	if err != nil {
		return nil, err
	}

	out := make([]int16, 0, len(packets)*960)
	buf := make([]int16, 5760) // up to 120 ms at 48 kHz mono
	for _, pkt := range packets {
		if len(pkt) == 0 {
			continue
		}
		n, derr := dec.DecodeToInt16(pkt, buf)
		if derr != nil {
			continue
		}
		out = append(out, buf[:n]...)
	}

	pcm := make([]byte, len(out)*2)
	for i, s := range out {
		pcm[2*i] = byte(s)
		pcm[2*i+1] = byte(uint16(s) >> 8)
	}
	return pcm, nil
}
