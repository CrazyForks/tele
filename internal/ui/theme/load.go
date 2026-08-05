package theme

import (
	"fmt"
	"image/color"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// baseKey is the one key in a theme file that is not a token: it names the theme
// this one inherits the tokens it does not set from.
const baseKey = "base"

// Origin records where a resolved token came from: the theme in the chain that
// set it, and the text it was written as. Scalar tokens carry their text so a
// dump can re-emit "bright-red" or "240" rather than flattening it to hex, which
// for a palette index would change what the token means.
type Origin struct {
	Theme string
	Raw   string
}

// Resolved is a theme together with where each of its tokens came from. Chain
// lists the themes that contributed, leaf first, ending at the built-in every
// chain roots in. Origins is keyed by token key in file spelling, the same
// strings TokenKeys returns.
type Resolved struct {
	Theme   Theme
	Chain   []string
	Origins map[string]Origin
}

// Loader reads themes from a directory. It is built once, reports what it found
// wrong through Warnings, and resolves any number of names against it.
//
// Nothing it does is fatal. A theme that cannot be read leaves its slot holding
// the built-in and adds a warning, because a config typo must not stand between
// the user and their messages.
type Loader struct {
	dir      string
	files    map[string]string // normalized name -> path
	parsed   map[string]*fileTheme
	warnings []string
}

// fileTheme is one theme file, parsed but not yet resolved against its base.
type fileTheme struct {
	name   string
	base   string
	tokens map[string]tokenValue // by Theme field name
}

// tokenValue is a parsed token ready to be assigned to its field, alongside the
// text it was written as.
type tokenValue struct {
	value reflect.Value
	raw   string
}

// NewLoader indexes the theme files in dir. A missing directory is not an error:
// having no themes of your own is the normal case.
func NewLoader(dir string) *Loader {
	l := &Loader{
		dir:    dir,
		files:  map[string]string{},
		parsed: map[string]*fileTheme{},
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if !os.IsNotExist(err) {
			l.warnf("themes directory %s: %v", dir, err)
		}
		return l
	}

	// Names are matched normalized, so the index is built by listing rather than
	// by opening a path: mine-dark.yml has to be findable as mine_dark.
	raw := map[string]string{} // normalized name -> raw file name, for reporting
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := filepath.Ext(e.Name())
		if ext != ".yml" && ext != ".yaml" {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ext)
		key := normalize(name)

		if _, ok := builtins[key]; ok {
			l.warnf("theme file %s shadows the built-in theme %s and is ignored; rename it", e.Name(), name)
			continue
		}
		// Two files can normalize to one name. Neither is more correct, so the
		// choice is made the only way that is stable across machines.
		if prev, ok := raw[key]; ok {
			winner := min(prev, e.Name())
			l.warnf("theme files %s and %s have the same name; using %s", prev, e.Name(), winner)
			raw[key], l.files[key] = winner, filepath.Join(dir, winner)
			continue
		}
		raw[key] = e.Name()
		l.files[key] = filepath.Join(dir, e.Name())
	}
	return l
}

// Warnings returns the non-fatal problems found so far, in the order they were
// found. Resolve appends to it, so read it after resolving.
func (l *Loader) Warnings() []string { return l.warnings }

// Loaded is the resolution of both slots, with the provenance kept so the
// diagnostics can explain what happened, and the warnings collected along the
// way for the caller to surface.
type Loaded struct {
	Dark, Light Resolved
	Warnings    []string
}

// Slots returns the pair to install.
func (l Loaded) Slots() Slots { return Slots{Dark: l.Dark.Theme, Light: l.Light.Theme} }

// LoadSlots resolves the configured theme names, reading files from dir. An
// empty name means the built-in for that slot. Nothing here fails: a name that
// cannot be resolved leaves its slot on the built-in and adds a warning.
func LoadSlots(dir, darkName, lightName string) Loaded {
	l := NewLoader(dir)
	// Resolved in a fixed order so the warnings read the same way every run.
	dark := l.Resolve(darkName, TeleDark)
	light := l.Resolve(lightName, TeleLight)
	return Loaded{Dark: dark, Light: light, Warnings: l.Warnings()}
}

func (l *Loader) warnf(format string, args ...any) {
	l.warnings = append(l.warnings, fmt.Sprintf(format, args...))
}

// Resolve returns the theme called name, with every token it does not set taken
// from its base, and ultimately from fallback. An empty name resolves to
// fallback itself, which is what an unset config slot means.
func (l *Loader) Resolve(name string, fallback Theme) Resolved {
	if strings.TrimSpace(name) == "" {
		return seed(fallback)
	}

	layers, root, err := l.chain(name)
	if err != nil {
		l.warnf("theme %q: %v; using %s", name, err, fallback.Name)
		return seed(fallback)
	}
	if root == nil {
		root = &fallback
	}

	res := seed(*root)
	// Layers come back leaf first; applying them from the far end means each
	// theme overwrites the one it inherits from.
	for i := len(layers) - 1; i >= 0; i-- {
		apply(&res, layers[i])
	}
	res.Chain = nil
	for _, layer := range layers {
		res.Chain = append(res.Chain, layer.name)
	}
	res.Chain = append(res.Chain, root.Name)
	// The resolved theme is known by the name that was asked for. With no
	// layers the name asked for was the built-in's own, which seed already set.
	if len(layers) > 0 {
		res.Theme.Name = layers[0].name
	}
	return res
}

// chain walks the base links from name outward. It returns the themes to apply
// (leaf first) and the built-in the chain roots in, if it named one explicitly.
func (l *Loader) chain(name string) ([]*fileTheme, *Theme, error) {
	var (
		layers []*fileTheme
		seen   []string
	)
	cur := name
	for {
		key := normalize(cur)
		if slices.Contains(seen, key) {
			return nil, nil, fmt.Errorf("base cycle: %s -> %s", strings.Join(seen, " -> "), cur)
		}
		seen = append(seen, key)

		if b, ok := builtins[key]; ok {
			// A chain may root in either built-in explicitly; only a chain that
			// names none falls back to the one for its slot.
			return layers, &b, nil
		}

		ft, err := l.parse(key)
		if err != nil {
			return nil, nil, err
		}
		layers = append(layers, ft)
		if ft.base == "" {
			return layers, nil, nil
		}
		cur = ft.base
	}
}

// parse reads and decodes one theme file, memoized: a base shared by several
// themes is read once.
func (l *Loader) parse(key string) (*fileTheme, error) {
	if ft, ok := l.parsed[key]; ok {
		return ft, nil
	}
	path, ok := l.files[key]
	if !ok {
		return nil, fmt.Errorf("no such theme; put it in %s", l.dir)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", filepath.Base(path), err)
	}

	ext := filepath.Ext(path)
	ft := &fileTheme{
		name:   strings.TrimSuffix(filepath.Base(path), ext),
		tokens: map[string]tokenValue{},
	}

	for k, v := range doc {
		nk := normalize(k)
		if nk == baseKey {
			s, ok := scalarText(v)
			if !ok {
				l.warnf("theme %s: base must be a theme name", ft.name)
				continue
			}
			ft.base = s
			continue
		}
		field, ok := tokenFields[nk]
		if !ok {
			// An unknown key is a warning rather than an error: a theme written
			// for a later version of tele must keep working here, and a typo must
			// not cost the user their theme.
			l.warnf("theme %s: unknown key %q, ignored", ft.name, k)
			continue
		}
		val, raw, err := parseToken(field, v)
		if err != nil {
			l.warnf("theme %s: %s: %v", ft.name, k, err)
			continue
		}
		ft.tokens[field.Name] = tokenValue{value: val, raw: raw}
	}

	l.parsed[key] = ft
	return ft, nil
}

// apply overwrites the tokens res holds with the ones layer sets.
func apply(res *Resolved, layer *fileTheme) {
	v := reflect.ValueOf(&res.Theme).Elem()
	for fieldName, tv := range layer.tokens {
		v.FieldByName(fieldName).Set(tv.value)
		res.Origins[TokenKey(fieldName)] = Origin{Theme: layer.name, Raw: tv.raw}
	}
}

// seed starts a resolution from a complete theme, recording every token as
// having come from it.
func seed(t Theme) Resolved {
	res := Resolved{Theme: t, Chain: []string{t.Name}, Origins: make(map[string]Origin, len(tokenFields))}
	v := reflect.ValueOf(t)
	for _, f := range tokenFields {
		o := Origin{Theme: t.Name}
		if f.Type == colorType {
			o.Raw = FormatColor(v.FieldByIndex(f.Index).Interface().(color.Color))
		}
		res.Origins[TokenKey(f.Name)] = o
	}
	return res
}

// parseToken converts one YAML value into something assignable to field.
func parseToken(field reflect.StructField, v any) (reflect.Value, string, error) {
	switch field.Type {
	case colorType:
		s, ok := scalarText(v)
		if !ok {
			return reflect.Value{}, "", fmt.Errorf("want a color, got %T", v)
		}
		c, err := ParseColor(s)
		if err != nil {
			return reflect.Value{}, "", err
		}
		if err := rejectNone(field.Name, c); err != nil {
			return reflect.Value{}, "", err
		}
		return reflect.ValueOf(&c).Elem(), s, nil

	case paletteType:
		items, ok := v.([]any)
		if !ok {
			return reflect.Value{}, "", fmt.Errorf("want a list of colors, got %T", v)
		}
		if len(items) == 0 {
			return reflect.Value{}, "", fmt.Errorf("needs at least one color")
		}
		pal := make([]color.Color, 0, len(items))
		for i, item := range items {
			s, ok := scalarText(item)
			if !ok {
				return reflect.Value{}, "", fmt.Errorf("entry %d: want a color, got %T", i, item)
			}
			c, err := ParseColor(s)
			if err != nil {
				return reflect.Value{}, "", fmt.Errorf("entry %d: %w", i, err)
			}
			pal = append(pal, c)
		}
		return reflect.ValueOf(pal), "", nil

	case gradientType:
		stops, err := parseGradient(v)
		if err != nil {
			return reflect.Value{}, "", err
		}
		return reflect.ValueOf(stops), "", nil
	}
	return reflect.Value{}, "", fmt.Errorf("unsupported token type %s", field.Type)
}

// parseGradient reads a ramp: a list of {pos, color} entries covering [0,1].
func parseGradient(v any) ([]GradientStop, error) {
	items, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("want a list of {pos, color} stops, got %T", v)
	}
	if len(items) < 2 {
		return nil, fmt.Errorf("needs at least two stops")
	}
	stops := make([]GradientStop, 0, len(items))
	for i, item := range items {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, fmt.Errorf("stop %d: want {pos, color}, got %T", i, item)
		}
		var (
			stop  GradientStop
			hasP  bool
			hasC  bool
			extra []string
		)
		for k, raw := range m {
			switch normalize(k) {
			case "pos":
				p, ok := numberValue(raw)
				if !ok {
					return nil, fmt.Errorf("stop %d: pos must be a number", i)
				}
				stop.Pos, hasP = p, true
			case "color":
				s, ok := scalarText(raw)
				if !ok {
					return nil, fmt.Errorf("stop %d: color must be a color", i)
				}
				c, err := ParseColor(s)
				if err != nil {
					return nil, fmt.Errorf("stop %d: %w", i, err)
				}
				// A gradient is interpolated, so "none" here would mean fading
				// to black rather than leaving the color alone.
				if isNone(c) {
					return nil, fmt.Errorf("stop %d: none is not a color to interpolate through", i)
				}
				stop.Color, hasC = c, true
			default:
				extra = append(extra, k)
			}
		}
		if !hasP || !hasC {
			return nil, fmt.Errorf("stop %d: needs both pos and color", i)
		}
		if len(extra) > 0 {
			return nil, fmt.Errorf("stop %d: unknown keys %s", i, strings.Join(extra, ", "))
		}
		if stop.Pos < 0 || stop.Pos > 1 {
			return nil, fmt.Errorf("stop %d: pos %v is outside 0..1", i, stop.Pos)
		}
		if i > 0 && stop.Pos <= stops[i-1].Pos {
			return nil, fmt.Errorf("stop %d: pos must increase", i)
		}
		stops = append(stops, stop)
	}
	if stops[0].Pos != 0 {
		return nil, fmt.Errorf("the first stop must be at pos 0")
	}
	if stops[len(stops)-1].Pos != 1 {
		return nil, fmt.Errorf("the last stop must be at pos 1")
	}
	return stops, nil
}

// rejectNone refuses "none" on the tokens that are interpolated rather than
// rendered, where it would silently mean black.
func rejectNone(fieldName string, c color.Color) error {
	if interpolated[fieldName] && isNone(c) {
		return fmt.Errorf("none is not allowed here: this token is interpolated, and would mean black")
	}
	return nil
}

// scalarText renders a YAML scalar as the text a color parser expects. An
// unquoted palette index arrives as an int, which is the natural way to write
// one.
func scalarText(v any) (string, bool) {
	switch t := v.(type) {
	case string:
		return t, true
	case int:
		return strconv.Itoa(t), true
	case int64:
		return strconv.FormatInt(t, 10), true
	}
	return "", false
}

func numberValue(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	}
	return 0, false
}
