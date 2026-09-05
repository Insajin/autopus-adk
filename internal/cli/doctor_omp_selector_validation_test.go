package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOMPDoctorSelectorCollection_ProjectsRejectedAgentsToTextAndJSON(t *testing.T) {
	skipWithoutPOSIXShellDoctorOMP(t)
	root := t.TempDir()
	agentDir := filepath.Join(root, ".omp", "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "malformed.md"),
		[]byte("---\nmodel: [not-a-scalar]\n---\n# agent\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "oversized.md"),
		[]byte("---\nmodel: sonnet\n---\n"+strings.Repeat("x", ompDoctorAgentMaxSize)), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "bad-model.md"),
		[]byte("---\nmodel: provider//secret\n---\n# agent\n"), 0o600))
	// SPEC-OMP-005 role references are valid selectors, not malformed models.
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "role-ref.md"),
		[]byte("---\nmodel: '@autopus_executor'\nthinking: xhigh\n---\n# agent\n"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(agentDir, "bad-role-ref.md"),
		[]byte("---\nmodel: '@auto/pus//x'\n---\n# agent\n"), 0o600))

	installDoctorOMPFixture(t)
	checks := probeAndProjectOMPDoctorChecks(context.Background(), root, nil)
	wantReasons := []string{"frontmatter_malformed", "agent_oversized", "model_malformed"}
	var text bytes.Buffer
	renderOMPDoctorChecksText(&text, checks)
	encoded, err := json.Marshal(sanitizeJSONChecks(checks))
	require.NoError(t, err)
	for _, reason := range wantReasons {
		check := ompDoctorCheckContaining(t, checks, reason)
		assert.Equal(t, "fail", check.Status)
		assert.Equal(t, "error", check.Severity)
		assert.Contains(t, text.String(), check.Detail)
		assert.Contains(t, string(encoded), reason)
	}
	assert.NotContains(t, text.String(), "provider//secret")
	assert.NotContains(t, string(encoded), "provider//secret")
	assert.Equal(t, 2, strings.Count(string(encoded), "reason=model_malformed"),
		"provider//secret and the malformed role reference fail; the valid role reference does not")
	collection := collectOMPDoctorSelectors(root)
	assert.Contains(t, collection.selectors, "@autopus_executor")
	assert.Len(t, collection.findings, 4)
}

func TestOMPDoctorSelectorCollection_ReportsUnreadableAgent(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root can read mode-000 fixtures")
	}
	skipWithoutPOSIXShellDoctorOMP(t)
	root := t.TempDir()
	agentDir := filepath.Join(root, ".omp", "agents")
	require.NoError(t, os.MkdirAll(agentDir, 0o700))
	path := filepath.Join(agentDir, "unreadable.md")
	require.NoError(t, os.WriteFile(path, []byte("---\nmodel: sonnet\n---\n"), 0))
	t.Cleanup(func() { _ = os.Chmod(path, 0o600) })
	installDoctorOMPFixture(t)

	checks := probeAndProjectOMPDoctorChecks(context.Background(), root, nil)
	check := ompDoctorCheckContaining(t, checks, "agent_unreadable")
	assert.Equal(t, "fail", check.Status)
}

func installDoctorOMPFixture(t *testing.T) {
	t.Helper()
	binDir := t.TempDir()
	executable := filepath.Join(binDir, "omp")
	script := `#!/bin/sh
case "$*" in
  *--version*) printf 'omp/17.1.8\n' ;;
  *--help*) printf '%s\n' '--mode <interactive|rpc> --no-session --cwd <path> --model <provider/model>' ;;
  *'config get'*) printf '%s\n' '[".agents/skills"]' ;;
  *'models --json'*) printf '%s\n' '{"models":[]}' ;;
  *) exit 17 ;;
esac
`
	require.NoError(t, os.WriteFile(executable, []byte(script), 0o755))
	t.Setenv("PATH", binDir+":/usr/bin:/bin")
}

func skipWithoutPOSIXShellDoctorOMP(t *testing.T) {
	t.Helper()
	if _, err := os.Stat("/bin/sh"); err != nil {
		t.Skip("POSIX shell fixture is unavailable")
	}
}
