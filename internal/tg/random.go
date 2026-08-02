package tg

import (
	"crypto/rand"
	"encoding/binary"
)

// NewRandomID returns a value for Telegram's random_id, the field it
// deduplicates sends on. Callers own it: generating one per attempt inside a
// retry loop defeats the deduplication entirely, which is how a single send
// could arrive twice (#193).
//
// Zero is remapped because Telegram treats an absent random_id as zero, and a
// send that cannot be told apart from "unset" cannot be deduplicated.
func NewRandomID() int64 {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		// crypto/rand does not fail on any platform this runs on, and there is
		// no fallback worth having: an id we cannot reason about is worse than
		// a crash, because it silently sends the message twice.
		panic("tg: crypto/rand unavailable: " + err.Error())
	}
	id := int64(binary.LittleEndian.Uint64(buf[:]))
	if id == 0 {
		return 1
	}
	return id
}
