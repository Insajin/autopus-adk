package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestDoctorJSONReportAppendPlatformValidationFinding_CarriesFileIntoPayloadAndDetail(t *testing.T) {
	t.Parallel()

	report := doctorJSONReport{status: jsonStatusOK}
	payload := doctorPlatformPayload{Name: "codex"}
	report.appendPlatformValidationFinding("codex", &payload, adapter.ValidationError{
		File:    filepath.Join(".codex", "skills", "auto-plan.md"),
		Message: "obsolete Codex managed surface가 남아 있음",
		Level:   "error",
	})

	require.Len(t, payload.Messages, 1)
	assert.Equal(t, "error", payload.Messages[0].Level)
	assert.Equal(t, ".codex/skills/auto-plan.md", payload.Messages[0].File)

	require.Len(t, report.checks, 1)
	assert.Equal(t, "doctor.platform.codex", report.checks[0].ID)
	assert.Equal(t, "error", report.checks[0].Severity)
	assert.Equal(t, "fail", report.checks[0].Status)
	assert.Equal(t,
		"codex: obsolete Codex managed surface가 남아 있음 (.codex/skills/auto-plan.md)",
		report.checks[0].Detail)
	assert.Equal(t, jsonStatusWarn, report.status)

	encoded, err := json.Marshal(payload.Messages[0])
	require.NoError(t, err)
	assert.JSONEq(
		t,
		`{"level":"error","message":"obsolete Codex managed surface가 남아 있음","file":".codex/skills/auto-plan.md"}`,
		string(encoded),
	)
}

// Two findings that share a message must stay distinguishable: the 80 identical
// obsolete-surface errors are only actionable because each check names its path.
func TestDoctorJSONReportAppendPlatformValidationFinding_DistinguishesRepeatedMessages(t *testing.T) {
	t.Parallel()

	report := doctorJSONReport{status: jsonStatusOK}
	payload := doctorPlatformPayload{Name: "codex"}
	for _, rel := range []string{".codex/prompts", ".codex/rules/autopus/branding.md"} {
		report.appendPlatformValidationFinding("codex", &payload, adapter.ValidationError{
			File: rel, Message: "obsolete Codex managed surface가 남아 있음", Level: "error",
		})
	}

	require.Len(t, report.checks, 2)
	assert.NotEqual(t, report.checks[0].Detail, report.checks[1].Detail)
	assert.Contains(t, report.checks[0].Detail, "(.codex/prompts)")
	assert.Contains(t, report.checks[1].Detail, "(.codex/rules/autopus/branding.md)")
}

func TestDoctorJSONReportAppendPlatformValidationFinding_PathlessFindingKeepsLegacyWording(t *testing.T) {
	t.Parallel()

	report := doctorJSONReport{status: jsonStatusOK}
	payload := doctorPlatformPayload{Name: "claude-code"}
	report.appendPlatformValidationFinding("claude-code", &payload, adapter.ValidationError{
		Message: "settings.json이 유효하지 않음",
		Level:   "warn",
	})

	require.Len(t, payload.Messages, 1)
	assert.Empty(t, payload.Messages[0].File)

	require.Len(t, report.checks, 1)
	assert.Equal(t, "claude-code: settings.json이 유효하지 않음", report.checks[0].Detail)
	assert.NotContains(t, report.checks[0].Detail, "()")
	assert.Equal(t, "warn", report.checks[0].Status)

	encoded, err := json.Marshal(payload.Messages[0])
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), `"file"`)
	decoded := map[string]any{}
	require.NoError(t, json.Unmarshal(encoded, &decoded))
	_, present := decoded["file"]
	assert.False(t, present, "omitempty must drop the file key for pathless findings")
}

func TestDoctorJSONReportAppendPlatformValidationFinding_BlankFileNormalizesToNoPath(t *testing.T) {
	t.Parallel()

	report := doctorJSONReport{status: jsonStatusOK}
	payload := doctorPlatformPayload{Name: "opencode"}
	report.appendPlatformValidationFinding("opencode", &payload, adapter.ValidationError{
		File: "   ", Message: "무언가 어긋남",
	})

	require.Len(t, payload.Messages, 1)
	assert.Empty(t, payload.Messages[0].File)
	// Empty Level normalizes to info, matching the pre-existing precedent.
	assert.Equal(t, "info", payload.Messages[0].Level)

	require.Len(t, report.checks, 1)
	assert.Equal(t, "opencode: 무언가 어긋남", report.checks[0].Detail)
	assert.Equal(t, "pass", report.checks[0].Status)
	assert.Equal(t, jsonStatusOK, report.status)

	encoded, err := json.Marshal(payload.Messages[0])
	require.NoError(t, err)
	assert.NotContains(t, string(encoded), `"file"`)
}

func TestDoctorValidationFile_NormalizesRepoRelativeSlashForm(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "", doctorValidationFile(""))
	assert.Equal(t, "", doctorValidationFile("  \t "))
	assert.Equal(t, ".codex/skills/auto-plan.md",
		doctorValidationFile(filepath.Join(".codex", "skills", "auto-plan.md")))
	assert.Equal(t, ".claude/skills/autopus/review.md",
		doctorValidationFile("./.claude/skills/autopus/review.md"))
}

// The plain-text renderer is the surface operators actually read, and it used
// to compose its own "<platform>: <message>" line, which is why identical
// obsolete-surface errors printed once per stale file with no path.
func TestRunDoctorText_NamesObsoleteSurfacePaths(t *testing.T) {
	dir := t.TempDir()
	cfg := config.DefaultFullConfig("doctor-paths")
	cfg.Platforms = []string{"codex"}
	require.NoError(t, config.Save(dir, cfg))

	// Codex validation returns early when AGENTS.md is unreadable, so the
	// fixture must supply it to reach the obsolete-surface findings.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte("# agents\n"), 0o644))

	stale := filepath.Join(dir, ".codex", "skills")
	require.NoError(t, os.MkdirAll(stale, 0o755))
	for _, name := range []string{"retired-one.md", "retired-two.md"} {
		require.NoError(t, os.WriteFile(filepath.Join(stale, name), []byte("legacy\n"), 0o644))
	}

	var stdout bytes.Buffer
	cmd := &cobra.Command{}
	cmd.SetOut(&stdout)
	require.NoError(t, runDoctorText(cmd, doctorOptions{dir: dir}))

	rendered := stdout.String()
	assert.Contains(t, rendered, ".codex/skills/retired-one.md")
	assert.Contains(t, rendered, ".codex/skills/retired-two.md")
}
