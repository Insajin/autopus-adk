package cli

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

// saveOrchestraResult writes orchestra results to a timestamped markdown file
// under .autopus/orchestra/. Returns the file path on success.
func saveOrchestraResult(command, strategy string, providers []string, timeout ResolvedOrchestraTimeout, result *orchestra.OrchestraResult) (string, error) {
	dir := ".autopus/orchestra"
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	ts := time.Now().Format("20060102-150405")
	filename := fmt.Sprintf("%s/%s-%s-%s.md", dir, command, strategy, ts)

	header := fmt.Sprintf("# Orchestra: %s (%s)\n\n**Date**: %s  \n**Strategy**: %s  \n**Providers**: %s  \n**Duration**: %s  \n",
		command, strategy,
		time.Now().Format("2006-01-02 15:04:05"),
		strategy,
		strings.Join(providers, ", "),
		result.Duration.Round(time.Second))
	if result.RunID != "" {
		header += fmt.Sprintf("**Run ID**: %s  \n", result.RunID)
	}
	if timeout.Seconds > 0 {
		header += fmt.Sprintf("**Effective Timeout**: %ds (%s)  \n", timeout.Seconds, timeout.Source)
	}
	if resultIsDegraded(result) {
		header += "**Status**: degraded  \n"
	}
	if result.Reliability != nil && result.Reliability.ArtifactDir != "" {
		header += fmt.Sprintf("**Artifacts**: %s  \n", result.Reliability.ArtifactDir)
	}
	header += "\n---\n\n"

	content := header + strings.TrimRight(result.Merged, "\n") + "\n"
	if diagnostics := renderProviderDiagnosticsMarkdown(timeout, result.FailedProviders); diagnostics != "" {
		content += "\n" + diagnostics
	}
	return filename, os.WriteFile(filename, []byte(content), 0o644)
}

func resultIsDegraded(result *orchestra.OrchestraResult) bool {
	return result != nil && (result.Degraded || len(result.FailedProviders) > 0)
}

func renderProviderDiagnosticsMarkdown(timeout ResolvedOrchestraTimeout, failed []orchestra.FailedProvider) string {
	if len(failed) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("## Provider Diagnostics\n\n")
	sb.WriteString("| Provider | Class | Error | Timeout | Timeout Source | Next | Stderr Preview | Output Preview |\n")
	sb.WriteString("|----------|-------|-------|---------|----------------|------|----------------|----------------|\n")
	for _, fp := range failed {
		duration, source := timeoutForProvider(timeout, fp.Name)
		fmt.Fprintf(&sb, "| %s | %s | %s | %s | %s | %s | %s | %s |\n",
			markdownCell(fp.Name),
			markdownCell(fp.FailureClass),
			markdownCell(fp.Error),
			markdownCell(duration),
			markdownCell(source),
			markdownCell(fp.NextRemediation),
			markdownCell(fp.StderrPreview),
			markdownCell(fp.OutputPreview),
		)
	}
	return sb.String()
}

func timeoutForProvider(timeout ResolvedOrchestraTimeout, provider string) (string, string) {
	for _, detail := range timeout.Providers {
		if detail.Provider == provider {
			return detail.Duration.String(), detail.Source
		}
	}
	if timeout.Seconds <= 0 {
		return "", ""
	}
	return (time.Duration(timeout.Seconds) * time.Second).String(), timeout.Source
}

func markdownCell(value string) string {
	value = strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	value = strings.ReplaceAll(value, "|", "\\|")
	return value
}
