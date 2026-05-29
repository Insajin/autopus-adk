package design

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type VisualCriticReport struct {
	Version  int                   `json:"version,omitempty"`
	Status   string                `json:"status,omitempty"`
	Findings []VisualCriticFinding `json:"findings,omitempty"`
	Source   string                `json:"source,omitempty"`
}

type VisualCriticFinding struct {
	Severity     string `json:"severity"`
	Category     string `json:"category"`
	Message      string `json:"message"`
	Screenshot   string `json:"screenshot,omitempty"`
	SuggestedFix string `json:"suggested_fix,omitempty"`
}

func LoadVisualCriticReport(root, rawPath string) (VisualCriticReport, error) {
	if strings.TrimSpace(rawPath) == "" {
		return VisualCriticReport{}, nil
	}
	path, err := resolveVisualCriticPath(root, rawPath)
	if err != nil {
		return VisualCriticReport{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return VisualCriticReport{}, err
	}
	var report VisualCriticReport
	if err := json.Unmarshal(data, &report); err != nil {
		return VisualCriticReport{}, err
	}
	if report.Status == "" {
		report.Status = deriveCriticStatus(report.Findings)
	}
	report.Source = relPath(root, path)
	return report, nil
}

func resolveVisualCriticPath(root, rawPath string) (string, error) {
	if strings.Contains(filepath.ToSlash(rawPath), "..") {
		return "", fmt.Errorf("visual critic path must not contain parent traversal")
	}
	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	if evaluatedRoot, err := filepath.EvalSymlinks(rootAbs); err == nil {
		rootAbs = evaluatedRoot
	}
	candidate := rawPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(rootAbs, candidate)
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	if rel, err := filepath.Rel(rootAbs, abs); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("visual critic path escapes project root")
	}
	if evaluated, err := filepath.EvalSymlinks(abs); err == nil {
		if rel, err := filepath.Rel(rootAbs, evaluated); err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return "", fmt.Errorf("visual critic path escapes project root")
		}
		abs = evaluated
	}
	if strings.ToLower(filepath.Ext(abs)) != ".json" {
		return "", fmt.Errorf("visual critic report must be json")
	}
	return abs, nil
}

func deriveCriticStatus(findings []VisualCriticFinding) string {
	status := "PASS"
	for _, finding := range findings {
		switch strings.ToUpper(finding.Severity) {
		case "FAIL", "ERROR", "HIGH":
			return "FAIL"
		case "WARN", "WARNING", "MEDIUM":
			status = "WARN"
		}
	}
	return status
}
