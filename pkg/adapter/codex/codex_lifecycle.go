package codex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

// Validate checks the validity of installed files.
func (a *Adapter) Validate(_ context.Context) ([]adapter.ValidationError, error) {
	var errs []adapter.ValidationError

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

	skillsDir := filepath.Join(a.root, ".codex", "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		errs = append(errs, adapter.ValidationError{
			File:    ".codex/skills",
			Message: ".codex/skills 디렉터리가 없음",
			Level:   "error",
		})
	}

	repoSkillPath := filepath.Join(a.root, ".agents", "skills", "auto", "SKILL.md")
	if _, err := os.Stat(repoSkillPath); os.IsNotExist(err) {
		errs = append(errs, adapter.ValidationError{
			File:    ".agents/skills/auto/SKILL.md",
			Message: "Codex 표준 router skill이 없음",
			Level:   "warning",
		})
	}

	marketplacePath := filepath.Join(a.root, ".agents", "plugins", "marketplace.json")
	if _, err := os.Stat(marketplacePath); os.IsNotExist(err) {
		errs = append(errs, adapter.ValidationError{
			File:    ".agents/plugins/marketplace.json",
			Message: "로컬 Codex plugin marketplace가 없음",
			Level:   "warning",
		})
	}

	return errs, nil
}

// Clean removes files created by this adapter.
func (a *Adapter) Clean(_ context.Context) error {
	if err := os.RemoveAll(filepath.Join(a.root, ".codex", "skills")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(".codex/skills 제거 실패: %w", err)
	}
	autoSkillDirs := []string{
		"auto",
		"auto-plan",
		"auto-go",
		"auto-fix",
		"auto-review",
		"auto-sync",
		"auto-idea",
		"auto-canary",
	}
	for _, dir := range autoSkillDirs {
		if err := os.RemoveAll(filepath.Join(a.root, ".agents", "skills", dir)); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf(".agents/skills/%s 제거 실패: %w", dir, err)
		}
	}
	if err := os.Remove(filepath.Join(a.root, ".agents", "plugins", "marketplace.json")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(".agents/plugins/marketplace.json 제거 실패: %w", err)
	}
	if err := os.RemoveAll(filepath.Join(a.root, ".autopus", "plugins", "auto")); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf(".autopus/plugins/auto 제거 실패: %w", err)
	}

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

// InstallHooks is a no-op — hooks are managed via .codex/hooks.json template.
func (a *Adapter) InstallHooks(_ context.Context, _ []adapter.HookConfig, _ *adapter.PermissionSet) error {
	return nil
}
