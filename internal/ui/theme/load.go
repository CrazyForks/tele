package theme

import (
	"errors"
	"fmt"
	"image/color"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/sorokin-vladimir/tele/themes"
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

// Source is the tier a theme in a chain was read from. It is a fact about this
// machine rather than about the theme: the same name is bundled on a fresh
// install and a file the moment the user writes one.
type Source string

const (
	// SourceBuiltin is tele-dark or tele-light: compiled in and unshadowable.
	SourceBuiltin Source = "built-in"
	// SourceBundled is one of the ported palettes shipped inside the binary.
	SourceBundled Source = "bundled"
	// SourceFile is a theme read from the themes directory. Named after where it
	// was read rather than who wrote it, which is what a report can show.
	SourceFile Source = "file"
)

// Resolved is a theme together with where each of its tokens came from. Chain
// lists the themes that contributed, leaf first, ending at the built-in every
// chain roots in. Origins is keyed by token key in file spelling, the same
// strings TokenKeys returns. Sources says which tier each name in Chain came
// from, and Shadows names the themes in the chain that replaced a bundled theme
// of the same name — invisible on screen, and the first thing to say when a
// theme is not the one the user expected.
//
// Findings is what the audit had to say about the result, and is empty for a
// theme that claims no canvas. It belongs here for the same reason Origins does:
// both are facts about this resolution rather than about the theme, and both are
// only knowable once the chain has been walked.
type Resolved struct {
	Theme    Theme
	Chain    []string
	Sources  map[string]Source
	Shadows  []string
	Origins  map[string]Origin
	Findings []Finding
}

// Loader reads themes from an ordered list of sources. It is built once,
// reports what it found wrong through Warnings, and resolves any number of names
// against it.
//
// The order is the precedence, and it belongs here rather than to the caller: a
// name is looked for in the themes directory first and among the bundled
// palettes second, so a file replaces a bundled theme of the same name. The two
// built-ins are not a source — they are checked before any of them and cannot be
// shadowed at all.
//
// Nothing it does is fatal. A theme that cannot be read leaves its slot holding
// the built-in and adds a warning, because a config typo must not stand between
// the user and their messages.
type Loader struct {
	dir      string
	sources  []source
	index    map[string]indexed // normalized name -> where to read it
	shadowed map[string]bool    // normalized name -> it replaced a bundled theme
	parsed   map[string]*fileTheme
	warnings []string
}

// source is one place themes are read from, as a file system so that the
// directory on disk and the copy inside the binary are read by one code path.
type source struct {
	kind Source
	fsys fs.FS
}

// indexed is a theme found by name but not yet read.
type indexed struct {
	src  int    // index into sources, which is also its precedence
	path string // within that source's file system
	name string // the spelling the theme is known by, without the extension
}

// fileTheme is one theme file, parsed but not yet resolved against its base.
type fileTheme struct {
	name    string
	base    string
	source  Source
	shadows bool                  // it took the name of a bundled theme
	tokens  map[string]tokenValue // by Theme field name
}

// tokenValue is a parsed token ready to be assigned to its field, alongside the
// text it was written as.
type tokenValue struct {
	value reflect.Value
	raw   string
}

// NewLoader indexes the themes in dir and the ones bundled in the binary. A
// missing directory is not an error: having no themes of your own is the normal
// case, and the bundled palettes resolve without it.
func NewLoader(dir string) *Loader {
	l := &Loader{
		dir:      dir,
		index:    map[string]indexed{},
		shadowed: map[string]bool{},
		parsed:   map[string]*fileTheme{},
	}
	// Precedence order. An empty path is not a directory anyone meant, and
	// os.DirFS("") would read from the process's own root.
	if dir != "" {
		l.sources = append(l.sources, source{kind: SourceFile, fsys: os.DirFS(dir)})
	}
	l.sources = append(l.sources, source{kind: SourceBundled, fsys: themes.FS})

	for i, s := range l.sources {
		l.indexSource(i, s)
	}
	return l
}

// indexSource lists one source and records every theme in it that no
// higher-precedence source has already claimed.
func (l *Loader) indexSource(i int, s source) {
	entries, err := fs.ReadDir(s.fsys, ".")
	if err != nil {
		// A themes directory that is not there is the normal case; one that is
		// there and unreadable is worth saying out loud. The bundled source is
		// compiled in and cannot fail either way.
		if s.kind == SourceFile && !errors.Is(err, fs.ErrNotExist) {
			l.warnf("themes directory %s: %v", l.dir, err)
		}
		return
	}

	// Names are matched normalized, so the index is built by listing rather than
	// by opening a path: mine-dark.yml has to be findable as mine_dark.
	raw := map[string]string{} // normalized name -> raw file name, within this source
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
		// Sources are indexed in precedence order, so a name another source has
		// already claimed outranks this one, and nothing here will be read.
		// Replacing a bundled theme is a legitimate thing to do and is not warned
		// about, but it is remembered: it is invisible on screen, and
		// --theme-check has to be able to say it.
		if ent, taken := l.index[key]; taken && ent.src != i {
			if s.kind == SourceBundled {
				l.shadowed[key] = true
			}
			continue
		}
		// Two files in one source can normalize to one name. Neither is more
		// correct, so the choice is made the only way that is stable across
		// machines.
		if prev, ok := raw[key]; ok {
			winner := min(prev, e.Name())
			l.warnf("theme files %s and %s have the same name; using %s", prev, e.Name(), winner)
			raw[key] = winner
			l.index[key] = indexed{src: i, path: winner, name: strings.TrimSuffix(winner, filepath.Ext(winner))}
			continue
		}
		raw[key] = e.Name()
		l.index[key] = indexed{src: i, path: e.Name(), name: name}
	}
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

// warnf records a problem, ignoring one it has already recorded. The repeat is
// not hypothetical: putting one theme in both slots is the configuration the
// documentation recommends, and it resolves that theme twice, so everything
// wrong with it would be reported twice. Every warning names the theme it is
// about, so identical text is the same problem with the same theme.
func (l *Loader) warnf(format string, args ...any) {
	text := fmt.Sprintf(format, args...)
	if slices.Contains(l.warnings, text) {
		return
	}
	l.warnings = append(l.warnings, text)
}

// Resolve returns the theme called name, with every token it does not set taken
// from its base, and ultimately from fallback. An empty name resolves to
// fallback itself, which is what an unset config slot means.
func (l *Loader) Resolve(name string, fallback Theme) Resolved {
	if strings.TrimSpace(name) == "" {
		return l.audit(seed(fallback))
	}

	layers, root, err := l.chain(name)
	if err != nil {
		l.warnf("theme %q: %v; using %s", name, err, fallback.Name)
		return l.audit(seed(fallback))
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
	res.Chain, res.Shadows = nil, nil
	for _, layer := range layers {
		res.Chain = append(res.Chain, layer.name)
		res.Sources[layer.name] = layer.source
		if layer.shadows {
			res.Shadows = append(res.Shadows, layer.name)
		}
	}
	res.Chain = append(res.Chain, root.Name)
	// The resolved theme is known by the name that was asked for. With no
	// layers the name asked for was the built-in's own, which seed already set.
	if len(layers) > 0 {
		res.Theme.Name = layers[0].name
	}
	// Checked here, on the resolution, rather than on each layer as it is
	// applied: a chain may set a token in one layer and the token it depends on
	// in another, and only the result says whether the requirement is met.
	for _, token := range enforceDependencies(&res.Theme) {
		l.warnf("%s", dependencyWarning(res.Theme.Name, token))
		// The dump has to show what is in effect, not what was asked for, or
		// dumping a refused theme would reproduce the file that was refused.
		res.Origins[TokenKey(token)] = Origin{Theme: res.Theme.Name, Raw: "none"}
	}
	return l.audit(res)
}

// audit judges the finished resolution and summarises what it found. It runs
// after the dependencies have been enforced, on the theme that will actually be
// installed: a canvas that was refused is not a canvas anything is drawn on, and
// measuring against it would report a screen nobody will see.
func (l *Loader) audit(res Resolved) Resolved {
	res.Findings = Audit(res.Theme)
	if len(res.Findings) > 0 {
		l.warnf("%s", auditWarning(res.Theme.Name, res.Findings))
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
	ent, ok := l.index[key]
	if !ok {
		return nil, fmt.Errorf("no such theme; put it in %s", l.dir)
	}
	data, err := fs.ReadFile(l.sources[ent.src].fsys, ent.path)
	if err != nil {
		return nil, err
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", ent.path, err)
	}

	ft := &fileTheme{
		name:    ent.name,
		source:  l.sources[ent.src].kind,
		shadows: l.shadowed[key],
		tokens:  map[string]tokenValue{},
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
	// Every resolution roots in a built-in, so that is the one source seed knows
	// and the only one it can be right about; the layers name their own.
	res := Resolved{
		Theme:   t,
		Chain:   []string{t.Name},
		Sources: map[string]Source{t.Name: SourceBuiltin},
		Origins: make(map[string]Origin, len(tokenFields)),
	}
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
