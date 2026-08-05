package theme_test

import (
	"image/color"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/ui/theme"
)

// builtins are the compiled-in themes, checked as a set so a third one cannot be
// added without meeting the same bar.
func builtins(t *testing.T) map[string]theme.Theme {
	t.Helper()
	out := make(map[string]theme.Theme)
	for _, name := range theme.BuiltinNames() {
		th, ok := theme.Builtin(name)
		require.True(t, ok, "built-in %s must resolve", name)
		out[name] = th
	}
	return out
}

// Every token of every built-in must be populated. A field added to Theme and
// filled in only one of them renders as transparent black at runtime with no
// other signal, so it is caught here instead.
func TestBuiltins_FullyPopulated(t *testing.T) {
	for name, th := range builtins(t) {
		t.Run(name, func(t *testing.T) {
			v := reflect.ValueOf(th)
			typ := v.Type()
			for i := range typ.NumField() {
				f := typ.Field(i)
				switch f.Type {
				case reflect.TypeFor[color.Color]():
					assert.NotNil(t, v.Field(i).Interface(), "token %s is nil", f.Name)
				case reflect.TypeFor[[]color.Color]():
					pal := v.Field(i).Interface().([]color.Color)
					assert.NotEmpty(t, pal, "token %s must have at least one entry", f.Name)
					for j, c := range pal {
						assert.NotNil(t, c, "token %s[%d] is nil", f.Name, j)
					}
				case reflect.TypeFor[[]theme.GradientStop]():
					stops := v.Field(i).Interface().([]theme.GradientStop)
					require.GreaterOrEqual(t, len(stops), 2, "token %s needs at least two stops", f.Name)
					assert.Equal(t, 0.0, stops[0].Pos, "token %s must start at 0.0", f.Name)
					assert.Equal(t, 1.0, stops[len(stops)-1].Pos, "token %s must end at 1.0", f.Name)
					for j, s := range stops {
						assert.NotNil(t, s.Color, "token %s[%d] has no color", f.Name, j)
						if j > 0 {
							assert.Greater(t, s.Pos, stops[j-1].Pos, "token %s stops must ascend", f.Name)
						}
					}
				}
			}
		})
	}
}

// The two built-ins must be distinct, otherwise the light one silently inherited
// the dark one.
func TestBuiltins_Differ(t *testing.T) {
	assert.NotEqual(t, theme.TeleDark.SurfaceOverlay, theme.TeleLight.SurfaceOverlay)
	assert.Equal(t, "tele-dark", theme.TeleDark.Name)
	assert.Equal(t, "tele-light", theme.TeleLight.Name)
}

// Built-ins resolve by name, and the name is normalized like any other.
func TestBuiltin_ResolvesNormalizedNames(t *testing.T) {
	for _, spelling := range []string{"tele-dark", "tele_dark", "TeleDark", "TELE-DARK"} {
		got, ok := theme.Builtin(spelling)
		require.True(t, ok, "%q must resolve", spelling)
		assert.Equal(t, "tele-dark", got.Name)
	}
	_, ok := theme.Builtin("default")
	assert.False(t, ok, "default names a role, not a theme")
}

// Apply selects the theme in the matching slot.
func TestApply_SelectsSlot(t *testing.T) {
	theme.SetSlots(theme.Slots{Dark: theme.TeleDark, Light: theme.TeleLight})

	theme.Apply(true)
	assert.Equal(t, "tele-dark", theme.T().Name)

	theme.Apply(false)
	assert.Equal(t, "tele-light", theme.T().Name)
}

// A theme may sit in both slots: that is how naming a single theme in the config
// is expressed, and it needs no separate "switching is off" state.
func TestSetSlots_SameThemeInBothSlots(t *testing.T) {
	t.Cleanup(func() { theme.SetSlots(theme.Slots{Dark: theme.TeleDark, Light: theme.TeleLight}) })
	theme.SetSlots(theme.Slots{Dark: theme.TeleDark, Light: theme.TeleDark})

	theme.Apply(false)
	assert.Equal(t, "tele-dark", theme.T().Name, "the light slot holds the same theme")
	theme.Apply(true)
	assert.Equal(t, "tele-dark", theme.T().Name)
}

// SetSlots keeps the background that was last applied, so reloading themes while
// running does not snap the app back to the dark slot.
func TestSetSlots_KeepsAppliedBackground(t *testing.T) {
	t.Cleanup(func() { theme.SetSlots(theme.Slots{Dark: theme.TeleDark, Light: theme.TeleLight}) })

	theme.Apply(false)
	theme.SetSlots(theme.Slots{Dark: theme.TeleDark, Light: theme.TeleLight})
	assert.Equal(t, "tele-light", theme.T().Name)
}

// Before anything is configured the built-ins are already installed, matching
// the root model's assumption that the terminal is dark until it reports.
func TestT_DefaultsToBuiltins(t *testing.T) {
	require.NotNil(t, theme.T())
	assert.NotNil(t, theme.S())
}
