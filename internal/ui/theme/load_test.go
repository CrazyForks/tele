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

// themesDir writes the given files into a fresh themes directory.
func themesDir(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for name, body := range files {
		require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(body), 0600))
	}
	return dir
}

// hasWarning reports whether any warning mentions all the given fragments.
func hasWarning(warnings []string, fragments ...string) bool {
	for _, w := range warnings {
		ok := true
		for _, f := range fragments {
			if !strings.Contains(w, f) {
				ok = false
				break
			}
		}
		if ok {
			return true
		}
	}
	return false
}

// The common case: a file that sets a few tokens and takes the rest from the
// built-in it roots in.
func TestResolve_PartialThemeInheritsTheRest(t *testing.T) {
	dir := themesDir(t, map[string]string{
		"mine.yml": "border_pane_active: \"#8ec07c\"\n",
	})
	l := theme.NewLoader(dir)

	got := l.Resolve("mine", theme.TeleDark)

	assert.Empty(t, l.Warnings())
	assert.Equal(t, "mine", got.Theme.Name)
	assert.Equal(t, mustParse(t, "#8ec07c"), got.Theme.BorderPaneActive)
	assert.Equal(t, theme.TeleDark.SurfaceOverlay, got.Theme.SurfaceOverlay,
		"a token the file does not set comes from the built-in")
	assert.Equal(t, "mine", got.Origins["border_pane_active"].Theme)
	assert.Equal(t, "tele-dark", got.Origins["surface_overlay"].Theme)
}

// A theme with no base still resolves: the built-in for the slot is the root of
// every chain, which is what keeps a file working when tele adds a token.
func TestResolve_UnsetTokensNeverEmpty(t *testing.T) {
	dir := themesDir(t, map[string]string{"sparse.yml": "accent: red\n"})
	l := theme.NewLoader(dir)

	got := l.Resolve("sparse", theme.TeleLight)

	for _, key := range theme.TokenKeys() {
		assert.NotEmpty(t, got.Origins[key].Theme, "token %s has no origin", key)
	}
	assert.Equal(t, theme.TeleLight.SurfaceHelp, got.Theme.SurfaceHelp)
}

// base: may point at another file, and the chain is applied from the root
// outward so the nearest theme wins.
func TestResolve_BaseChain(t *testing.T) {
	dir := themesDir(t, map[string]string{
		"low.yml": "accent: \"#111111\"\ntext_dim: \"#222222\"\n",
		"mid.yml": "base: low\ntext_dim: \"#333333\"\n",
		"top.yml": "base: mid\nstatus_error: \"#444444\"\n",
	})
	l := theme.NewLoader(dir)

	got := l.Resolve("top", theme.TeleDark)

	assert.Empty(t, l.Warnings())
	assert.Equal(t, mustParse(t, "#111111"), got.Theme.Accent, "from the far end of the chain")
	assert.Equal(t, mustParse(t, "#333333"), got.Theme.TextDim, "the nearer theme wins")
	assert.Equal(t, mustParse(t, "#444444"), got.Theme.StatusError)
	assert.Equal(t, []string{"top", "mid", "low", "tele-dark"}, got.Chain)
}

// A chain may root in a built-in explicitly, including the one for the other
// slot: base is a reference to a palette, not to a slot.
func TestResolve_BaseMayNameABuiltin(t *testing.T) {
	dir := themesDir(t, map[string]string{
		"mine.yml": "base: tele-light\naccent: \"#abcdef\"\n",
	})
	l := theme.NewLoader(dir)

	got := l.Resolve("mine", theme.TeleDark)

	assert.Empty(t, l.Warnings())
	assert.Equal(t, theme.TeleLight.SurfaceHelp, got.Theme.SurfaceHelp,
		"the named base wins over the slot's built-in")
	assert.Equal(t, []string{"mine", "tele-light"}, got.Chain)
}

// A cycle is reported by name and costs the user their theme, not their session.
func TestResolve_BaseCycleWarnsAndFallsBack(t *testing.T) {
	dir := themesDir(t, map[string]string{
		"a.yml": "base: b\naccent: \"#111111\"\n",
		"b.yml": "base: a\naccent: \"#222222\"\n",
	})
	l := theme.NewLoader(dir)

	got := l.Resolve("a", theme.TeleDark)

	assert.Equal(t, theme.TeleDark.Accent, got.Theme.Accent)
	assert.True(t, hasWarning(l.Warnings(), "cycle"), "warnings: %v", l.Warnings())
}

// A name that resolves to nothing is a warning, never a failure to start.
func TestResolve_MissingThemeWarnsAndFallsBack(t *testing.T) {
	l := theme.NewLoader(themesDir(t, nil))

	got := l.Resolve("nope", theme.TeleDark)

	assert.Equal(t, "tele-dark", got.Theme.Name)
	assert.True(t, hasWarning(l.Warnings(), "nope"), "warnings: %v", l.Warnings())
}

// An unknown key is a warning and the rest of the file is still used: a theme
// written for a later tele must not stop working here.
func TestResolve_UnknownKeyWarnsButKeepsTheRest(t *testing.T) {
	dir := themesDir(t, map[string]string{
		"mine.yml": "accent: \"#abcdef\"\nsurface_bogus: \"#000000\"\n",
	})
	l := theme.NewLoader(dir)

	got := l.Resolve("mine", theme.TeleDark)

	assert.Equal(t, mustParse(t, "#abcdef"), got.Theme.Accent)
	assert.True(t, hasWarning(l.Warnings(), "surface_bogus"), "warnings: %v", l.Warnings())
}

// Token keys are matched normalized, the same way theme names are.
func TestResolve_TokenKeySpellings(t *testing.T) {
	dir := themesDir(t, map[string]string{
		"mine.yml": "surface-overlay: \"#010101\"\nsurfaceHelp: \"#020202\"\nSURFACE_TOAST: \"#030303\"\n",
	})
	l := theme.NewLoader(dir)

	got := l.Resolve("mine", theme.TeleDark)

	assert.Empty(t, l.Warnings())
	assert.Equal(t, mustParse(t, "#010101"), got.Theme.SurfaceOverlay)
	assert.Equal(t, mustParse(t, "#020202"), got.Theme.SurfaceHelp)
	assert.Equal(t, mustParse(t, "#030303"), got.Theme.SurfaceToast)
}

// A file named after a built-in is ignored rather than silently replacing the
// root every chain falls back to.
func TestNewLoader_BuiltinShadowIsRefused(t *testing.T) {
	dir := themesDir(t, map[string]string{
		"tele-dark.yml": "accent: \"#ff0000\"\n",
	})
	l := theme.NewLoader(dir)

	got := l.Resolve("tele-dark", theme.TeleLight)

	assert.Equal(t, theme.TeleDark.Accent, got.Theme.Accent, "the built-in wins")
	assert.True(t, hasWarning(l.Warnings(), "shadows"), "warnings: %v", l.Warnings())
}

// Two files whose names normalize alike are reported, and the winner is picked
// the only way that is the same on every machine.
func TestNewLoader_DuplicateNormalizedNames(t *testing.T) {
	dir := themesDir(t, map[string]string{
		"my-theme.yml": "accent: \"#111111\"\n",
		"my_theme.yml": "accent: \"#222222\"\n",
	})
	l := theme.NewLoader(dir)

	got := l.Resolve("mytheme", theme.TeleDark)

	assert.True(t, hasWarning(l.Warnings(), "same name"), "warnings: %v", l.Warnings())
	assert.Equal(t, mustParse(t, "#111111"), got.Theme.Accent, "my-theme.yml sorts first")
}

// Both file extensions are read.
func TestNewLoader_AcceptsYamlExtension(t *testing.T) {
	dir := themesDir(t, map[string]string{"mine.yaml": "accent: \"#abcdef\"\n"})
	l := theme.NewLoader(dir)

	got := l.Resolve("mine", theme.TeleDark)

	assert.Empty(t, l.Warnings())
	assert.Equal(t, mustParse(t, "#abcdef"), got.Theme.Accent)
}

// A missing themes directory is the normal case, not a problem.
func TestNewLoader_MissingDirectoryIsSilent(t *testing.T) {
	l := theme.NewLoader(filepath.Join(t.TempDir(), "absent"))
	assert.Empty(t, l.Warnings())
}

// none is the only way to leave the terminal's own backdrop showing, so it has
// to survive the round trip into a style.
func TestResolve_NoneIsAcceptedOnRenderedTokens(t *testing.T) {
	dir := themesDir(t, map[string]string{"mine.yml": "surface_overlay: none\n"})
	l := theme.NewLoader(dir)

	got := l.Resolve("mine", theme.TeleDark)

	assert.Empty(t, l.Warnings())
	assert.Equal(t, mustParse(t, "none"), got.Theme.SurfaceOverlay)
}

// The four interpolated tokens are arithmetic input: none there would mean
// black, so it is refused rather than obeyed.
func TestResolve_NoneIsRefusedOnInterpolatedTokens(t *testing.T) {
	for _, key := range []string{"highlight_accent", "highlight_error", "highlight_base_chat", "highlight_base_bubble"} {
		t.Run(key, func(t *testing.T) {
			dir := themesDir(t, map[string]string{"mine.yml": key + ": none\n"})
			l := theme.NewLoader(dir)

			got := l.Resolve("mine", theme.TeleDark)

			assert.True(t, hasWarning(l.Warnings(), key, "none"), "warnings: %v", l.Warnings())
			assert.Equal(t, "tele-dark", got.Origins[key].Theme, "the refused token keeps the inherited value")
		})
	}
}

// Sender colors are never interpolated, so none there is a legitimate "do not
// color this name".
func TestResolve_NoneIsAllowedInSenderPalette(t *testing.T) {
	dir := themesDir(t, map[string]string{
		"mine.yml": "sender_palette: [none, \"#ff0000\"]\n",
	})
	l := theme.NewLoader(dir)

	got := l.Resolve("mine", theme.TeleDark)

	assert.Empty(t, l.Warnings())
	require.Len(t, got.Theme.SenderPalette, 2)
	assert.Equal(t, mustParse(t, "none"), got.Theme.SenderPalette[0])
}

// A list token replaces the one it inherited rather than merging into it: a
// per-entry merge has no answer that is not a surprise.
func TestResolve_ListTokensReplaceWhole(t *testing.T) {
	dir := themesDir(t, map[string]string{
		"mine.yml": "sender_palette: [\"#ff0000\", \"#00ff00\"]\n",
	})
	l := theme.NewLoader(dir)

	got := l.Resolve("mine", theme.TeleDark)

	assert.Empty(t, l.Warnings())
	assert.Len(t, got.Theme.SenderPalette, 2, "not merged into the built-in's eight")
}

// Palette indexes and ANSI names are colors in their own right: they follow the
// terminal's palette instead of overriding it.
func TestResolve_IndexAndAnsiNameColors(t *testing.T) {
	dir := themesDir(t, map[string]string{
		"mine.yml": "accent: 240\ntext_dim: bright-blue\nstatus_error: \"#f00\"\n",
	})
	l := theme.NewLoader(dir)

	got := l.Resolve("mine", theme.TeleDark)

	assert.Empty(t, l.Warnings())
	assert.Equal(t, mustParse(t, "240"), got.Theme.Accent)
	assert.Equal(t, mustParse(t, "12"), got.Theme.TextDim)
	assert.Equal(t, mustParse(t, "#ff0000"), got.Theme.StatusError, "the short hex form expands")
	assert.Equal(t, "240", got.Origins["accent"].Raw, "the text is kept so a dump can re-emit it")
}

func TestResolve_GradientRules(t *testing.T) {
	for name, body := range map[string]string{
		"too few stops":    "logo_gradient: [{pos: 0, color: \"#000000\"}]\n",
		"does not start 0": "logo_gradient: [{pos: 0.2, color: \"#000000\"}, {pos: 1, color: \"#ffffff\"}]\n",
		"does not end 1":   "logo_gradient: [{pos: 0, color: \"#000000\"}, {pos: 0.8, color: \"#ffffff\"}]\n",
		"not ascending":    "logo_gradient: [{pos: 0, color: \"#000000\"}, {pos: 0, color: \"#ffffff\"}]\n",
		"none stop":        "logo_gradient: [{pos: 0, color: none}, {pos: 1, color: \"#ffffff\"}]\n",
	} {
		t.Run(name, func(t *testing.T) {
			dir := themesDir(t, map[string]string{"mine.yml": body})
			l := theme.NewLoader(dir)

			got := l.Resolve("mine", theme.TeleDark)

			assert.NotEmpty(t, l.Warnings())
			assert.Equal(t, theme.TeleDark.LogoGradient, got.Theme.LogoGradient)
		})
	}
}

func TestResolve_GradientAccepted(t *testing.T) {
	dir := themesDir(t, map[string]string{
		"mine.yml": "logo_gradient:\n  - {pos: 0, color: \"#000000\"}\n  - {pos: 0.5, color: \"#808080\"}\n  - {pos: 1, color: \"#ffffff\"}\n",
	})
	l := theme.NewLoader(dir)

	got := l.Resolve("mine", theme.TeleDark)

	assert.Empty(t, l.Warnings())
	require.Len(t, got.Theme.LogoGradient, 3)
	assert.Equal(t, 0.5, got.Theme.LogoGradient[1].Pos)
}

// A malformed file costs the user their theme, not their session.
func TestResolve_MalformedFileWarnsAndFallsBack(t *testing.T) {
	dir := themesDir(t, map[string]string{"mine.yml": "accent: [unclosed\n"})
	l := theme.NewLoader(dir)

	got := l.Resolve("mine", theme.TeleDark)

	assert.Equal(t, "tele-dark", got.Theme.Name)
	assert.NotEmpty(t, l.Warnings())
}

// An empty slot means the built-in, with no file read at all.
func TestResolve_EmptyNameIsTheBuiltin(t *testing.T) {
	l := theme.NewLoader(themesDir(t, nil))

	got := l.Resolve("", theme.TeleLight)

	assert.Empty(t, l.Warnings())
	assert.Equal(t, "tele-light", got.Theme.Name)
}

// What a dump writes must load back to the same thing, or it is not the way to
// start a theme.
func TestDump_RoundTrips(t *testing.T) {
	src := themesDir(t, map[string]string{
		"mine.yml": "accent: 240\nsurface_overlay: none\nsender_palette: [\"#ff0000\"]\n",
	})
	first := theme.NewLoader(src).Resolve("mine", theme.TeleDark)

	dir := themesDir(t, map[string]string{"dumped.yml": theme.Dump(first)})
	l := theme.NewLoader(dir)
	second := l.Resolve("dumped", theme.TeleLight)

	assert.Empty(t, l.Warnings(), "a dump must be a valid theme file")
	assert.Equal(t, first.Theme.Accent, second.Theme.Accent)
	assert.Equal(t, first.Theme.SurfaceOverlay, second.Theme.SurfaceOverlay)
	assert.Equal(t, first.Theme.SenderPalette, second.Theme.SenderPalette)
	assert.Equal(t, first.Theme.LogoGradient, second.Theme.LogoGradient)
	assert.Equal(t, "240", second.Origins["accent"].Raw, "an index survives the round trip as an index")
	for _, key := range theme.TokenKeys() {
		assert.Equal(t, "dumped", second.Origins[key].Theme, "token %s must be set by the dump itself", key)
	}
}

func mustParse(t *testing.T, s string) any {
	t.Helper()
	c, err := theme.ParseColor(s)
	require.NoError(t, err)
	return c
}
