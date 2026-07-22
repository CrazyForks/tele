package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sorokin-vladimir/tele/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_ValidYAML(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yml")
	require.NoError(t, os.WriteFile(f, []byte(`
telegram:
  api_id: 12345
  api_hash: "abc"
  session_file: "/tmp/session.json"
ui:
  date_format: "15:04"
  history_limit: 100
`), 0600))

	cfg, err := config.Load(f)
	require.NoError(t, err)
	assert.Equal(t, 12345, cfg.Telegram.APIID)
	assert.Equal(t, "abc", cfg.Telegram.APIHash)
	assert.Equal(t, "/tmp/session.json", cfg.Telegram.SessionFile)
	assert.Equal(t, "15:04", cfg.UI.DateFormat)
	assert.Equal(t, 100, cfg.UI.HistoryLimit)
}

func TestLoad_SessionFileDerivedFromConfigDir(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yml")
	require.NoError(t, os.WriteFile(f, []byte("telegram:\n  api_id: 1\n  api_hash: x\n"), 0600))

	cfg, err := config.Load(f)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dir, "session.json"), cfg.Telegram.SessionFile)
}

func TestLoad_Defaults(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yml")
	require.NoError(t, os.WriteFile(f, []byte("telegram:\n  api_id: 1\n  api_hash: x\n"), 0600))

	cfg, err := config.Load(f)
	require.NoError(t, err)
	assert.Equal(t, 50, cfg.UI.HistoryLimit)
	assert.Equal(t, "15:04", cfg.UI.DateFormat)
	assert.Equal(t, "default", cfg.UI.Theme)
}

func TestDefaults_Toasts(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yml")
	require.NoError(t, os.WriteFile(f, []byte("telegram:\n  api_id: 1\n  api_hash: x\n"), 0600))

	cfg, err := config.Load(f)
	require.NoError(t, err)
	assert.Equal(t, "bottom-right", cfg.UI.Toasts.ErrorZone)
	assert.Equal(t, "top-right", cfg.UI.Toasts.NotifyZone)
	assert.Equal(t, 3, cfg.UI.Toasts.MaxVisible)
}

func TestDefaults_PhotosDiskCacheSize(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yml")
	require.NoError(t, os.WriteFile(f, []byte("telegram:\n  api_id: 1\n  api_hash: x\n"), 0600))

	cfg, err := config.Load(f)
	require.NoError(t, err)
	assert.Equal(t, int64(256*1024*1024), cfg.Photos.DiskCacheSize)
}

func TestLoad_MissingFile(t *testing.T) {
	_, err := config.Load("/nonexistent/config.yml")
	assert.Error(t, err)
}

func TestLoad_KeybindingsScalarAndSequence(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yml")
	require.NoError(t, os.WriteFile(f, []byte(`
telegram:
  api_id: 1
  api_hash: x
keybindings:
  chat:
    reply: "R"
    go_top: ["g g", "gg"]
`), 0600))

	cfg, err := config.Load(f)
	require.NoError(t, err)

	ov := cfg.KeybindingOverrides()
	assert.Equal(t, []string{"R"}, ov["chat"]["reply"])
	assert.Equal(t, []string{"g g", "gg"}, ov["chat"]["go_top"])
}

func TestKeybindingOverrides_AbsentSectionIsNil(t *testing.T) {
	dir := t.TempDir()
	f := filepath.Join(dir, "config.yml")
	require.NoError(t, os.WriteFile(f, []byte("telegram:\n  api_id: 1\n  api_hash: x\n"), 0600))

	cfg, err := config.Load(f)
	require.NoError(t, err)
	assert.Nil(t, cfg.KeybindingOverrides())
}
