package ui

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// downloadsDir resolves the OS Downloads directory. A package var so tests can
// stub it.
var downloadsDir = resolveDownloadsDir

// resolveDownloadsDir returns the user's Downloads directory, falling back to
// the home dir and finally the OS temp dir so it always yields a usable path.
func resolveDownloadsDir() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS != "darwin" {
		if d := os.Getenv("XDG_DOWNLOAD_DIR"); d != "" {
			return d
		}
		if d := xdgDownloadFromUserDirs(home); d != "" {
			return d
		}
	}
	if home != "" {
		dl := filepath.Join(home, "Downloads")
		if fi, err := os.Stat(dl); err == nil && fi.IsDir() {
			return dl
		}
		return home
	}
	return os.TempDir()
}

// xdgDownloadFromUserDirs parses ~/.config/user-dirs.dirs for an
// XDG_DOWNLOAD_DIR="$HOME/..." entry and expands it. Returns "" if absent.
func xdgDownloadFromUserDirs(home string) string {
	if home == "" {
		return ""
	}
	f, err := os.Open(filepath.Join(home, ".config", "user-dirs.dirs"))
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.HasPrefix(line, "XDG_DOWNLOAD_DIR=") {
			continue
		}
		val := strings.Trim(strings.TrimPrefix(line, "XDG_DOWNLOAD_DIR="), `"`)
		val = strings.Replace(val, "$HOME", home, 1)
		if val != "" {
			return val
		}
	}
	return ""
}

// Creating the saved file and naming it moved to the owner in #196: the name
// follows from the media reference and its MIME type, which is domain knowledge
// rather than rendering. See core.createUniqueFile and core.savedFileName.
