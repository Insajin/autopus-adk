package run

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The evidence chain must not depend on how --project-dir was spelled. An
// absolute project dir used to produce absolute refs that private-path
// redaction then replaced with "", so the release index recorded no evidence
// for lanes that actually ran.
func TestExecutePublishesProjectRelativeRefsForEveryProjectDirSpelling(t *testing.T) {
	absolute := fixtureGoProject(t, true)
	relative := relativeToWorkingDir(t, fixtureGoProject(t, true))
	tmp := fixtureGoProjectUnder(t, "/tmp")
	symlinked := fixtureGoProjectUnder(t, os.TempDir())

	spellings := map[string]string{
		"absolute users path": absolute,
		"relative":            relative,
		"tmp":                 tmp,
		"symlinked tmpdir":    symlinked,
	}
	for name, projectDir := range spellings {
		projectDir := projectDir
		t.Run(name, func(t *testing.T) {
			result, err := Execute(Options{ProjectDir: projectDir, Lane: "fast"})
			require.NoError(t, err)

			require.NotEmpty(t, result.RunIndexPath)
			assert.False(t, filepath.IsAbs(result.RunIndexPath), result.RunIndexPath)
			assert.NotContains(t, result.RunIndexPath, RedactedLocalPath)
			assert.True(t, strings.HasPrefix(result.RunIndexPath, ".autopus/qa/runs/"), result.RunIndexPath)
			assert.FileExists(t, filepath.Join(projectDir, result.RunIndexPath))

			require.Len(t, result.ManifestPaths, 1)
			require.NotEmpty(t, result.ManifestPaths[0])
			assert.False(t, filepath.IsAbs(result.ManifestPaths[0]), result.ManifestPaths[0])
			assert.FileExists(t, filepath.Join(projectDir, result.ManifestPaths[0]))

			require.Len(t, result.AdapterResults, 1)
			assert.False(t, filepath.IsAbs(result.AdapterResults[0].QAMESHManifestPath))
			assert.False(t, filepath.IsAbs(result.OutputRoot))

			// The persisted index carries the same relative refs; a consumer
			// resolving them against the workspace root must find the files.
			var index Index
			body, readErr := os.ReadFile(filepath.Join(projectDir, result.RunIndexPath))
			require.NoError(t, readErr)
			require.NoError(t, json.Unmarshal(body, &index))
			require.Len(t, index.ManifestPaths, 1)
			assert.Equal(t, result.ManifestPaths[0], index.ManifestPaths[0])
			assert.FileExists(t, filepath.Join(projectDir, index.ManifestPaths[0]))
			assert.NotContains(t, string(body), projectDir)
		})
	}
}

// A path genuinely outside the project cannot be made relative. It must collapse
// to a visible sentinel, never to the empty string.
func TestPublicProjectPathNeverYieldsEmptyForANonEmptyPath(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	for _, path := range []string{
		"/etc/passwd",
		"/Users/alice/other-project/x",
		"https://example.test/a",
		filepath.Join(filepath.Dir(root), "sibling", "run-index.json"),
	} {
		got := PublicProjectPath(root, path)
		assert.NotEmpty(t, got, path)
		assert.Equal(t, RedactedLocalPath, got, path)
	}
	assert.Empty(t, PublicProjectPath(root, ""))
}

// The sentinel has to survive downstream ref validation, which rejects shell
// metacharacters. `$PROJECT_ROOT` would be refused even spelled literally.
func TestRedactedLocalPathHasNoShellMetacharacters(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, RedactedLocalPath)
	assert.False(t, strings.ContainsAny(RedactedLocalPath, "\x00\r\n\t:;&|$`<>"), RedactedLocalPath)
}

// projectPath resolves a published ref the way a consumer does. Execute returns
// project-root-relative refs, so a test must join them with the project dir
// rather than treat them as paths from its own working directory.
func projectPath(dir, ref string) string {
	return filepath.Join(dir, ref)
}

func relativeToWorkingDir(t *testing.T, dir string) string {
	t.Helper()
	cwd, err := os.Getwd()
	require.NoError(t, err)
	rel, err := filepath.Rel(cwd, dir)
	require.NoError(t, err)
	return rel
}

func fixtureGoProjectUnder(t *testing.T, parent string) string {
	t.Helper()
	dir, err := os.MkdirTemp(parent, "qa-run-refs-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	writeGoProjectFixture(t, dir, true)
	return dir
}
