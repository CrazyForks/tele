//go:build freebsd

package app

import "github.com/sorokin-vladimir/tele/internal/core"

// noopNotifier is the fallback on freebsd, where beeep's godbus dependency does
// not build. Desktop notifications still work through the terminal-native OSC
// path (see newNotifier); only the out-of-process fallback is a no-op here.
type noopNotifier struct{}

func (noopNotifier) Notify(title, body string) error { return nil }

func fallbackNotifier() core.Notifier { return noopNotifier{} }
