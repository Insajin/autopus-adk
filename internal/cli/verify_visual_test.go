package cli

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/insajin/autopus-adk/pkg/design"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteVerifyVisualGate_StrictFailsAfterWritingEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	err := writeVerifyVisualGate(root, []string{"src/components/Button.tsx"}, nil, nil, "desktop", design.Context{Found: true, SourcePath: "DESIGN.md"}, 2, nil, true, "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "strict visual gate failed")
	assert.FileExists(t, filepath.Join(root, ".autopus", "design", "verify", "latest.json"))
}

func TestWriteVerifyVisualGateMergesVisualCriticReport(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	critic := filepath.Join(root, ".autopus", "design", "critic.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(critic), 0o755))
	require.NoError(t, os.WriteFile(critic, []byte(`{"status":"FAIL","findings":[{"severity":"FAIL","category":"overlap","message":"button overlaps"}]}`), 0o644))

	err := writeVerifyVisualGate(root, []string{"src/components/Button.tsx"}, []string{"shot.png"}, nil, "all", design.Context{Found: true, SourcePath: "DESIGN.md"}, 2, nil, true, ".autopus/design/critic.json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "strict visual gate failed")
}

func TestCollectVisualArtifactsRedactsExternalAbsolutePaths(t *testing.T) {
	t.Parallel()

	input := buildPlaywrightJSON([]playwrightSuite{{
		Specs: []playwrightSpec{{Tests: []playwrightTest{{Results: []playwrightTestResult{{Attachments: []playwrightAttachment{
			{Name: "actual", Path: "/tmp/playwright/actual.png"},
			{Name: "expected", Path: "/tmp/playwright/expected.png"},
			{Name: "diff", Path: "/tmp/playwright/diff.png"},
		}}}}}}},
	}})

	artifacts := collectVisualArtifacts(input)
	require.Len(t, artifacts, 3)
	assert.Equal(t, "actual", artifacts[0].Kind)
	assert.Contains(t, artifacts[0].Path, "external:")
	assert.Equal(t, "/tmp/playwright/actual.png", artifacts[0].LocalPath)
}
