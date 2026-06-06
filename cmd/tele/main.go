package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/sorokin-vladimir/tele/internal/app"
	"github.com/sorokin-vladimir/tele/internal/config"
)

// Injected at build time via -ldflags. Fall back to config file values if zero.
var (
	buildAPIID   = "0"
	buildAPIHash = ""
	version      = "dev"
)

func main() {
	cfgPath := flag.String("config", "~/.config/tele/config.yml", "path to config file")
	verbose := flag.Bool("e", false, "debug logging")
	trace := flag.Bool("trace", false, "log sensitive metadata (peer IDs, message lengths) — never use in shared environments")
	versionFlag := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *versionFlag {
		fmt.Println(version)
		os.Exit(0)
	}

	expanded := expandTilde(*cfgPath)
	cfgPath = &expanded

	if err := ensureConfig(*cfgPath); err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	if cfg.Telegram.APIID == 0 {
		if id, err := strconv.Atoi(buildAPIID); err == nil && id != 0 {
			cfg.Telegram.APIID = id
		}
	}
	if cfg.Telegram.APIHash == "" {
		cfg.Telegram.APIHash = buildAPIHash
	}

	if cfg.Telegram.APIID == 0 || cfg.Telegram.APIHash == "" {
		fmt.Fprintf(os.Stderr, "config: set telegram.api_id and telegram.api_hash in %s\nGet credentials at https://my.telegram.org\n", *cfgPath)
		os.Exit(1)
	}

	sd, err := stateDir()
	if err != nil {
		fmt.Fprintf(os.Stderr, "state dir: %v\n", err)
		os.Exit(1)
	}
	level := zap.NewAtomicLevelAt(zap.InfoLevel)
	if *verbose {
		level = zap.NewAtomicLevelAt(zap.DebugLevel)
	}
	w := zapcore.AddSync(&lumberjack.Logger{
		Filename:   filepath.Join(sd, "tele.log"),
		MaxSize:    10,
		MaxBackups: 3,
		Compress:   false,
	})
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		w,
		level,
	)
	log := zap.New(core)
	defer log.Sync() //nolint:errcheck

	a, err := app.New(cfg, log, *verbose, *trace)
	if err != nil {
		fmt.Fprintf(os.Stderr, "init: %v\n", err)
		os.Exit(1)
	}
	if err := a.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

const defaultConfig = `telegram:
  api_id: 0
  api_hash: ""

ui:
  date_format: "15:04"
  history_limit: 50
  theme: default

photos:
  eager_full_quality: true  # download full resolution in background on chat open

# keybindings:                 # override default keys; see README for action/context names
#   chat:                      # context: global | folders | chatlist | chat | composer | search | context_menu | delete_submenu
#     reply: "R"               # one key per action replaces that action's defaults
#     go_top: ["g g", "gg"]    # a chord is space-separated key tokens ("g g" = press g then g)
`

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// stateDir returns ~/.local/state/tele (or $XDG_STATE_HOME/tele) and ensures it exists.
func stateDir() (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	dir := filepath.Join(base, "tele")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", err
	}
	return dir, nil
}

// ensureConfig creates a default config file if it does not exist.
func ensureConfig(path string) error {
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(defaultConfig), 0600)
}
