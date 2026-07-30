package config

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/viper"
)

type TelegramConfig struct {
	APIID       int    `mapstructure:"api_id"`
	APIHash     string `mapstructure:"api_hash"`
	SessionFile string `mapstructure:"session_file"`
}

type UIConfig struct {
	Theme        string `mapstructure:"theme"`
	DateFormat   string `mapstructure:"date_format"`
	HistoryLimit int    `mapstructure:"history_limit"`
	// NotificationPreview controls whether the message text is included in
	// desktop notifications. Set false to send only the sender name (#80).
	NotificationPreview bool         `mapstructure:"notification_preview"`
	Toasts              ToastsConfig `mapstructure:"toasts"`
}

// ToastsConfig controls the floating toast component (#87). Zone strings are
// "bottom-right", "top-right", or "bottom-left"; unknown values fall back to
// the default at the UI layer.
type ToastsConfig struct {
	ErrorZone  string `mapstructure:"error_zone"`
	NotifyZone string `mapstructure:"notify_zone"`
	MaxVisible int    `mapstructure:"max_visible"`
}

type PhotosConfig struct {
	EagerFullQuality bool   `mapstructure:"eager_full_quality"`
	Mode             string `mapstructure:"mode"` // auto | kitty | blocks
	// KittyPlacementCap bounds how many Kitty image placements are kept on the
	// terminal at once. Transmitting an entire heavy chat exceeds the terminal's
	// limit and corrupts placements, so only on-screen images (plus a few
	// recently scrolled-past) stay transmitted. Lower it if images still corrupt.
	KittyPlacementCap int `mapstructure:"kitty_placement_cap"`
	// MaxLongSidePx caps a rendered inline image's long side in pixels (mirrors
	// the desktop client's fixed media ceiling). Height is additionally bounded
	// to a fraction of the chat pane. Raise it for larger inline images.
	MaxLongSidePx int `mapstructure:"max_long_side_px"`
	// DiskCacheSize bounds the on-disk media cache in bytes. Fetched thumbnails,
	// stickers and voice notes are cached per account under the user cache
	// directory, so a chat re-renders its images instantly on restart. 0 means
	// keep nothing between runs: the cache moves into the process's temp
	// directory under a fixed bound and is deleted on exit. See issues #174 and
	// #196.
	DiskCacheSize int64 `mapstructure:"disk_cache_size"`
}

type Config struct {
	Telegram    TelegramConfig            `mapstructure:"telegram"`
	UI          UIConfig                  `mapstructure:"ui"`
	Photos      PhotosConfig              `mapstructure:"photos"`
	Keybindings map[string]map[string]any `mapstructure:"keybindings"`

	// StateDir holds one account's state: the session, the SQLite database and
	// the ownership lock. See resolveState for how it is chosen.
	StateDir string `mapstructure:"state_dir"`

	// SessionPinned reports that telegram.session_file named a deliberate
	// location. The caller must not migrate files away from it.
	SessionPinned bool `mapstructure:"-"`

	// Warnings collects non-fatal config notices for the caller to log, in the
	// same spirit as keys.MergeOverrides.
	Warnings []string `mapstructure:"-"`
}

// Load reads the config at path. defaultStateDir is the platform state
// directory, used when the config names none.
func Load(path, defaultStateDir string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	setDefaults(v)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	cfg.resolveState(defaultStateDir)
	return &cfg, nil
}

// resolveState fixes StateDir and Telegram.SessionFile. Precedence:
//
//  1. state_dir is canonical when set; a session_file alongside it is ignored.
//  2. telegram.session_file alone keeps its own directory as the state
//     directory and pins it. A deliberate path — an encrypted volume, an
//     external disk — must never be relocated behind the user's back.
//  3. neither: the platform state directory. Legacy files next to the config
//     are moved into it at startup by statedir.Migrate.
func (c *Config) resolveState(defaultStateDir string) {
	switch {
	case c.StateDir != "":
		c.StateDir = ExpandTilde(c.StateDir)
		if c.Telegram.SessionFile != "" {
			c.Warnings = append(c.Warnings,
				"telegram.session_file is ignored because state_dir is set; remove it from the config")
		}
	case c.Telegram.SessionFile != "":
		c.Telegram.SessionFile = ExpandTilde(c.Telegram.SessionFile)
		c.StateDir = filepath.Dir(c.Telegram.SessionFile)
		c.SessionPinned = true
		c.Warnings = append(c.Warnings,
			"telegram.session_file is deprecated and will be removed in the next release; set state_dir instead")
		return
	default:
		c.StateDir = defaultStateDir
	}
	c.Telegram.SessionFile = filepath.Join(c.StateDir, "session.json")
}

// ExpandTilde replaces a leading ~/ with the user home directory. Paths that do
// not start with ~/ are returned unchanged.
func ExpandTilde(path string) string {
	if !strings.HasPrefix(path, "~/") {
		return path
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return path
	}
	return filepath.Join(home, path[2:])
}

// KeybindingOverrides flattens the raw keybindings section into
// context -> action -> []key, normalizing scalar ("R") and sequence
// (["g g","gg"]) values. Returns nil when the section is absent.
// Exported because internal/app and external tests call it across packages.
func (c *Config) KeybindingOverrides() map[string]map[string][]string {
	if len(c.Keybindings) == 0 {
		return nil
	}
	out := make(map[string]map[string][]string, len(c.Keybindings))
	for ctx, actions := range c.Keybindings {
		m := make(map[string][]string, len(actions))
		for action, raw := range actions {
			m[action] = toStringSlice(raw)
		}
		out[ctx] = m
	}
	return out
}

func toStringSlice(v any) []string {
	switch t := v.(type) {
	case string:
		return []string{t}
	case []string:
		return t
	case []any:
		out := make([]string, 0, len(t))
		for _, e := range t {
			if s, ok := e.(string); ok {
				out = append(out, s)
			}
		}
		return out
	default:
		return nil
	}
}
