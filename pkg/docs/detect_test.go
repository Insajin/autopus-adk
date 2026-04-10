package docs

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDetect_GoMod verifies that Go module dependencies are parsed from go.mod.
// Given: a go.mod file with require block
// When: DetectFromGoMod is called
// Then: all non-stdlib dependency module paths are returned
func TestDetect_GoMod(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	gomod := `module example.com/app

go 1.21

require (
	github.com/spf13/cobra v1.9.1
	github.com/stretchr/testify v1.11.1
	gopkg.in/yaml.v3 v3.0.1
)
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(gomod), 0o644))

	libs, err := DetectFromGoMod(filepath.Join(dir, "go.mod"))
	require.NoError(t, err)
	assert.Contains(t, libs, "github.com/spf13/cobra")
	assert.Contains(t, libs, "github.com/stretchr/testify")
	assert.NotContains(t, libs, "fmt", "stdlib must be excluded")
}

// TestDetect_PackageJSON verifies that npm dependencies are parsed from package.json.
// Given: a package.json with dependencies and devDependencies
// When: DetectFromPackageJSON is called
// Then: all dependency package names are returned
func TestDetect_PackageJSON(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pkgjson := `{
  "name": "my-app",
  "dependencies": {
    "express": "^4.18.0",
    "lodash": "^4.17.21"
  },
  "devDependencies": {
    "vitest": "^1.0.0"
  }
}`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(pkgjson), 0o644))

	libs, err := DetectFromPackageJSON(filepath.Join(dir, "package.json"))
	require.NoError(t, err)
	assert.Contains(t, libs, "express")
	assert.Contains(t, libs, "lodash")
	assert.Contains(t, libs, "vitest")
}

// TestDetect_PyProjectToml verifies that Python dependencies are parsed from pyproject.toml.
// Given: a pyproject.toml with dependencies list
// When: DetectFromPyProjectToml is called
// Then: all dependency names are returned without version specifiers
func TestDetect_PyProjectToml(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pyproject := `[project]
name = "my-app"
dependencies = [
  "requests>=2.28.0",
  "fastapi[all]>=0.100.0",
  "pydantic>=2.0",
]
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte(pyproject), 0o644))

	libs, err := DetectFromPyProjectToml(filepath.Join(dir, "pyproject.toml"))
	require.NoError(t, err)
	assert.Contains(t, libs, "requests")
	assert.Contains(t, libs, "fastapi")
	assert.Contains(t, libs, "pydantic")
}

// TestDetect_FiltersStdLib verifies that standard library modules are excluded from results.
// Given: a file listing that includes stdlib modules
// When: FilterStdLib is called
// Then: stdlib names are removed and only third-party names remain
func TestDetect_FiltersStdLib(t *testing.T) {
	t.Parallel()

	input := []string{"fmt", "os", "cobra", "path", "testify", "net/http"}
	result := FilterStdLib("go", input)

	assert.Contains(t, result, "cobra")
	assert.Contains(t, result, "testify")
	assert.NotContains(t, result, "fmt")
	assert.NotContains(t, result, "os")
	assert.NotContains(t, result, "net/http")
}

// TestDetect_FromSPEC verifies that library names are extracted from SPEC or plan.md text.
// Given: a text blob referencing library names
// When: DetectFromText is called
// Then: recognized library names are extracted
func TestDetect_FromSPEC(t *testing.T) {
	t.Parallel()

	specText := `## Requirements
Use cobra for CLI parsing and viper for configuration.
The system should also integrate with testify for tests.
Standard libraries like fmt and os are already available.`

	libs := DetectFromText("go", specText)
	assert.Contains(t, libs, "cobra")
	assert.Contains(t, libs, "viper")
	assert.Contains(t, libs, "testify")
	assert.NotContains(t, libs, "fmt")
	assert.NotContains(t, libs, "os")
}
