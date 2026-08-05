package theme

import (
	"fmt"
	"image/color"
	"reflect"
	"strconv"
	"strings"
)

// Dump renders a resolved theme as a theme file with every token written out.
// It is how a theme author starts: dump what is on screen, then edit it. Because
// nothing is left implicit, the result needs no base.
//
// Scalar tokens come back as they were written where that is known, so a token
// given as a palette index stays an index — flattening it to hex would change
// what it means, since indexes follow the terminal's own palette.
func Dump(r Resolved) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# tele theme, dumped from %s\n", strings.Join(r.Chain, " <- "))
	b.WriteString("# Every token is set, so this file inherits nothing.\n")

	v := reflect.ValueOf(r.Theme)
	group := ""
	for _, f := range tokenOrder {
		if g := leadingWord(f.Name); g != group {
			b.WriteString("\n")
			group = g
		}
		key := TokenKey(f.Name)
		switch f.Type {
		case colorType:
			raw := r.Origins[key].Raw
			if raw == "" {
				raw = FormatColor(v.FieldByIndex(f.Index).Interface().(color.Color))
			}
			fmt.Fprintf(&b, "%s: %s\n", key, quoteColor(raw))
		case paletteType:
			fmt.Fprintf(&b, "%s:\n", key)
			for _, c := range v.FieldByIndex(f.Index).Interface().([]color.Color) {
				fmt.Fprintf(&b, "  - %s\n", quoteColor(FormatColor(c)))
			}
		case gradientType:
			fmt.Fprintf(&b, "%s:\n", key)
			for _, s := range v.FieldByIndex(f.Index).Interface().([]GradientStop) {
				fmt.Fprintf(&b, "  - {pos: %s, color: %s}\n",
					strconv.FormatFloat(s.Pos, 'g', -1, 64), quoteColor(FormatColor(s.Color)))
			}
		}
	}
	return b.String()
}

// Report describes what a slot resolved to and where its tokens came from. It
// answers the two questions a theme file raises that nothing on screen can:
// which theme am I actually running, and which of these colors did I set.
func Report(slot string, r Resolved) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s slot: %s\n", slot, r.Theme.Name)
	fmt.Fprintf(&b, "  chain: %s\n", strings.Join(r.Chain, " <- "))

	bySource := map[string][]string{}
	for _, key := range TokenKeys() {
		src := r.Origins[key].Theme
		bySource[src] = append(bySource[src], key)
	}
	// Reported in chain order rather than alphabetically, so the theme that was
	// asked for comes first and the built-in that filled the gaps comes last.
	for _, src := range r.Chain {
		keys, ok := bySource[src]
		if !ok {
			fmt.Fprintf(&b, "  0 from %s: it sets nothing the themes above it do not\n", src)
			continue
		}
		fmt.Fprintf(&b, "  %d from %s: %s\n", len(keys), src, elide(keys, listCap))
	}
	return b.String()
}

// listCap bounds how many token names a report prints per source. A theme that
// sets a handful of tokens is listed in full, which is the case worth reading;
// the sixty inherited from the built-in are a count, not a list.
const listCap = 12

func elide(keys []string, cap int) string {
	if len(keys) <= cap {
		return strings.Join(keys, ", ")
	}
	return strings.Join(keys[:cap], ", ") + fmt.Sprintf(", and %d more", len(keys)-cap)
}

// quoteColor quotes what YAML would otherwise misread: a leading # starts a
// comment, and a bare number is not a string.
func quoteColor(s string) string {
	if s == "" {
		return `""`
	}
	if strings.HasPrefix(s, "#") {
		return `"` + s + `"`
	}
	if _, err := strconv.Atoi(s); err == nil {
		return `"` + s + `"`
	}
	return s
}

// leadingWord returns the first CamelCase word of a field name, used only to
// group a dump into readable blocks.
func leadingWord(name string) string {
	for i := 1; i < len(name); i++ {
		if name[i] >= 'A' && name[i] <= 'Z' {
			return name[:i]
		}
	}
	return name
}
