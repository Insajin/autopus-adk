// Package gemini provides lifecycle methods (Validate, Clean) for the Antigravity CLI adapter.
package gemini

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

	geminiMDPath := filepath.Join(a.root, "GEMINI.md")
	data, err := os.ReadFile(geminiMDPath)
	if err != nil {
		errs = append(errs, adapter.ValidationError{
			File:    "GEMINI.md",
			Message: "GEMINI.md를 읽을 수 없음",
			Level:   "error",
		})
		return errs, nil
	}
	if !strings.Contains(string(data), markerBegin) {
		errs = append(errs, adapter.ValidationError{
			File:    "GEMINI.md",
			Message: "AUTOPUS 마커 섹션이 없음",
			Level:   "warning",
		})
	}

	skillDirs := []string{"auto-plan", "auto-go", "auto-fix", "auto-sync", "auto-review"}
	for _, sd := range skillDirs {
		skillPath := filepath.Join(a.root, ".gemini", "skills", "autopus", sd, "SKILL.md")
		if _, err := os.Stat(skillPath); os.IsNotExist(err) {
			errs = append(errs, adapter.ValidationError{
				File:    skillPath,
				Message: fmt.Sprintf("SKILL.md가 없음: %s", sd),
				Level:   "error",
			})
		}
	}

	agentsPath := filepath.Join(a.root, ".agents", "skills")
	if _, err := os.Stat(agentsPath); os.IsNotExist(err) {
		errs = append(errs, adapter.ValidationError{
			File:    ".agents/skills",
			Message: ".agents/skills 디렉터리가 없음",
			Level:   "warning",
		})
	}

	return errs, nil
}

// Clean removes files created by the adapter.
func (a *Adapter) Clean(_ context.Context) error {
	dirsToRemove := []string{
		filepath.Join(a.root, ".gemini", "skills"),
		filepath.Join(a.root, ".gemini", "commands"),
		filepath.Join(a.root, ".gemini", "rules"),
		filepath.Join(a.root, ".gemini", "agents"),
		filepath.Join(a.root, ".agents", "skills"),
		filepath.Join(a.root, antigravityPluginDir),
	}
	for _, d := range dirsToRemove {
		if err := os.RemoveAll(d); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("%s 제거 실패: %w", d, err)
		}
	}

	// Remove .gemini/settings.json and statusline.sh
	filesToRemove := []string{
		filepath.Join(a.root, ".gemini", "settings.json"),
		filepath.Join(a.root, ".gemini", "statusline.sh"),
	}
	for _, f := range filesToRemove {
		if err := os.Remove(f); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("%s 제거 실패: %w", filepath.Base(f), err)
		}
	}
	if err := a.removeAntigravityHooksJSON(); err != nil {
		return err
	}

	// Remove AUTOPUS marker section from GEMINI.md
	geminiPath := filepath.Join(a.root, "GEMINI.md")
	data, err := os.ReadFile(geminiPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("GEMINI.md 읽기 실패: %w", err)
	}
	cleaned := removeMarkerSection(string(data))
	return os.WriteFile(geminiPath, []byte(cleaned), 0644)
}
