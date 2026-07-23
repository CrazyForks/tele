//go:build !freebsd

package app

import "github.com/gen2brain/beeep"

// beeepNotifier posts a generic out-of-process desktop notification. It is the
// fallback used when the terminal-native OSC path is unavailable. Excluded on
// freebsd, where beeep's godbus dependency does not build.
type beeepNotifier struct{}

func (b beeepNotifier) Notify(title, body string) error {
	return beeep.Notify(title, body, "")
}

func fallbackNotifier() Notifier { return beeepNotifier{} }
