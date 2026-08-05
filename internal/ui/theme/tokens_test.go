package theme_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/ui/theme"
)

const goldenKeys = "testdata/token_keys.golden"

// The token keys are what user theme files are written against. They are derived
// from the field names of Theme, so renaming a field renames a key and breaks
// every file that set it. Pinning the list here does not forbid that — it makes
// it show up in the diff, where it can be decided on rather than discovered by a
// user whose colors changed.
//
// To accept a change: delete testdata/token_keys.golden and run the test once.
func TestTokenKeys_MatchTheGoldenList(t *testing.T) {
	got := strings.Join(theme.TokenKeys(), "\n") + "\n"

	want, err := os.ReadFile(goldenKeys)
	if os.IsNotExist(err) {
		require.NoError(t, os.MkdirAll(filepath.Dir(goldenKeys), 0o755))
		require.NoError(t, os.WriteFile(goldenKeys, []byte(got), 0o644))
		t.Log("wrote " + goldenKeys)
		return
	}
	require.NoError(t, err)

	// Git hands this file over with CRLF on a Windows checkout. The token names
	// are what is being pinned, not how the working copy spells a line break.
	assert.Equal(t, strings.ReplaceAll(string(want), "\r\n", "\n"), got,
		"the token keys changed; user theme files are written against these names")
}

// Two fields that normalize to one key would leave one of them unreachable from
// a file, with nothing to show for it. The package refuses to load in that case;
// this states the rule where it can be read.
func TestTokenKeys_AreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, k := range theme.TokenKeys() {
		assert.False(t, seen[k], "duplicate token key %s", k)
		seen[k] = true
	}
	assert.NotEmpty(t, seen)
}

// Every key a dump writes must be one the loader accepts. A key that round-trips
// through neither direction is a token nobody can set.
func TestTokenKeys_AreAcceptedByTheLoader(t *testing.T) {
	var body strings.Builder
	for _, key := range theme.TokenKeys() {
		switch key {
		case "sender_palette":
			body.WriteString("sender_palette: [\"#ffffff\"]\n")
		case "logo_gradient":
			body.WriteString("logo_gradient: [{pos: 0, color: \"#000000\"}, {pos: 1, color: \"#ffffff\"}]\n")
		default:
			body.WriteString(key + ": \"#abcdef\"\n")
		}
	}
	dir := themesDir(t, map[string]string{"all.yml": body.String()})
	l := theme.NewLoader(dir)

	got := l.Resolve("all", theme.TeleDark)

	assert.Empty(t, l.Warnings())
	for _, key := range theme.TokenKeys() {
		assert.Equal(t, "all", got.Origins[key].Theme, "token %s was not set by the file", key)
	}
}
