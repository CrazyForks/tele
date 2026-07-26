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

func TestStateDirPath_UsesXDGStateHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "/xdg/state")

	got, err := stateDirPath("tele")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join("/xdg/state", "tele"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestStateDirPath_FallsBackToHome(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", "")
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	got, err := stateDirPath("tele-beta")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(home, ".local", "state", "tele-beta"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

// stateDirPath must not create anything. Creation belongs to whoever owns the
// directory: statedir.Acquire for the state directory, main for the log
// directory.
func TestStateDirPath_DoesNotCreateTheDirectory(t *testing.T) {
	base := t.TempDir()
	t.Setenv("XDG_STATE_HOME", base)

	got, err := stateDirPath("tele-beta")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(got); !os.IsNotExist(err) {
		t.Fatalf("directory should not exist, stat err = %v", err)
	}
}
