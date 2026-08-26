package content

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readOpus5ContractFile(t *testing.T, root, relativePath string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(root, relativePath))
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}
	return string(data)
}

func assertOpus5ContractFragments(t *testing.T, relativePath, document string, fragments []string) {
	t.Helper()

	for _, fragment := range fragments {
		if !strings.Contains(document, fragment) {
			t.Errorf("%s missing Opus 5 contract fragment %q", relativePath, fragment)
		}
	}
}

func TestOpus5Guidance_SourceAndGeneratedContracts(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Dir(repoContentDir(t))
	matrixFragments := []string{
		"| Anthropic API | Opus 5 | Opus 4.8 on v2.1.154–v2.1.218 |",
		"| Claude Platform on AWS | Opus 5 | Opus 4.8 on v2.1.207–v2.1.218; Opus 4.7 before v2.1.207 |",
		"| Amazon Bedrock | Opus 5 | Opus 4.8 on v2.1.207–v2.1.218; Opus 4.6 before v2.1.207 |",
		"| Google Cloud Agent Platform | Opus 5 | Opus 4.8 on v2.1.207–v2.1.218; Opus 4.6 before v2.1.207 |",
		"| Microsoft Foundry | Opus 4.6 | Opus 4.6 |",
	}
	contractFiles := map[string][]string{
		"content/skills/adaptive-quality.md": {
			"drop-in upgrade from Opus 4.8",
			"Adaptive thinking is enabled by default",
			"`thinking: {\"type\": \"disabled\"}`",
			"disabled thinking with `xhigh` or `max` returns HTTP 400",
		},
		"content/skills/using-autopus.md": {
			"drop-in upgrade",
			"adaptive thinking이 기본 활성화",
			"`thinking: {\"type\": \"disabled\"}`",
			"`xhigh` 또는 `max`를 함께 보내면 HTTP 400",
		},
		"templates/codex/skills/adaptive-quality.md.tmpl": {
			"drop-in upgrade from Opus 4.8",
			"disabled thinking with `xhigh` or `max` returns HTTP 400",
		},
		"templates/codex/skills/using-autopus.md.tmpl": {
			"drop-in upgrade",
			"`xhigh` 또는 `max`를 함께 보내면 HTTP 400",
		},
		"templates/gemini/skills/adaptive-quality/SKILL.md.tmpl": {
			"drop-in upgrade from Opus 4.8",
			"disabled thinking with `xhigh` or `max` returns HTTP 400",
		},
		"templates/gemini/skills/using-autopus/SKILL.md.tmpl": {
			"drop-in upgrade",
			"`xhigh` 또는 `max`를 함께 보내면 HTTP 400",
		},
	}

	for relativePath, migrationFragments := range contractFiles {
		relativePath := relativePath
		migrationFragments := migrationFragments
		t.Run(relativePath, func(t *testing.T) {
			t.Parallel()

			document := readOpus5ContractFile(t, repoRoot, relativePath)
			assertOpus5ContractFragments(t, relativePath, document, matrixFragments)
			assertOpus5ContractFragments(t, relativePath, document, migrationFragments)
		})
	}
}

func TestWorkflowDoctorGuidance_UsesRouteAwarePins(t *testing.T) {
	t.Parallel()

	repoRoot := filepath.Dir(repoContentDir(t))
	files := map[string][]string{
		"content/skills/harness-workflow.md": {
			"`auto workflow doctor --route route_a`",
			"`RouteAMinVersion=2.1.246`",
			"`auto workflow doctor --route route_team`",
			"`RouteTeamMinVersion=2.1.246`",
		},
		"content/skills/using-autopus.md": {
			"`auto workflow doctor --route route_team`",
			"`route_a`",
			"`2.1.154`",
		},
		"templates/claude/commands/auto-workflows.md.tmpl": {
			"`auto workflow doctor --route route_a`",
			"`auto workflow doctor --route route_team`",
		},
	}

	for relativePath, fragments := range files {
		document := readOpus5ContractFile(t, repoRoot, relativePath)
		assertOpus5ContractFragments(t, relativePath, document, fragments)
	}
}
