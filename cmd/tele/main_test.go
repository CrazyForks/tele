package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultConfigPath(t *testing.T) {
	if got, want := defaultConfigPath("tele"), filepath.Join("~", ".config", "tele", "config.yml"); got != want {
		t.Fatalf("stable: got %q want %q", got, want)
	}
	if got, want := defaultConfigPath("tele-beta"), filepath.Join("~", ".config", "tele-beta", "config.yml"); got != want {
		t.Fatalf("beta: got %q want %q", got, want)
	}
}

func TestStateDir_UsesAppName(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_STATE_HOME", base)

	dir, err := stateDir("tele-beta")
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(base, "tele-beta")
	if dir != want {
		t.Fatalf("got %q want %q", dir, want)
	}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		t.Fatalf("state dir not created: %v", err)
	}
}
