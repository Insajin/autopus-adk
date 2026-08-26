package adapter_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertOMPLatestCLIContract(t *testing.T, fixture latestCLIFixture) {
	t.Helper()
	manifest := fixture.manifests["omp"]
	counts := map[string]int{}
	for _, path := range manifestPaths(manifest) {
		for _, root := range []string{"skills", "commands", "agents", "rules"} {
			if strings.HasPrefix(path, ".omp/"+root+"/") {
				counts[root]++
			}
		}
		assert.False(t, strings.HasPrefix(path, ".agents/skills/"), path)
		assert.False(t, strings.HasPrefix(path, ".agents/commands/"), path)
	}
	for _, root := range []string{"skills", "commands", "agents", "rules"} {
		assert.Greater(t, counts[root], 0, ".omp/%s must be manifest-owned", root)
	}
	assert.False(t, manifestHas(manifest, ".omp/config.yml"))
	assert.NoFileExists(t, filepath.Join(fixture.root, ".omp", "config.yml"))

	body := readLatestCLISurface(t, fixture, ".omp/skills/auto-go/SKILL.md")
	example := decodeOMPTaskExample(t, body)
	assert.ElementsMatch(t, []string{"i", "context", "tasks"}, mapKeys(example))
	require.NotEmpty(t, example["i"])
	require.NotEmpty(t, example["context"])
	tasks, ok := example["tasks"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, tasks)
	for _, raw := range tasks {
		item, itemOK := raw.(map[string]any)
		require.True(t, itemOK)
		assert.NotContains(t, item, "isolated")
		assert.NotContains(t, item, "effort")
		assert.NotEmpty(t, item["task"])
		assert.Equal(t, "strict", item["schemaMode"])
	}
	assertOMPReadinessProviderFree(t, fixture.root)
}

func decodeOMPTaskExample(t *testing.T, body string) map[string]any {
	t.Helper()
	section := strings.Index(body, "## OMP Coordination Contract")
	require.NotEqual(t, -1, section)
	rest := body[section:]
	start := strings.Index(rest, "```json\n")
	require.NotEqual(t, -1, start)
	rest = rest[start+len("```json\n"):]
	end := strings.Index(rest, "\n```")
	require.NotEqual(t, -1, end)
	var example map[string]any
	require.NoError(t, json.Unmarshal([]byte(rest[:end]), &example))
	return example
}

func mapKeys(values map[string]any) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	return keys
}

func assertLatestCLIManifestOwnership(t *testing.T, fixture latestCLIFixture) {
	t.Helper()
	owners := map[string][]string{}
	for _, platform := range latestCLIPlatforms {
		for _, path := range manifestPaths(fixture.manifests[platform]) {
			owners[path] = append(owners[path], platform)
		}
	}
	allowedShared := map[string][]string{
		"AGENTS.md":                        {"codex", "opencode"},
		".agents/plugins/marketplace.json": {"codex", "opencode"},
	}
	for path, pathOwners := range owners {
		if len(pathOwners) == 1 {
			continue
		}
		allowed, ok := allowedShared[path]
		require.True(t, ok, "unexpected shared manifest ownership: %s by %v", path, pathOwners)
		assert.ElementsMatch(t, allowed, pathOwners, path)
	}
	assert.Equal(t, []string{"opencode"}, owners["AGENTS.md"])
}
