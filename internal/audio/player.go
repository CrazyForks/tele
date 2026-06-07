// Package audio provides in-app playback of Telegram voice messages: it decodes
// Opus/Ogg to PCM (pure Go, via pion/opus) and plays it through the system
// audio device (ebitengine/oto, cgo-free via purego).
package audio

import (
	"bytes"
	"io"
	"sync"

	"github.com/ebitengine/oto/v3"
)

// shared oto context: only one may exist per process.
var (
	ctxOnce sync.Once
	ctxInst *oto.Context
	ctxErr  error
)

func sharedContext() (*oto.Context, error) {
	ctxOnce.Do(func() {
		c, ready, err := oto.NewContext(&oto.NewContextOptions{
			SampleRate:   sampleRate,
			ChannelCount: channels,
			Format:       oto.FormatSignedInt16LE,
		})
		if err != nil {
			ctxErr = err
			return
		}
		<-ready
		ctxInst = c
	})
	return ctxInst, ctxErr
}

// countingReader tracks how many bytes oto has pulled from the PCM buffer, so
// the player can report playback position.
type countingReader struct {
	r  io.Reader
	mu sync.Mutex
	n  int64
}

func (c *countingReader) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.mu.Lock()
	c.n += int64(n)
	c.mu.Unlock()
	return n, err
}

func (c *countingReader) count() int64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.n
}

type current struct {
	player *oto.Player
	reader *countingReader
	total  int64
	docID  int64
	paused bool
}

// Player plays a single voice message at a time through the system audio device.
type Player struct {
	ctx *oto.Context
	mu  sync.Mutex
	cur *current
}

// NewPlayer initialises the shared audio context. Returns an error when no audio
// device is available; callers should degrade gracefully.
func NewPlayer() (*Player, error) {
	ctx, err := sharedContext()
	if err != nil {
		return nil, err
	}
	return &Player{ctx: ctx}, nil
}

// Play decodes and starts playing the given voice document, replacing any
// current playback.
func (p *Player) Play(docID int64, ogg []byte) error {
	pcm, err := decodeVoicePCM(ogg)
	if err != nil {
		return err
	}
	p.Stop()

	reader := &countingReader{r: bytes.NewReader(pcm)}
	pl := p.ctx.NewPlayer(reader)
	pl.Play()

	p.mu.Lock()
	p.cur = &current{player: pl, reader: reader, total: int64(len(pcm)), docID: docID}
	p.mu.Unlock()
	return nil
}

// Toggle pauses or resumes the active playback. Returns true if the requested
// docID is the one playing (so callers can decide between toggle and restart).
func (p *Player) Toggle(docID int64) bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.cur == nil || p.cur.docID != docID {
		return false
	}
	if p.cur.paused {
		p.cur.player.Play()
		p.cur.paused = false
	} else {
		p.cur.player.Pause()
		p.cur.paused = true
	}
	return true
}

// Stop halts and clears any playback.
func (p *Player) Stop() {
	p.mu.Lock()
	cur := p.cur
	p.cur = nil
	p.mu.Unlock()
	if cur != nil {
		// oto v3.4+: Close is unnecessary; pausing and dropping the reference
		// lets the player be reclaimed.
		cur.player.Pause()
	}
}

// State reports the current playback. active is false when nothing is playing or
// the message has finished (which also clears it).
func (p *Player) State() (docID int64, progress float64, posSecs int, active bool) {
	p.mu.Lock()
	cur := p.cur
	p.mu.Unlock()
	if cur == nil {
		return 0, 0, 0, false
	}

	played := cur.reader.count() - int64(cur.player.BufferedSize())
	if played < 0 {
		played = 0
	}
	if played > cur.total {
		played = cur.total
	}

	// Finished: not paused, device drained, and all bytes consumed.
	if !cur.paused && !cur.player.IsPlaying() && played >= cur.total {
		p.Stop()
		return 0, 0, 0, false
	}

	if cur.total > 0 {
		progress = float64(played) / float64(cur.total)
	}
	return cur.docID, progress, int(played / bytesPerSecond), true
}
