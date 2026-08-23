// Package antigravity provides lifecycle methods (Validate, Clean) for the Antigravity CLI adapter.
package antigravity

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/processprobe"
)

// antigravityPluginListTimeout은 best-effort `agy plugin list` 프로브의
// 마지막 상한이다.
//
// processprobe.DefaultWaitDelay보다 반드시 커야 한다. 주석(:24-28)이 말하는
// "inherited-pipe bound가 이 상한보다 먼저 반환한다"는 성질이 그 순서에
// 달려 있는데, 두 값이 같아지면 유예가 실제로 필요해진 순간 상한 안에서
// 끝날 수 없다. 그러면 프로브가 실패하고, best-effort 저하가 경고를 조용히
// 삼켜서 결과가 뒤집힌다 - -race 계측이 붙었을 때 실제로 그렇게 깨졌다.
// 순서는 antigravity_lifecycle_bounds_test.go가 고정한다.
const antigravityPluginListTimeout = processprobe.DefaultWaitDelay + 2*time.Second

// Validate checks the validity of installed files.
func (a *Adapter) Validate(ctx context.Context) ([]adapter.ValidationError, error) {
	return a.validateWithin(ctx, antigravityPluginListTimeout)
}

// validateWithin is Validate with an explicit ceiling on the best-effort
// `agy plugin list` probe. That ceiling is the last-resort bound: the caller
// deadline and processprobe.Output's inherited-pipe bound both return long
// before it. Tests widen the ceiling to tell those bounds apart without timing
// the machine.
func (a *Adapter) validateWithin(ctx context.Context, pluginListTimeout time.Duration) ([]adapter.ValidationError, error) {
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

	// Verify if the autopus plugin is installed in agy
	if binary, lookErr := exec.LookPath(cliBinary); lookErr == nil {
		if ctx == nil {
			ctx = context.Background()
		}
		probeCtx, cancel := context.WithTimeout(ctx, pluginListTimeout)
		cmd := exec.CommandContext(probeCtx, binary, "plugin", "list")
		output, err := processprobe.Output(cmd)
		cancel()
		if err == nil && !strings.Contains(string(output), `"autopus"`) {
			errs = append(errs, adapter.ValidationError{
				File:    ".agents/plugins/autopus",
				Message: "antigravity에 autopus 플러그인이 설치되지 않음 (자동 복구를 위해 'auto setup'을 실행하세요)",
				Level:   "warning",
			})
		}
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
		filepath.Join(a.root, ".gemini", "hooks", "autopus"),
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
