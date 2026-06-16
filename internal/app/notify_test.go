package app

import (
	"strings"
	"testing"
)

func TestDetectOSCFormat(t *testing.T) {
	cases := []struct {
		name string
		env  map[string]string
		want oscFormat
	}{
		{"ghostty", map[string]string{"TERM_PROGRAM": "ghostty"}, osc777},
		{"wezterm", map[string]string{"TERM_PROGRAM": "WezTerm"}, osc777},
		{"iterm", map[string]string{"TERM_PROGRAM": "iTerm.app"}, osc9},
		{"foot", map[string]string{"TERM": "foot-extra"}, osc777},
		{"unknown", map[string]string{"TERM_PROGRAM": "Apple_Terminal"}, oscNone},
		{"empty", map[string]string{}, oscNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := detectOSCFormat(func(k string) string { return tc.env[k] })
			if got != tc.want {
				t.Errorf("detectOSCFormat = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestOSCNotifierNotify(t *testing.T) {
	t.Run("osc777 keeps title and body in separate fields", func(t *testing.T) {
		var buf strings.Builder
		n := &oscNotifier{w: &buf, format: osc777}
		if err := n.Notify("Alice", "hello"); err != nil {
			t.Fatal(err)
		}
		if got, want := buf.String(), "\x1b]777;notify;Alice;hello\x07"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("osc9 folds title into body", func(t *testing.T) {
		var buf strings.Builder
		n := &oscNotifier{w: &buf, format: osc9}
		if err := n.Notify("Alice", "hello"); err != nil {
			t.Fatal(err)
		}
		if got, want := buf.String(), "\x1b]9;Alice: hello\x07"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("control chars that would break the sequence are stripped", func(t *testing.T) {
		var buf strings.Builder
		n := &oscNotifier{w: &buf, format: osc777}
		if err := n.Notify("Ali\x1bce", "he\x07ll\no"); err != nil {
			t.Fatal(err)
		}
		if got, want := buf.String(), "\x1b]777;notify;Alice;hello\x07"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("semicolons in body are preserved", func(t *testing.T) {
		var buf strings.Builder
		n := &oscNotifier{w: &buf, format: osc777}
		if err := n.Notify("Bob", "a; b; c"); err != nil {
			t.Fatal(err)
		}
		if got, want := buf.String(), "\x1b]777;notify;Bob;a; b; c\x07"; got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}
