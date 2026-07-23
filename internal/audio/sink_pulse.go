//go:build linux || freebsd || openbsd || netbsd

package audio

import (
	"bytes"
	"sync"

	"github.com/jfreymuth/pulse"
	"github.com/jfreymuth/pulse/proto"
)

// On Linux and the BSDs oto needs a native backend (cgo), which breaks the
// static cgo-free build. Instead we talk to the system audio server (PulseAudio
// / PipeWire's pulse protocol) with the pure-Go jfreymuth/pulse client. One
// shared client per process.
var (
	clientOnce sync.Once
	clientInst *pulse.Client
	clientErr  error
)

func sharedClient() (*pulse.Client, error) {
	clientOnce.Do(func() {
		clientInst, clientErr = pulse.NewClient()
	})
	return clientInst, clientErr
}

// checkBackend verifies a PulseAudio/PipeWire server is reachable.
func checkBackend() error {
	_, err := sharedClient()
	return err
}

type pulseSink struct{ st *pulse.PlaybackStream }

func newSink(pcm []byte) (sink, error) {
	c, err := sharedClient()
	if err != nil {
		return nil, err
	}
	r := pulse.NewReader(bytes.NewReader(pcm), proto.FormatInt16LE)
	st, err := c.NewPlayback(r, pulse.PlaybackSampleRate(sampleRate))
	if err != nil {
		return nil, err
	}
	st.Start()
	return &pulseSink{st: st}, nil
}

func (s *pulseSink) pause()  { s.st.Pause() }
func (s *pulseSink) resume() { s.st.Resume() }
func (s *pulseSink) stop()   { s.st.Close() }
