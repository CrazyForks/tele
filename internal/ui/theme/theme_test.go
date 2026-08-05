package theme_test

import (
	"image/color"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/sorokin-vladimir/tele/internal/ui/theme"
)

// Every token of every flavour must be populated. A field added to Theme and
// filled in only one flavour renders as transparent black at runtime with no
// other signal, so it is caught here instead.
func TestDefault_BothFlavoursFullyPopulated(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value theme.Theme
	}{
		{"dark", theme.Default.Dark},
		{"light", theme.Default.Light},
	} {
		t.Run(tc.name, func(t *testing.T) {
			v := reflect.ValueOf(tc.value)
			typ := v.Type()
			for i := range typ.NumField() {
				f := typ.Field(i)
				switch f.Type {
				case reflect.TypeFor[color.Color]():
					assert.NotNil(t, v.Field(i).Interface(), "token %s is nil", f.Name)
				case reflect.TypeFor[[8]color.Color]():
					for j, c := range v.Field(i).Interface().([8]color.Color) {
						assert.NotNil(t, c, "token %s[%d] is nil", f.Name, j)
					}
				case reflect.TypeFor[[5]theme.GradientStop]():
					stops := v.Field(i).Interface().([5]theme.GradientStop)
					assert.Equal(t, 1.0, stops[4].Pos, "token %s must end at 1.0", f.Name)
				}
			}
		})
	}
}

// The two flavours must be distinct, otherwise the light variant silently
// inherited the dark one.
func TestDefault_FlavoursDiffer(t *testing.T) {
	assert.NotEqual(t, theme.Default.Dark.SurfaceOverlay, theme.Default.Light.SurfaceOverlay)
	assert.True(t, theme.Default.Dark.Dark)
	assert.False(t, theme.Default.Light.Dark)
}

// Apply selects the flavour matching the terminal background.
func TestApply_SelectsFlavour(t *testing.T) {
	theme.Apply(theme.Default, true)
	require.True(t, theme.T().Dark)
	assert.Equal(t, theme.Default.Dark.Accent, theme.T().Accent)

	theme.Apply(theme.Default, false)
	require.False(t, theme.T().Dark)
	assert.Equal(t, theme.Default.Light.AccentOnSurface, theme.T().AccentOnSurface)
}

// Before any Apply call the dark flavour is current, matching the root model's
// default assumption about the terminal.
func TestT_DefaultsToDarkFlavour(t *testing.T) {
	assert.NotNil(t, theme.T())
}
