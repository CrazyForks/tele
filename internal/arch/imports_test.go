// Package arch holds tests about the shape of the codebase rather than its
// behaviour: boundaries that are easy to state and easy to erode.
package arch_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const storePkg = "github.com/sorokin-vladimir/tele/internal/store"

// storeImportAllowlist names the files under internal/ui that may still import
// internal/store, and the issue that removes each one. A client renders from
// projections (#194); reaching into the persistence package is how that
// boundary erodes, so the exceptions are enumerated rather than tolerated.
//
// root.go is the load-bearing entry: it holds the store.Store field every other
// UI file reaches the store through. When #198 deletes that field, every
// remaining m.st call site stops compiling, which is the point.
var storeImportAllowlist = map[string]string{
	"root.go": "#193, #195, #196, #198 — optimistic writes still go through m.st",
}

func TestUIDoesNotImportStore(t *testing.T) {
	fset := token.NewFileSet()
	for _, path := range goFilesUnder(t, filepath.Join("..", "ui")) {
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		require.NoError(t, err)
		for _, imp := range f.Imports {
			if strings.Trim(imp.Path.Value, `"`) != storePkg {
				continue
			}
			if _, allowed := storeImportAllowlist[filepath.Base(path)]; !allowed {
				assert.Fail(t, "the UI must not read domain state",
					"%s imports internal/store; render from projections instead (#194)", path)
			}
		}
	}
}

func TestStoreImportAllowlistHasNoStaleEntries(t *testing.T) {
	// A stale entry is worse than no list: it silently re-permits an import the
	// issue that owned it has already removed.
	fset := token.NewFileSet()
	for base := range storeImportAllowlist {
		path := filepath.Join("..", "ui", base)
		f, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
		require.NoError(t, err, "allowlisted file %s no longer exists", base)
		found := false
		for _, imp := range f.Imports {
			if strings.Trim(imp.Path.Value, `"`) == storePkg {
				found = true
			}
		}
		assert.True(t, found, "%s no longer imports internal/store — drop it from the allowlist", base)
	}
}

// goFilesUnder collects every non-test Go source under root, including
// subpackages: components and screens are as much a client as root is.
func goFilesUnder(t *testing.T, root string) []string {
	t.Helper()
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		out = append(out, path)
		return nil
	})
	require.NoError(t, err)
	require.NotEmpty(t, out, "no UI sources found — the walk root is wrong")
	return out
}
