//go:build darwin || windows

package audio

import (
	"bytes"
	"sync"

	"github.com/ebitengine/oto/v3"
)

// oto is cgo-free on macOS/Windows (purego). A single context per process.
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

// checkBackend verifies the audio device is available.
func checkBackend() error {
	_, err := sharedContext()
	return err
}

type otoSink struct{ p *oto.Player }

func newSink(pcm []byte) (sink, error) {
	ctx, err := sharedContext()
	if err != nil {
		return nil, err
	}
	pl := ctx.NewPlayer(bytes.NewReader(pcm))
	pl.Play()
	return &otoSink{p: pl}, nil
}

func (s *otoSink) pause()  { s.p.Pause() }
func (s *otoSink) resume() { s.p.Play() }
func (s *otoSink) stop()   { s.p.Pause() } // oto v3.4+: Close unnecessary; drop the ref
