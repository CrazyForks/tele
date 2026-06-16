package app

import (
	"fmt"
	"io"
	"os"
	"strings"

	"go.uber.org/zap"
)

// Desktop notifications prefer a terminal-native path: the app writes an OSC
// escape sequence and the terminal turns it into a system notification that it
// binds to the originating tab/window. Clicking such a notification focuses the
// exact tab the app runs in — something an out-of-process notifier cannot do,
// and the core of #17. Terminals that don't support the escape fall back to
// beeep (which posts a generic notification with no tab affinity).

type oscFormat int

const (
	oscNone oscFormat = iota
	// osc777 carries an explicit title: ESC ] 777 ; notify ; <title> ; <body> BEL.
	osc777
	// osc9 has no title field (the terminal name is shown instead):
	// ESC ] 9 ; <body> BEL. We fold the title into the body.
	osc9
)

// detectOSCFormat returns the notification escape a terminal is known to
// support, or oscNone for terminals where the escape would be silently dropped
// (posting there would show the user nothing, so we must fall back instead).
func detectOSCFormat(getenv func(string) string) oscFormat {
	switch getenv("TERM_PROGRAM") {
	case "ghostty":
		return osc777
	case "WezTerm":
		return osc777
	case "iTerm.app":
		return osc9
	}
	// foot (Linux) identifies itself via TERM and supports OSC 777.
	if strings.HasPrefix(getenv("TERM"), "foot") {
		return osc777
	}
	return oscNone
}

type oscNotifier struct {
	w      io.Writer
	format oscFormat
}

// sanitizeOSC drops bytes that would prematurely terminate or corrupt an OSC
// sequence. Other characters (including ';') are left intact: the body is the
// final field, so a ';' in message text does not split it.
func sanitizeOSC(s string) string {
	return strings.Map(func(r rune) rune {
		switch r {
		case '\x1b', '\x07', '\r', '\n':
			return -1
		}
		return r
	}, s)
}

func (n *oscNotifier) Notify(title, body string) error {
	title = sanitizeOSC(title)
	body = sanitizeOSC(body)
	var seq string
	switch n.format {
	case osc777:
		seq = fmt.Sprintf("\x1b]777;notify;%s;%s\x07", title, body)
	case osc9:
		msg := body
		if title != "" {
			msg = title + ": " + body
		}
		seq = fmt.Sprintf("\x1b]9;%s\x07", msg)
	default:
		return nil
	}
	// One Write so the sequence reaches the terminal whole. We target stderr:
	// bubbletea renders to stdout, and the tty serialises writes across FDs, so
	// our escape lands between frames rather than tearing one.
	_, err := io.WriteString(n.w, seq)
	return err
}

// newNotifier picks the best available desktop notifier: terminal-native OSC
// when the terminal supports it and stderr is a real terminal, otherwise beeep.
func newNotifier(log *zap.Logger) Notifier {
	if f := detectOSCFormat(os.Getenv); f != oscNone && isTerminal(os.Stderr) {
		log.Debug("notifications: using terminal-native OSC", zap.Int("format", int(f)))
		return &oscNotifier{w: os.Stderr, format: f}
	}
	return beeepNotifier{}
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
