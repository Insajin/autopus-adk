package adapter_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/adapter/antigravity"
	"github.com/insajin/autopus-adk/pkg/adapter/claude"
	"github.com/insajin/autopus-adk/pkg/adapter/codex"
	"github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/adapter/opencode"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type contextEngineeringSurface struct {
	name, root, pipeline string
	details              map[string]string
}

type contextEngineeringMatrix struct{ required, optional, workerOptional, excluded []string }

func generateContextEngineeringSurfaces(t *testing.T) map[string]contextEngineeringSurface {
	t.Helper()
	base := t.TempDir()
	type generator struct {
		name     string
		generate func(string) (*adapter.PlatformFiles, error)
		pipeline string
		detail   func(string) string
	}
	generators := []generator{
		{"claude", func(root string) (*adapter.PlatformFiles, error) {
			return claude.NewWithRoot(root).Generate(context.Background(), config.DefaultFullConfig("context-engineering"))
		}, ".claude/skills/agent-pipeline/SKILL.md",
			func(command string) string { return ".claude/skills/auto-" + command + "/SKILL.md" }},
		{"codex", func(root string) (*adapter.PlatformFiles, error) {
			return codex.NewWithRoot(root).Generate(context.Background(), config.DefaultFullConfig("context-engineering"))
		}, ".codex/skills/codex-agent-pipeline/SKILL.md",
			func(command string) string { return ".codex/skills/codex-auto-" + command + "/SKILL.md" }},
		{"opencode", func(root string) (*adapter.PlatformFiles, error) {
			return opencode.NewWithRoot(root).Generate(context.Background(), config.DefaultFullConfig("context-engineering"))
		}, ".agents/skills/agent-pipeline/SKILL.md",
			func(command string) string { return ".agents/skills/auto-" + command + "/SKILL.md" }},
		{"omp", func(root string) (*adapter.PlatformFiles, error) {
			cfg := config.DefaultFullConfig("context-engineering")
			cfg.Platforms = []string{"omp"}
			return omp.NewWithRoot(root).Generate(context.Background(), cfg)
		}, ".omp/skills/agent-pipeline/SKILL.md",
			func(command string) string { return ".omp/skills/auto-" + command + "/SKILL.md" }},
		{"gemini", func(root string) (*adapter.PlatformFiles, error) {
			return antigravity.NewWithRoot(root, antigravity.WithoutPluginInstall()).Generate(
				context.Background(), config.DefaultFullConfig("context-engineering"))
		}, ".gemini/skills/autopus/agent-pipeline/SKILL.md",
			func(command string) string { return ".gemini/skills/autopus/auto-" + command + "/SKILL.md" }},
	}
	out := make(map[string]contextEngineeringSurface, len(generators)+1)
	for _, item := range generators {
		root := filepath.Join(base, item.name)
		files, err := item.generate(root)
		require.NoError(t, err, item.name)
		for _, file := range files.Files {
			assert.False(t, filepath.IsAbs(file.TargetPath), "%s emitted absolute target %s", item.name, file.TargetPath)
			assert.False(t, strings.HasPrefix(filepath.Clean(file.TargetPath), ".."), "%s escaped scratch root", item.name)
		}
		details := make(map[string]string, 4)
		for _, command := range []string{"plan", "test", "canary", "go"} {
			details[command] = item.detail(command)
		}
		out[item.name] = contextEngineeringSurface{
			name: item.name, root: root, pipeline: item.pipeline, details: details,
		}
	}
	geminiSurface := out["gemini"]
	pluginDetails := make(map[string]string, 4)
	for _, command := range []string{"plan", "test", "canary", "go"} {
		pluginDetails[command] = ".agents/plugins/autopus/skills/auto-" + command + "/SKILL.md"
	}
	out["antigravity-mirror"] = contextEngineeringSurface{
		name: "antigravity-mirror", root: geminiSurface.root,
		pipeline: ".agents/plugins/autopus/skills/agent-pipeline/SKILL.md",
		details:  pluginDetails,
	}
	return out
}

func extractContextEngineeringPipelineRefs(body string) []string {
	seen := make(map[string]bool)
	var refs []string
	for _, field := range strings.Fields(body) {
		start := strings.Index(field, ".")
		if start < 0 {
			continue
		}
		end := strings.Index(field[start:], ".md")
		if end < 0 {
			continue
		}
		ref := field[start : start+end+len(".md")]
		if !strings.Contains(ref, "agent-pipeline") || seen[ref] {
			continue
		}
		seen[ref] = true
		refs = append(refs, ref)
	}
	sort.Strings(refs)
	return refs
}

func resolveContextEngineeringPipeline(root, detail, expected string) (string, error) {
	refs := extractContextEngineeringPipelineRefs(detail)
	if len(refs) != 1 || refs[0] != expected {
		return "", fmt.Errorf("auto-go refs %v do not match exact generated pipeline target %q", refs, expected)
	}
	resolved := filepath.Clean(filepath.FromSlash(refs[0]))
	if filepath.IsAbs(resolved) || resolved == ".." ||
		strings.HasPrefix(resolved, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("pipeline reference escapes scratch root: %q", refs[0])
	}
	body, err := os.ReadFile(filepath.Join(root, resolved))
	if err != nil {
		return "", fmt.Errorf("read referenced pipeline %q: %w", refs[0], err)
	}
	return string(body), nil
}

func parseContextEngineeringMatrix(body string) (contextEngineeringMatrix, error) {
	section, err := contextEngineeringSection(body, "## Context Profile")
	if err != nil {
		return contextEngineeringMatrix{}, err
	}
	var result contextEngineeringMatrix
	for _, line := range strings.Split(section, "\n") {
		line = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-"))
		label, value, ok := strings.Cut(line, ":")
		if !ok {
			continue
		}
		values := normalizeContextEngineeringValues(value)
		switch strings.ToLower(strings.TrimSpace(label)) {
		case "required", "supervisor required":
			result.required = values
		case "optional", "conditional":
			result.optional = values
		case "worker optional":
			result.workerOptional = values
		case "excluded", "excluded by default":
			result.excluded = values
		}
	}
	if len(result.required) == 0 || len(result.excluded) == 0 {
		return contextEngineeringMatrix{}, fmt.Errorf("explicit required/excluded context sets are missing")
	}
	return result, nil
}

func normalizeContextEngineeringValues(value string) []string {
	value = strings.Trim(strings.TrimSpace(value), ".")
	parts := strings.Split(value, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(strings.TrimSpace(part), "`")
		part = strings.ToLower(strings.ReplaceAll(part, "-", "_"))
		if part != "" {
			values = append(values, part)
		}
	}
	sort.Strings(values)
	return values
}

func contextEngineeringSection(body, heading string) (string, error) {
	start := strings.Index(body, heading)
	if start < 0 {
		return "", fmt.Errorf("%s is missing", heading)
	}
	section := body[start+len(heading):]
	end := len(section)
	for _, next := range []string{"\n## ", "\n### "} {
		if candidate := strings.Index(section, next); candidate >= 0 && candidate < end {
			end = candidate
		}
	}
	return section[:end], nil
}

func extractCanonicalWorkerFields(t *testing.T, body string) []string {
	t.Helper()
	start := strings.Index(body, `"worker_receipt_fields"`)
	require.GreaterOrEqual(t, start, 0)
	open := strings.Index(body[start:], "[")
	close := strings.Index(body[start+open:], "]")
	require.GreaterOrEqual(t, open, 0)
	require.GreaterOrEqual(t, close, 0)
	var fields []string
	require.NoError(t, json.Unmarshal([]byte(body[start+open:start+open+close+1]), &fields))
	sort.Strings(fields)
	return fields
}

func extractGeneratedWorkerFields(body string) []string {
	section, err := contextEngineeringSection(body, "Required return fields:")
	if err != nil {
		return nil
	}
	var fields []string
	for _, line := range strings.Split(section, "\n") {
		field := strings.Trim(strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(line), "-")), "`")
		if field != "" && !strings.ContainsAny(field, " :") {
			fields = append(fields, field)
		}
	}
	sort.Strings(fields)
	return fields
}

func assertContextEngineeringGuidance(t *testing.T, body string) {
	t.Helper()
	require.NoError(t, validateContextEngineeringSecurity(body))
	for _, token := range contextEngineeringEvidenceClauses {
		assert.True(t, strings.Contains(normalizeContextEngineeringProse(body), normalizeContextEngineeringProse(token)),
			"generated pipeline is missing semantic token %q", token)
	}
}

var contextEngineeringEvidenceClauses = []string{"supervisor verified delivery", "delegated-worker optional recall", "selected refs", "hashes", "omitted count", "2,000 estimated tokens", "short correct", "without padding"}

var contextEngineeringSecurityClauses = []string{
	"accept only stable project-relative source refs", "reject absolute paths, .. traversal, symlinks, and non-regular files",
	"sanitize and redact retrieved content while preserving injection evidence", "do not relay full repeated artifact bodies",
	"do not replay raw tool results, provider payloads, or any required document body",
}

var contextEngineeringAdversarialFixtures = []struct{ name, body string }{
	{"unsafe refs permitted", "stable project-relative source refs are optional; absolute paths, .. traversal, symlinks, and non-regular files are permitted"},
	{"sanitization disabled", "do not sanitize or redact retrieved content while preserving injection evidence"},
	{"raw replay allowed", "workers may replay raw tool results, provider payloads, required document bodies, and repeated artifacts"},
}

func validateContextEngineeringSecurity(body string) error {
	normalized := normalizeContextEngineeringProse(body)
	for _, clause := range contextEngineeringSecurityClauses {
		if !strings.Contains(normalized, normalizeContextEngineeringProse(clause)) {
			return fmt.Errorf("missing restrictive context clause %q", clause)
		}
	}
	for _, forbidden := range []string{"absolute paths, .. traversal, symlinks, and non-regular files are permitted", "do not sanitize or redact", "may replay raw tool results"} {
		if strings.Contains(normalized, forbidden) {
			return fmt.Errorf("contradictory context polarity %q", forbidden)
		}
	}
	return nil
}

func normalizeContextEngineeringProse(body string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.ReplaceAll(body, "`", ""))), " ")
}

func readContextEngineeringFile(t *testing.T, root, rel string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	require.NoError(t, err, rel)
	return string(body)
}

func readEmbeddedContextEngineeringFile(t *testing.T, fsys interface{ ReadFile(string) ([]byte, error) }, path string) []byte {
	t.Helper()
	body, err := fsys.ReadFile(path)
	require.NoError(t, err, path)
	return body
}
