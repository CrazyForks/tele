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
	"github.com/sorokin-vladimir/tele/internal/ui/keys"
)

// Injected at build time via -ldflags. Fall back to config file values if zero.
var (
	buildAPIID   = "0"
	buildAPIHash = ""
	version      = "dev"
	appName      = "tele" // injected via -ldflags for the beta channel
)

func main() {
	cfgPath := flag.String("config", defaultConfigPath(appName), "path to config file")
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

	sd, err := stateDir(appName)
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

const defaultConfigHead = `telegram:
  api_id: 0
  api_hash: ""

ui:
  date_format: "15:04"
  history_limit: 50
  theme: default

photos:
  eager_full_quality: true  # download full resolution in background on chat open

# Keybindings — every action with its current default keys (see docs/keybindings.md).
# Uncomment a line and change its key(s) to override that action in that context.
# One key replaces the defaults; a chord is space-separated tokens ("g g" = g then g).
`

// defaultConfig is the head plus the generated, fully-commented keybindings
// reference, so the written config stays in sync with the actual defaults.
func defaultConfig() string {
	return defaultConfigHead + keys.DefaultKeybindingsYAML()
}

func expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, path[2:])
	}
	return path
}

// defaultConfigPath returns the default config file location for the given app
// name, e.g. ~/.config/tele/config.yml (or ~/.config/tele-beta/config.yml for
// the beta channel). The tilde is expanded later by expandTilde.
func defaultConfigPath(app string) string {
	return filepath.Join("~", ".config", app, "config.yml")
}

// stateDir returns ~/.local/state/<app> (or $XDG_STATE_HOME/<app>) and ensures it exists.
func stateDir(app string) (string, error) {
	base := os.Getenv("XDG_STATE_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".local", "state")
	}
	dir := filepath.Join(base, app)
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
	return os.WriteFile(path, []byte(defaultConfig()), 0600)
}
