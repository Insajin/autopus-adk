package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/insajin/autopus-adk/pkg/config"
)

// checkAgentEffort ensures Claude subagents declare an explicit effort field.
func checkAgentEffort(dir string, out io.Writer, quiet bool) bool {
	if !quiet {
		tui.SectionHeader(out, "cc21: agent effort")
	}

	agentsDir := filepath.Join(dir, ".claude", "agents", "autopus")
	entries, err := os.ReadDir(agentsDir)
	if err != nil {
		if !quiet {
			tui.SKIP(out, "no .claude/agents/autopus directory found")
		}
		return true
	}

	allOK := true
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".md") || entry.Name() == "AGENT-GUIDE.md" {
			continue
		}
		path := filepath.Join(agentsDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			tui.FAIL(out, "cannot read "+path)
			allOK = false
			continue
		}
		if !frontmatterContainsKey(string(data), "effort") {
			tui.FAIL(out, "missing effort field: "+path)
			allOK = false
		}
	}

	if allOK && !quiet {
		tui.OK(out, "all Claude subagents declare effort")
	}
	return allOK
}

// checkMonitorCommands warns when Monitor command strings lack a line-buffering guard.
func checkMonitorCommands(dir string, out io.Writer, quiet bool) bool {
	if !quiet {
		tui.SectionHeader(out, "cc21: monitor commands")
	}

	path := filepath.Join(dir, ".claude", "skills", "autopus", "idea.md")
	data, err := os.ReadFile(path)
	if err != nil {
		if !quiet {
			tui.SKIP(out, "no idea.md Monitor surface found")
		}
		return true
	}

	text := string(data)
	if !strings.Contains(text, "Monitor(") {
		if !quiet {
			tui.SKIP(out, "no Monitor command found")
		}
		return true
	}

	if strings.Contains(text, "--line-buffered") || strings.Contains(text, "stdbuf -oL grep") {
		if !quiet {
			tui.OK(out, "Monitor commands include a line-buffered guard")
		}
		return true
	}

	if !quiet {
		tui.Info(out, "WARN monitor-patterns command lacks --line-buffered or stdbuf -oL grep guard")
	}
	return true
}

func checkEffortRuntime(flags globalFlags, out io.Writer, quiet bool) bool {
	if !quiet {
		tui.SectionHeader(out, "cc21: effort runtime")
	}

	envValue := ReadEnvEffort()
	for _, warning := range envEffortWarning(envValue) {
		if !quiet {
			tui.Info(out, warning)
		}
	}

	result, err := ResolveEffort(EffortResolveInput{
		EnvValue:    envValue,
		FlagEffort:  flags.Effort,
		FlagQuality: flags.Quality,
	})
	if err != nil {
		tui.FAIL(out, "effort resolution failed: "+err.Error())
		return false
	}

	switch {
	case flags.Effort != "":
		if result.Source != EffortSourceFlag || result.Effort != EffortValue(flags.Effort) {
			tui.FAIL(out, fmt.Sprintf("expected --effort=%s to win, got effort=%s (source=%s)", flags.Effort, displayEffortValue(result.Effort), result.Source))
			return false
		}
	case envValue != "" && isValidEffort(EffortValue(envValue)):
		if result.Source != EffortSourceEnv || result.Effort != EffortValue(envValue) {
			tui.FAIL(out, fmt.Sprintf("expected %s=%s to win, got effort=%s (source=%s)", envEffortKey, envValue, displayEffortValue(result.Effort), result.Source))
			return false
		}
	}

	if !quiet {
		tui.OK(out, fmt.Sprintf("effective effort=%s (source=%s)", displayEffortValue(result.Effort), result.Source))
	}
	return true
}

func checkTaskCreatedModePrecedence(dir string, flags globalFlags, out io.Writer, quiet bool) bool {
	if !quiet {
		tui.SectionHeader(out, "cc21: task-created mode")
	}

	var cfg *config.HarnessConfig
	configPath := filepath.Join(dir, "autopus.yaml")
	if _, err := os.Stat(configPath); err == nil {
		loaded, loadErr := config.Load(dir)
		if loadErr != nil {
			tui.FAIL(out, "cannot load autopus.yaml for TaskCreated mode: "+loadErr.Error())
			return false
		}
		cfg = loaded
	}

	result := resolveTaskCreatedMode(flags, cfg)
	for _, warning := range result.Warnings {
		if !quiet {
			tui.Info(out, warning)
		}
	}

	switch {
	case flags.TaskMode != "":
		if result.Source != "flag" || result.Mode != flags.TaskMode {
			tui.FAIL(out, fmt.Sprintf("expected --task-created-mode=%s to win, got mode=%s (source=%s)", flags.TaskMode, result.Mode, result.Source))
			return false
		}
	case isValidTaskCreatedMode(os.Getenv(taskCreatedModeEnvKey)):
		envMode := normalizeTaskCreatedMode(os.Getenv(taskCreatedModeEnvKey))
		if result.Source != "env" || result.Mode != envMode {
			tui.FAIL(out, fmt.Sprintf("expected %s=%s to win, got mode=%s (source=%s)", taskCreatedModeEnvKey, envMode, result.Mode, result.Source))
			return false
		}
	case cfg != nil && isValidTaskCreatedMode(cfg.Features.CC21.TaskCreatedMode):
		cfgMode := normalizeTaskCreatedMode(cfg.Features.CC21.TaskCreatedMode)
		if result.Source != "autopus.yaml" || result.Mode != cfgMode {
			tui.FAIL(out, fmt.Sprintf("expected autopus.yaml mode=%s to win, got mode=%s (source=%s)", cfgMode, result.Mode, result.Source))
			return false
		}
	}

	if !quiet {
		tui.OK(out, fmt.Sprintf("effective mode=%s (source=%s)", result.Mode, result.Source))
	}
	return true
}

func displayEffortValue(value EffortValue) string {
	if value == EffortStripped {
		return "<stripped>"
	}
	return string(value)
}

func frontmatterContainsKey(content, key string) bool {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 || strings.TrimSpace(lines[0]) != "---" {
		return false
	}
	for i := 1; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "---" {
			return false
		}
		if strings.HasPrefix(trimmed, key+":") {
			return true
		}
	}
	return false
}
