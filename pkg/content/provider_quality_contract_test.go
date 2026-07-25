package content

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderQualityGuidanceSourceAndGeneratedParity(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Dir(repoContentDir(t))
	commonFragments := []string{
		"quality.providers.<claude|codex>",
		"quality.default",
		"balanced",
		"auto quality provider claude ultra --apply",
		"auto quality provider codex balanced --apply",
		"auto quality provider claude inherit --apply",
		"refreshes only",
		"refresh every configured platform",
	}
	files := []string{
		"content/skills/adaptive-quality.md",
		"templates/codex/skills/adaptive-quality.md.tmpl",
		"templates/gemini/skills/adaptive-quality/SKILL.md.tmpl",
	}

	for _, relativePath := range files {
		relativePath := relativePath
		t.Run(relativePath, func(t *testing.T) {
			t.Parallel()
			document := readOpus5ContractFile(t, repoRoot, relativePath)
			assertOpus5ContractFragments(t, relativePath, document, commonFragments)
		})
	}
}

func TestProviderQualityClaudeDispatcherPrecedence(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Dir(repoContentDir(t))
	relativePath := "templates/claude/commands/auto-workflows.md.tmpl"
	document := readOpus5ContractFile(t, repoRoot, relativePath)
	fragments := []string{
		"explicit override",
		"quality.providers.claude",
		"quality.default",
		`fallback to "balanced"`,
		"quality.providers.codex",
	}
	assertOpus5ContractFragments(t, relativePath, document, fragments)

	qualityFlag := strings.Index(document, "2. If `QUALITY` is set via `--quality` flag")
	provider := strings.Index(document, "3. Read `autopus.yaml` → `quality.providers.claude`")
	globalDefault := strings.Index(document, "4. Otherwise read `autopus.yaml` → `quality.default`")
	safety := strings.Index(document, "5. If `AUTO_MODE = true` and no configured mode is found → fallback to \"balanced\"")
	if qualityFlag < 0 || provider < 0 || globalDefault < 0 || safety < 0 {
		t.Fatalf("missing provider quality precedence steps in %s", relativePath)
	}
	if !(qualityFlag < provider && provider < globalDefault && globalDefault < safety) {
		t.Fatalf(
			"provider quality precedence order is invalid: flag=%d provider=%d default=%d safety=%d",
			qualityFlag,
			provider,
			globalDefault,
			safety,
		)
	}
}

func TestProviderQualityOperatorDocsExposeIndependentModes(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Dir(repoContentDir(t))
	files := map[string][]string{
		"README.md": {
			"providers:\n    claude: ultra\n    codex: balanced",
			"Provider-specific `--apply` refreshes only that configured platform",
		},
		"docs/README.ko.md": {
			"providers:\n    claude: ultra\n    codex: balanced",
			"Provider별 `--apply`는 해당 configured platform만 갱신",
		},
		"content/skills/using-autopus.md": {
			"quality.providers.claude",
			"quality.providers.codex",
			"canonical key",
		},
	}
	for relativePath, fragments := range files {
		document := readOpus5ContractFile(t, repoRoot, relativePath)
		assertOpus5ContractFragments(t, relativePath, document, fragments)
	}
}
