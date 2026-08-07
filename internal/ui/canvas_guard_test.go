package ui_test

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// A theme may paint the canvas, and the colour cannot be applied by wrapping the
// composed view: a line holding a coloured run carries an SGR reset in the
// middle of it, and anything wrapped around that loses the background from the
// reset onward. So the canvas is baked into every cell at the point the cell is
// created.
//
// That makes every emitter of cells a place the canvas can be forgotten, and a
// forgotten one is a hole visible only to whoever happens to be looking at the
// right screen state. There are two kinds of emitter, and this refuses the bare
// form of both, so the rule is enforced by the build rather than remembered:
//
//   - a style, which must come from theme.NewStyle;
//   - a run of spaces padding a row out to a width, which must come from
//     theme.Pad or theme.PadTo.
//
// Tests are exempt: a style built to assert something never reaches a screen.
//
// Not every bare form is wrong. Spaces that end up inside a string a style later
// renders are already painted by that style, and painting them again would put
// an escape in the middle of text lipgloss is about to measure. Such a site opts
// out with a canvasOK comment on the line or the line above, which says why —
// the point of the rule is that every exception is written down, not that there
// are none.

// guardRoot is the tree the rule covers, relative to this file.
const guardRoot = "."

// guardExempt is the package the rule cannot apply to, because it is where the
// replacements are defined.
var guardExempt = filepath.Join("theme")

// canvasOK marks a site the rule does not apply to. It must be followed by a
// reason: an exception nobody had to justify is the discipline this replaces.
const canvasOK = "canvas:ok"

func TestCanvas_NoBareStyleConstructor(t *testing.T) {
	found := scanGuarded(t, func(call *ast.CallExpr) string {
		if isSelector(call.Fun, "lipgloss", "NewStyle") {
			return "lipgloss.NewStyle(): use theme.NewStyle(), which carries the canvas"
		}
		return ""
	})
	require.Empty(t, found, "a style built outside theme.NewStyle leaves a hole in the canvas")
}

func TestCanvas_NoBareSpacePadding(t *testing.T) {
	found := scanGuarded(t, func(call *ast.CallExpr) string {
		if !isSelector(call.Fun, "strings", "Repeat") || len(call.Args) == 0 {
			return ""
		}
		if !isSpaceLiteral(call.Args[0]) {
			return "" // repeating a border rune is not padding
		}
		return `strings.Repeat(" ", n): use theme.Pad(n) or theme.PadTo(w, width)`
	})
	require.Empty(t, found, "padding emitted as bare spaces leaves a hole in the canvas")
}

// scanGuarded parses every non-test Go file under the guarded tree and reports
// what report names, as "file:line: message".
func scanGuarded(t *testing.T, report func(*ast.CallExpr) string) []string {
	t.Helper()

	var found []string
	fset := token.NewFileSet()

	err := filepath.WalkDir(guardRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != guardRoot && filepath.Base(path) == guardExempt {
				return filepath.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".go" || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if err != nil {
			return err
		}
		exempt := exemptLines(fset, file)
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			pos := fset.Position(call.Pos())
			if exempt[pos.Line] || exempt[pos.Line-1] {
				return true
			}
			if msg := report(call); msg != "" {
				found = append(found, pos.String()+": "+msg)
			}
			return true
		})
		return nil
	})
	require.NoError(t, err)
	return found
}

// exemptLines returns the lines carrying a canvasOK marker with a reason after
// it. A bare marker is not an exemption: it exempts nothing and shows up as the
// violation it was meant to silence, which is the only way to keep the reason
// from becoming optional.
func exemptLines(fset *token.FileSet, file *ast.File) map[int]bool {
	lines := map[int]bool{}
	for _, group := range file.Comments {
		for _, c := range group.List {
			i := strings.Index(c.Text, canvasOK)
			if i < 0 {
				continue
			}
			if strings.TrimSpace(c.Text[i+len(canvasOK):]) == "" {
				continue
			}
			lines[fset.Position(c.Pos()).Line] = true
		}
	}
	return lines
}

// isSelector reports whether expr is the call pkg.name, matching on the package
// identifier as written rather than on the import path: a file that aliases the
// import is a file that has been thought about.
func isSelector(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	ident, ok := sel.X.(*ast.Ident)
	return ok && ident.Name == pkg
}

// isSpaceLiteral reports whether expr is a string literal of nothing but spaces.
// A single space is the padding case; anything else is drawing something.
func isSpaceLiteral(expr ast.Expr) bool {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING {
		return false
	}
	s, err := strconv.Unquote(lit.Value)
	if err != nil || s == "" {
		return false
	}
	return strings.TrimLeft(s, " ") == ""
}
