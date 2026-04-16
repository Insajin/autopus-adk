package codex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

const minProjectDocMaxBytes = 262144

// Validate checks the validity of installed files.
func (a *Adapter) Validate(_ context.Context) ([]adapter.ValidationError, error) {
	var errs []adapter.ValidationError

	if a.managesFile("AGENTS.md") {
		agentsPath := filepath.Join(a.root, "AGENTS.md")
		data, err := os.ReadFile(agentsPath)
		if err != nil {
			errs = append(errs, adapter.ValidationError{
				File:    "AGENTS.md",
				Message: "AGENTS.md를 읽을 수 없음",
				Level:   "error",
			})
			return errs, nil
		}

		content := string(data)
		if !strings.Contains(content, markerBegin) || !strings.Contains(content, markerEnd) {
			errs = append(errs, adapter.ValidationError{
				File:    "AGENTS.md",
				Message: "AUTOPUS 마커 섹션이 없음",
				Level:   "warning",
			})
		}
	}

	skillsDir := filepath.Join(a.root, ".codex", "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		errs = append(errs, adapter.ValidationError{
			File:    ".codex/skills",
			Message: ".codex/skills 디렉터리가 없음",
			Level:   "error",
		})
	}

	repoSkillRel := filepath.Join(".agents", "skills", "auto", "SKILL.md")
	if a.managesFile(repoSkillRel) {
		repoSkillPath := filepath.Join(a.root, repoSkillRel)
		if _, err := os.Stat(repoSkillPath); os.IsNotExist(err) {
			errs = append(errs, adapter.ValidationError{
				File:    repoSkillRel,
				Message: "Codex 표준 router skill이 없음",
				Level:   "warning",
			})
		}
	}

	marketplacePath := filepath.Join(a.root, ".agents", "plugins", "marketplace.json")
	if _, err := os.Stat(marketplacePath); os.IsNotExist(err) {
		errs = append(errs, adapter.ValidationError{
			File:    ".agents/plugins/marketplace.json",
			Message: "로컬 Codex plugin marketplace가 없음",
			Level:   "warning",
		})
	}

	a.validateRouterPrompt(&errs)
	a.validateConfig(&errs)
	a.validateContext7Rule(&errs)

	return errs, nil
}

// Clean removes files created by this adapter.
func (a *Adapter) Clean(_ context.Context) error {
	if err := os.RemoveAll(filepath.Join(a.root, ".codex", "skills")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(".codex/skills 제거 실패: %w", err)
	}
	if a.managesFile(filepath.Join(".agents", "skills", "auto", "SKILL.md")) {
		autoSkillDirs := []string{
			"auto",
			"auto-setup",
			"auto-status",
			"auto-plan",
			"auto-go",
			"auto-fix",
			"auto-review",
			"auto-sync",
			"auto-idea",
			"auto-map",
			"auto-why",
			"auto-verify",
			"auto-secure",
			"auto-test",
			"auto-dev",
			"auto-canary",
			"auto-doctor",
		}
		for _, dir := range autoSkillDirs {
			if err := os.RemoveAll(filepath.Join(a.root, ".agents", "skills", dir)); err != nil && !os.IsNotExist(err) {
				return fmt.Errorf(".agents/skills/%s 제거 실패: %w", dir, err)
			}
		}
	}
	if err := os.Remove(filepath.Join(a.root, ".agents", "plugins", "marketplace.json")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(".agents/plugins/marketplace.json 제거 실패: %w", err)
	}
	if err := os.RemoveAll(filepath.Join(a.root, ".autopus", "plugins", "auto")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(".autopus/plugins/auto 제거 실패: %w", err)
	}

	if a.managesFile("AGENTS.md") {
		agentsPath := filepath.Join(a.root, "AGENTS.md")
		data, err := os.ReadFile(agentsPath)
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return fmt.Errorf("AGENTS.md 읽기 실패: %w", err)
		}
		cleaned := removeMarkerSection(string(data))
		return os.WriteFile(agentsPath, []byte(cleaned), 0644)
	}
	return nil
}

// InstallHooks is a no-op — hooks are managed via .codex/hooks.json template.
func (a *Adapter) InstallHooks(_ context.Context, _ []adapter.HookConfig, _ *adapter.PermissionSet) error {
	return nil
}

func (a *Adapter) managesFile(targetPath string) bool {
	manifest, err := adapter.LoadManifest(a.root, adapterName)
	if err == nil && manifest != nil {
		_, ok := manifest.Files[targetPath]
		return ok
	}

	if isSharedSurfacePath(targetPath) {
		opencodeManifest, loadErr := adapter.LoadManifest(a.root, "opencode")
		if loadErr == nil && opencodeManifest != nil {
			return false
		}
	}

	return true
}

func isSharedSurfacePath(targetPath string) bool {
	if targetPath == "AGENTS.md" {
		return true
	}
	return strings.HasPrefix(targetPath, filepath.Join(".agents", "skills")+string(os.PathSeparator))
}

func (a *Adapter) validateRouterPrompt(errs *[]adapter.ValidationError) {
	routerPromptRel := filepath.Join(".codex", "prompts", "auto.md")
	if !a.managesFile(routerPromptRel) {
		return
	}

	data, err := os.ReadFile(filepath.Join(a.root, routerPromptRel))
	if err != nil {
		if os.IsNotExist(err) {
			*errs = append(*errs, adapter.ValidationError{
				File:    routerPromptRel,
				Message: "Codex router prompt가 없음",
				Level:   "warning",
			})
			return
		}
		*errs = append(*errs, adapter.ValidationError{
			File:    routerPromptRel,
			Message: "Codex router prompt를 읽을 수 없음",
			Level:   "warning",
		})
		return
	}

	content := string(data)
	if !strings.Contains(content, "## Autopus Branding") || !strings.Contains(content, "🐙 Autopus ─────────────────────────") {
		*errs = append(*errs, adapter.ValidationError{
			File:    routerPromptRel,
			Message: "Codex router prompt에 Autopus 브랜딩 블록이 없음",
			Level:   "warning",
		})
	}
	hasProjectContextContract := strings.Contains(content, "ARCHITECTURE.md") &&
		(strings.Contains(content, ".autopus/project/*") || strings.Contains(content, ".autopus/project/product.md"))
	if !strings.Contains(content, "## Router Execution Contract") || !hasProjectContextContract || !strings.Contains(content, "## SPEC Path Resolution") {
		*errs = append(*errs, adapter.ValidationError{
			File:    routerPromptRel,
			Message: "Codex router prompt에 상세 워크플로우/프로젝트 컨텍스트 계약이 없음",
			Level:   "warning",
		})
	}
}

func (a *Adapter) validateConfig(errs *[]adapter.ValidationError) {
	if !a.managesFile("config.toml") {
		return
	}

	data, err := os.ReadFile(filepath.Join(a.root, "config.toml"))
	if err != nil {
		if os.IsNotExist(err) {
			*errs = append(*errs, adapter.ValidationError{
				File:    "config.toml",
				Message: "config.toml이 없음",
				Level:   "warning",
			})
			return
		}
		*errs = append(*errs, adapter.ValidationError{
			File:    "config.toml",
			Message: "config.toml을 읽을 수 없음",
			Level:   "warning",
		})
		return
	}

	maxBytes, ok := parseProjectDocMaxBytes(string(data))
	if !ok {
		*errs = append(*errs, adapter.ValidationError{
			File:    "config.toml",
			Message: "project_doc_max_bytes 설정이 없음",
			Level:   "warning",
		})
		return
	}
	if maxBytes < minProjectDocMaxBytes {
		*errs = append(*errs, adapter.ValidationError{
			File:    "config.toml",
			Message: fmt.Sprintf("project_doc_max_bytes가 너무 낮음 (%d < %d): 대형 프로젝트 문서가 잘릴 수 있음", maxBytes, minProjectDocMaxBytes),
			Level:   "warning",
		})
	}
}

func (a *Adapter) validateContext7Rule(errs *[]adapter.ValidationError) {
	ruleRel := filepath.Join(".codex", "rules", "autopus", "context7-docs.md")
	if !a.managesFile(ruleRel) {
		return
	}

	data, err := os.ReadFile(filepath.Join(a.root, ruleRel))
	if err != nil {
		if os.IsNotExist(err) {
			*errs = append(*errs, adapter.ValidationError{
				File:    ruleRel,
				Message: "Codex Context7 규칙 파일이 없음",
				Level:   "warning",
			})
			return
		}
		*errs = append(*errs, adapter.ValidationError{
			File:    ruleRel,
			Message: "Codex Context7 규칙 파일을 읽을 수 없음",
			Level:   "warning",
		})
		return
	}

	content := string(data)
	if !strings.Contains(content, "Context7 MCP") || !strings.Contains(content, "web search") {
		*errs = append(*errs, adapter.ValidationError{
			File:    ruleRel,
			Message: "Codex Context7 규칙에 web fallback 계약이 없음",
			Level:   "warning",
		})
	}
}

func parseProjectDocMaxBytes(content string) (int, bool) {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "project_doc_max_bytes") {
			continue
		}

		parts := strings.SplitN(trimmed, "=", 2)
		if len(parts) != 2 {
			return 0, false
		}

		value, err := strconv.Atoi(strings.TrimSpace(parts[1]))
		if err != nil {
			return 0, false
		}
		return value, true
	}

	return 0, false
}
