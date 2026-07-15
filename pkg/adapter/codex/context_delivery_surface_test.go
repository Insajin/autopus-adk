package codex_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/config"
)

func TestCodexAdapter_AgentPipelineCarriesVerifiedRequiredContextContract(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	_, err := codex.NewWithRoot(root).Generate(context.Background(), config.DefaultFullConfig("context-delivery"))
	require.NoError(t, err)
	body, err := os.ReadFile(filepath.Join(root, ".codex", "skills", "agent-pipeline.md"))
	require.NoError(t, err)
	content := string(body)
	lower := strings.ToLower(content)

	for _, required := range []string{
		"auto workflow context", "optional recall", "required documents", "required_references",
		"source_hash", "prompt_hash", "context_ack", "context_integrity_failed",
		"compact_ultra", "full_ultra", "--context-required-document",
	} {
		assertSurfaceContains(t, lower, required)
	}
	assertSurfaceContains(t, lower, "never truncate")
	assertSurfaceContains(t, lower, "never summarize")
	assertSurfaceContains(t, lower, "never drop")
	assertSurfaceContains(t, lower, "hash mismatch")

	for _, nativeToken := range []string{`task_name=`, `message=`, `fork_turns="all"`} {
		assertSurfaceContains(t, content, nativeToken)
	}
	assertSurfaceOmits(t, content, "fork_context")
	assertSurfaceOmits(t, content, "agent_type")

	autoGo, err := os.ReadFile(filepath.Join(root, ".agents", "skills", "auto-go", "SKILL.md"))
	require.NoError(t, err)
	autoGoContent := string(autoGo)
	for _, required := range []string{
		"auto workflow context", "--required-document", "--context-required-document",
		"provider를 호출하지 않습니다", "supervisor가 보유한 필수 reference 집합",
	} {
		assertSurfaceContains(t, autoGoContent, required)
	}
	assertSurfaceOmits(t, autoGoContent, "현재 SPEC 구현 흐름은 계속 진행합니다")
}

func assertSurfaceContains(t *testing.T, content, needle string) {
	t.Helper()
	assert.True(t, strings.Contains(content, needle), "generated agent-pipeline is missing %q", needle)
}

func assertSurfaceOmits(t *testing.T, content, needle string) {
	t.Helper()
	assert.False(t, strings.Contains(content, needle), "generated agent-pipeline contains unsupported %q", needle)
}
