package theme_test

import (
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/ui/theme"
	"github.com/sorokin-vladimir/tele/themes"
)

// bundledNames lists what is actually in the binary, so the tests below cover a
// palette added to themes/ without anyone remembering to name it here.
func bundledNames(t *testing.T) []string {
	t.Helper()
	entries, err := fs.ReadDir(themes.FS, ".")
	require.NoError(t, err)

	var names []string
	for _, e := range entries {
		if ext := filepath.Ext(e.Name()); ext == ".yml" || ext == ".yaml" {
			names = append(names, strings.TrimSuffix(e.Name(), ext))
		}
	}
	require.NotEmpty(t, names, "no themes are embedded; the go:embed pattern matched nothing")
	return names
}

// We ship these, so anything wrong with one of them is our defect and not the
// user's: an unknown key, a color that will not parse, a refused token
// dependency and an unreadable token all have to fail here rather than on
// someone's screen. Warnings and findings are checked together because they
// catch different halves of it — a misspelled token key never reaches the audit,
// because it never becomes a token.
func TestBundled_EveryThemeResolvesClean(t *testing.T) {
	for _, name := range bundledNames(t) {
		t.Run(name, func(t *testing.T) {
			l := theme.NewLoader(themesDir(t, nil))

			got := l.Resolve(name, theme.TeleDark)

			assert.Equal(t, name, got.Theme.Name)
			assert.Empty(t, l.Warnings(), "%s does not load cleanly", name)
			assert.Empty(t, got.Findings, "%s claims a canvas it cannot be read on:\n%s",
				name, theme.Report(name, got))
		})
	}
}

// The whole point of embedding: a fresh install has no themes directory at all,
// and every bundled name still resolves.
func TestBundled_ResolvesWithoutAThemesDirectory(t *testing.T) {
	l := theme.NewLoader(filepath.Join(t.TempDir(), "absent"))

	got := l.Resolve("nord", theme.TeleDark)

	assert.Empty(t, l.Warnings())
	assert.Equal(t, "nord", got.Theme.Name)
	assert.Equal(t, theme.SourceBundled, got.Sources["nord"])
	assert.Equal(t, theme.SourceBuiltin, got.Sources["tele-dark"])
}

// A bundled theme is a legal base, which is how "I like nord but for two
// colors" is written without copying sixty-two tokens.
func TestBundled_IsALegalBase(t *testing.T) {
	dir := themesDir(t, map[string]string{
		"mine.yml": "base: nord\naccent: \"#abcdef\"\n",
	})
	l := theme.NewLoader(dir)

	got := l.Resolve("mine", theme.TeleDark)

	assert.Empty(t, l.Warnings())
	assert.Equal(t, []string{"mine", "nord", "tele-dark"}, got.Chain)
	assert.Equal(t, theme.SourceFile, got.Sources["mine"])
	assert.Equal(t, theme.SourceBundled, got.Sources["nord"])
	assert.Equal(t, "nord", got.Origins["surface_overlay"].Theme,
		"a token the file does not set comes from the bundled theme it names")
}

// A file outranks the bundled theme of the same name, and replaces it rather
// than layering onto it: what the file leaves unset comes from the built-in,
// not from the palette whose name it took. Inheritance that appears in no file
// would make the theme unreadable from its own source.
func TestBundled_AUserFileReplacesIt(t *testing.T) {
	dir := themesDir(t, map[string]string{"nord.yml": "accent: \"#abcdef\"\n"})
	l := theme.NewLoader(dir)

	got := l.Resolve("nord", theme.TeleDark)

	assert.Empty(t, l.Warnings(), "replacing a bundled theme is allowed and is not warned about")
	assert.Equal(t, []string{"nord", "tele-dark"}, got.Chain)
	assert.Equal(t, theme.SourceFile, got.Sources["nord"])
	assert.Equal(t, theme.TeleDark.SurfaceOverlay, got.Theme.SurfaceOverlay,
		"the bundled nord contributes nothing to the file that replaced it")
}

// Replacing a bundled theme is invisible on screen: same name in the config,
// different colors. --theme-check is where that is said.
func TestBundled_ReportNamesTheFileThatTookTheName(t *testing.T) {
	dir := themesDir(t, map[string]string{"nord.yml": "accent: \"#abcdef\"\n"})
	l := theme.NewLoader(dir)

	got := l.Resolve("nord", theme.TeleDark)

	assert.Equal(t, []string{"nord"}, got.Shadows)
	report := theme.Report("dark", got)
	assert.Contains(t, report, "chain: nord (file) <- tele-dark (built-in)")
	assert.Contains(t, report, "note: nord is also a bundled theme")
}

// The reverse: a theme nobody has taken the name of says nothing about it.
func TestBundled_NoNoteWhenNothingWasShadowed(t *testing.T) {
	l := theme.NewLoader(themesDir(t, nil))

	got := l.Resolve("nord", theme.TeleDark)

	assert.Empty(t, got.Shadows)
	assert.NotContains(t, theme.Report("dark", got), "note:")
}
