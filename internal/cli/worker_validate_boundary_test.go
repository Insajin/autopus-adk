package cli

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// controlplaneImportPath is the module path this test looks for among each
// file's parsed import specs. Using go/parser (ImportsOnly mode) instead of
// a raw text grep for the quoted path avoids two traps: (1) false
// positives, where a file merely quotes the import path as a string literal
// (as this very test does, to build the expected value) would otherwise
// look like a real import; (2) false negatives from a naive substring
// filter like "grep -v pkg/worker", which also matches inside the import
// path text of legitimate external importers. Parsing each file's actual
// import declarations sidesteps both.
const controlplaneImportPath = "github.com/insajin/autopus-adk/pkg/worker/controlplane"

// TestControlplaneExternalImporters_ExactlyWorkerValidateFiles is the S7
// oracle for REQ-009: the set of .go files whose path does NOT start with
// "pkg/worker/" but that import pkg/worker/controlplane must be exactly
// {internal/cli/worker_validate.go, internal/cli/worker_validate_test.go}.
func TestControlplaneExternalImporters_ExactlyWorkerValidateFiles(t *testing.T) {
	repoRoot := moduleRootForTest(t)

	want := map[string]bool{
		"internal/cli/worker_validate.go":      true,
		"internal/cli/worker_validate_test.go": true,
	}

	var got []string
	err := filepath.WalkDir(repoRoot, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "node_modules" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}
		rel, err := filepath.Rel(repoRoot, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if strings.HasPrefix(rel, "pkg/worker/") {
			// Anything under pkg/worker/ is inside the worker module itself,
			// never an "external" importer regardless of import content.
			return nil
		}
		imports, err := parsedImports(path)
		if err != nil {
			return err
		}
		if imports[controlplaneImportPath] {
			got = append(got, rel)
		}
		return nil
	})
	require.NoError(t, err)
	sort.Strings(got)

	gotSet := make(map[string]bool, len(got))
	for _, f := range got {
		gotSet[f] = true
	}
	assert.Equal(t, want, gotSet,
		"external pkg/worker/controlplane importer set drifted from {worker_validate.go, worker_validate_test.go}: got %v", got)
}

// parsedImports returns the set of import paths (unquoted) declared by the
// Go source file at path, parsed via go/parser so string literals elsewhere
// in the file (e.g. a constant quoting an import path for comparison) are
// never mistaken for an actual import declaration.
func parsedImports(path string) (map[string]bool, error) {
	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	imports := make(map[string]bool, len(astFile.Imports))
	for _, imp := range astFile.Imports {
		unquoted := strings.Trim(imp.Path.Value, `"`)
		imports[unquoted] = true
	}
	return imports, nil
}

// moduleRootForTest resolves the repository root from this test file's own
// location so the boundary scan works regardless of the process's working
// directory.
func moduleRootForTest(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	require.True(t, ok, "runtime.Caller failed to resolve test file location")
	// internal/cli/worker_validate_boundary_test.go -> repo root is two levels up.
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}
