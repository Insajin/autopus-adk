package design

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

const maxVisualCriticReportBytes int64 = 4 << 20

type visualCriticRoot interface {
	Lstat(name string) (os.FileInfo, error)
	Open(name string) (*os.File, error)
}

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
	rootFS, rootAbs, err := openVisualCriticRoot(root)
	if err != nil {
		return VisualCriticReport{}, err
	}
	defer func() { _ = rootFS.Close() }()
	relativePath, err := resolveVisualCriticPath(rootAbs, rawPath)
	if err != nil {
		return VisualCriticReport{}, err
	}
	return loadVisualCriticReportRoot(rootFS, relativePath)
}

func loadVisualCriticReportRoot(root visualCriticRoot, relativePath string) (VisualCriticReport, error) {
	data, err := readVisualCriticReport(root, relativePath)
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
	report.Source = filepath.ToSlash(relativePath)
	return report, nil
}

func openVisualCriticRoot(path string) (*os.Root, string, error) {
	rootAbs, err := filepath.Abs(path)
	if err != nil {
		return nil, "", err
	}
	resolvedRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, "", err
	}
	expected, err := os.Lstat(resolvedRoot)
	if err != nil {
		return nil, "", err
	}
	if expected.Mode()&os.ModeSymlink != 0 || !expected.IsDir() {
		return nil, "", fmt.Errorf("visual critic root must resolve to a directory")
	}
	root, err := os.OpenRoot(resolvedRoot)
	if err != nil {
		return nil, "", err
	}
	actual, err := root.Stat(".")
	if err != nil || !os.SameFile(expected, actual) {
		_ = root.Close()
		if err != nil {
			return nil, "", err
		}
		return nil, "", fmt.Errorf("visual critic root changed while opening")
	}
	return root, resolvedRoot, nil
}

func resolveVisualCriticPath(rootAbs, rawPath string) (string, error) {
	rawPath = strings.TrimSpace(rawPath)
	if hasParentTraversal(rawPath) {
		return "", fmt.Errorf("visual critic path must not contain parent traversal")
	}
	candidate := filepath.Clean(rawPath)
	if !filepath.IsAbs(candidate) && filepath.VolumeName(candidate) != "" {
		return "", fmt.Errorf("visual critic path escapes project root")
	}
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(rootAbs, candidate)
	}
	abs, err := filepath.Abs(candidate)
	if err != nil {
		return "", err
	}
	if !isInsideRoot(rootAbs, abs) {
		return "", fmt.Errorf("visual critic path escapes project root")
	}
	relativePath, err := filepath.Rel(rootAbs, abs)
	if err != nil {
		return "", err
	}
	if strings.ToLower(filepath.Ext(relativePath)) != ".json" {
		return "", fmt.Errorf("visual critic report must be json")
	}
	return filepath.Clean(relativePath), nil
}

func readVisualCriticReport(root visualCriticRoot, relativePath string) ([]byte, error) {
	relativePath = filepath.Clean(relativePath)
	expected, err := lstatVisualCriticReport(root, relativePath)
	if err != nil {
		return nil, err
	}
	if err := validateVisualCriticReportSize(expected.Size()); err != nil {
		return nil, err
	}
	file, err := root.Open(relativePath)
	if err != nil {
		return nil, err
	}
	defer func() { _ = file.Close() }()
	actual, err := file.Stat()
	if err != nil {
		return nil, err
	}
	if !actual.Mode().IsRegular() || !os.SameFile(expected, actual) {
		return nil, fmt.Errorf("visual critic report changed while opening")
	}
	if err := validateVisualCriticReportSize(actual.Size()); err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(file, maxVisualCriticReportBytes+1))
	if err != nil {
		return nil, err
	}
	if err := validateVisualCriticReportSize(int64(len(data))); err != nil {
		return nil, err
	}
	return data, nil
}

func lstatVisualCriticReport(root visualCriticRoot, relativePath string) (os.FileInfo, error) {
	parts := strings.Split(filepath.ToSlash(relativePath), "/")
	current := ""
	var info os.FileInfo
	for index, part := range parts {
		if part == "" || part == "." || part == ".." {
			return nil, fmt.Errorf("invalid visual critic path component")
		}
		current = filepath.Join(current, filepath.FromSlash(part))
		var err error
		info, err = root.Lstat(current)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("visual critic path escapes project root: symlink component %s", current)
		}
		if index < len(parts)-1 && !info.IsDir() {
			return nil, fmt.Errorf("visual critic path component is not a directory: %s", current)
		}
	}
	if info == nil || !info.Mode().IsRegular() {
		return nil, fmt.Errorf("visual critic report is not a regular file")
	}
	return info, nil
}

func validateVisualCriticReportSize(size int64) error {
	if size > maxVisualCriticReportBytes {
		return fmt.Errorf("visual critic report size limit exceeded: %d > %d", size, maxVisualCriticReportBytes)
	}
	return nil
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
