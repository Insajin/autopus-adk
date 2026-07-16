package design

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteVisualGateReportWritesRuntimeEvidence(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	report := BuildVisualGateReport(VisualGateInput{
		UIChanged:     []string{"src/components/Button.tsx"},
		Screenshots:   []string{"Button.spec.ts-snapshots/button.png"},
		Viewport:      "all",
		DesignContext: Context{Found: true, SourcePath: "DESIGN.md"},
	})
	path, err := WriteVisualGateReport(root, report)
	require.NoError(t, err)
	assert.Equal(t, filepath.Join(root, ".autopus", "design", "verify", "latest.json"), path)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var decoded VisualGateReport
	require.NoError(t, json.Unmarshal(data, &decoded))
	assert.Equal(t, "WARN", decoded.Verdict)
}
