package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/orchestra"
	"github.com/insajin/autopus-adk/pkg/spec"
)

func TestRunSpecReviewLoop_StaticContractFindingBlocksProviderPass(t *testing.T) {
	dir := t.TempDir()
	specID := "SPEC-STATIC-CONTRACT-001"
	specDir := scaffoldReviewSpec(t, dir, specID)
	writeStaticContractDriftFixture(t, specDir)
	doc, err := spec.Load(specDir)
	require.NoError(t, err)

	origRunner := specReviewRunOrchestra
	specReviewRunOrchestra = func(_ context.Context, _ orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
		return &orchestra.OrchestraResult{Responses: []orchestra.ProviderResponse{
			{Provider: "claude", Output: `{"verdict":"PASS","summary":"ok","findings":[]}`},
			{Provider: "codex", Output: `{"verdict":"PASS","summary":"ok","findings":[]}`},
			{Provider: "gemini", Output: `{"verdict":"PASS","summary":"ok","findings":[]}`},
		}}, nil
	}
	defer func() { specReviewRunOrchestra = origRunner }()

	params := reviewLoopParams(specID, specDir)
	result, err := runSpecReviewLoop(params, doc, nil)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, spec.VerdictRevise, result.Verdict)

	findings, err := spec.LoadFindings(specDir)
	require.NoError(t, err)
	require.Len(t, findings, 1)
	assert.Equal(t, "spec-static", findings[0].Provider)
	assert.Equal(t, spec.FindingStatusOpen, findings[0].Status)
	assert.Contains(t, findings[0].Description, "missing_event_kind_values")
	assert.Contains(t, findings[0].Description, "invalid_stance_value")
}

func writeStaticContractDriftFixture(t *testing.T, specDir string) {
	t.Helper()
	files := map[string]string{
		"spec.md": `---
id: SPEC-STATIC-CONTRACT-001
title: Static Contract Drift
status: draft
---

## Requirements

### REQ-001

- Type: event-driven
- Priority: Must
- Statement: WHEN a council event is persisted, THE SYSTEM SHALL retain only allowlisted fields.

## Retained-Field Allowlist

- ` + "`ts`" + ` — timestamp.
- ` + "`event_kind`" + ` — "session_open" | "session_close" | "trust_boundary_violation".
- ` + "`stance`" + ` — stance.
- ` + "`redaction_status`" + ` — "redacted".

### Per-event-kind Non-null Column Table

| event_kind | event-specific non-null columns |
|------------|---------------------------------|
| ` + "`session_open`" + ` | none |
| ` + "`session_close`" + ` | none |
| ` + "`trust_boundary_violation`" + ` | ` + "`reason`" + ` |
`,
		"acceptance.md": `# Acceptance

### AC-001

Given a role emits an invalid stance
When the value is validated
Then an ` + "`event_kind=\"invalid_stance_value\"`" + ` row is appended with {attempted_value:"agree", expected_pool:["찬성","조건부","반대","blocker"]}.
`,
		"plan.md":     "# Plan\n",
		"research.md": "# Research\n",
	}
	for name, body := range files {
		require.NoError(t, os.WriteFile(filepath.Join(specDir, name), []byte(body), 0o644))
	}
}
