package adapter_test

import (
	"path/filepath"
	"strings"
	"testing"

	contentfs "github.com/insajin/autopus-adk/content"
	templatefs "github.com/insajin/autopus-adk/templates"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContextEngineering_GeneratedSurfacesMatchCanonicalContract(t *testing.T) {
	surfaces := generateContextEngineeringSurfaces(t)
	t.Run("S1 claude thin router", func(t *testing.T) {
		surface := surfaces["claude"]
		router := readContextEngineeringFile(t, surface.root, ".claude/skills/auto/SKILL.md")
		detail := readContextEngineeringFile(t, surface.root, surface.details["go"])
		assert.Equal(t, 1, strings.Count(router, surface.details["go"]))
		assert.Equal(t, 1, strings.Count(detail, "## Context Profile"))
		assert.NoFileExists(t, filepath.Join(surface.root, ".claude", "skills", "autopus", "auto-workflows.md"))
		for _, forbidden := range []string{"Always load every project context document", "unconditional all-document preload"} {
			assert.NotContains(t, router+"\n"+detail, forbidden)
		}
	})

	expectedMatrix := map[string]contextEngineeringMatrix{
		"plan": {required: []string{"architecture", "core", "relevant_spec"},
			optional: []string{"learning", "signature"}, excluded: []string{"canary", "test"}},
		"test": {required: []string{"core", "test"},
			optional: []string{"learning", "signature"}, excluded: []string{"canary"}},
		"canary": {required: []string{"canary", "core"},
			optional: []string{"learning"}, excluded: []string{"signature", "test"}},
		"go": {required: []string{"acceptance", "available_architecture", "core", "plan", "resolved_spec"},
			workerOptional: []string{"learning", "signature", "task_declared_extra"}, excluded: []string{"canary", "test"}},
	}
	for _, surface := range surfaces {
		surface := surface
		t.Run("S2 matrix "+surface.name, func(t *testing.T) {
			for command, expected := range expectedMatrix {
				body := readContextEngineeringFile(t, surface.root, surface.details[command])
				actual, err := parseContextEngineeringMatrix(body)
				require.NoError(t, err, "%s %s", surface.name, command)
				assert.Equal(t, expected, actual, "%s %s", surface.name, command)
			}
		})
	}
	t.Run("S2 codex native go mirror", func(t *testing.T) {
		body := readContextEngineeringFile(t, surfaces["codex"].root, ".codex/skills/auto-go.md")
		actual, err := parseContextEngineeringMatrix(body)
		require.NoError(t, err)
		assert.Equal(t, expectedMatrix["go"], actual)
	})

	expectedFields := []string{"blockers", "changed_files", "next_required_step", "owned_paths", "verification"}
	for name, source := range map[string][]byte{
		"content agent-teams": readEmbeddedContextEngineeringFile(t, contentfs.FS, "skills/agent-teams.md"),
		"shared contract":     readEmbeddedContextEngineeringFile(t, templatefs.FS, "shared/orchestration-contract.md.tmpl"),
		"codex agent-teams":   readEmbeddedContextEngineeringFile(t, templatefs.FS, "codex/skills/agent-teams.md.tmpl"),
	} {
		name, source := name, source
		t.Run("S3 canonical owner "+name, func(t *testing.T) {
			assert.Equal(t, expectedFields, extractCanonicalWorkerFields(t, string(source)))
		})
	}
	for _, surface := range surfaces {
		surface := surface
		t.Run("S3-S4 route-resolved pipeline "+surface.name, func(t *testing.T) {
			detail := readContextEngineeringFile(t, surface.root, surface.details["go"])
			body, err := resolveContextEngineeringPipeline(surface.root, detail, surface.pipeline)
			require.NoError(t, err)
			assert.Equal(t, expectedFields, extractGeneratedWorkerFields(body))
			assertContextEngineeringGuidance(t, body)
		})
	}
	t.Run("S3-S4 stale and missing pipeline refs fail closed", func(t *testing.T) {
		const expected = ".agents/skills/agent-pipeline/SKILL.md"
		_, err := resolveContextEngineeringPipeline(t.TempDir(),
			"Load `.agents/skills/stale-agent-pipeline/SKILL.md`.", expected)
		assert.ErrorContains(t, err, "exact generated pipeline target")
		_, err = resolveContextEngineeringPipeline(t.TempDir(), "Load `"+expected+"`.", expected)
		assert.Error(t, err, "referenced but missing pipeline must fail")
	})
	t.Run("S4 gemini antigravity normalized parity", func(t *testing.T) {
		native := normalizeContextEngineeringProse(readContextEngineeringFile(t, surfaces["gemini"].root, surfaces["gemini"].pipeline))
		plugin := normalizeContextEngineeringProse(readContextEngineeringFile(t, surfaces["antigravity-mirror"].root, surfaces["antigravity-mirror"].pipeline))
		for _, clause := range append(contextEngineeringSecurityClauses, contextEngineeringEvidenceClauses...) {
			assert.Equal(t, strings.Contains(native, normalizeContextEngineeringProse(clause)),
				strings.Contains(plugin, normalizeContextEngineeringProse(clause)), clause)
		}
	})
	t.Run("S4 inverted clauses are rejected", func(t *testing.T) {
		valid := strings.Join(contextEngineeringSecurityClauses, ". ")
		for _, fixture := range contextEngineeringAdversarialFixtures {
			t.Run(fixture.name, func(t *testing.T) {
				assert.Error(t, validateContextEngineeringSecurity(valid+". "+fixture.body))
			})
		}
	})
}
