package scaffold

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/domainreadiness"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

const (
	DesktopGUIJourneyID = "desktop-gui-explore"
	journeyRootRel      = ".autopus/qa/journeys"
)

type Options struct {
	ProjectDir         string
	ProjectDirExplicit bool
	Release            bool
	Workflow           string
}

type Result struct {
	Status              string       `json:"status"`
	ProjectDir          string       `json:"project_dir"`
	RequestedProjectDir string       `json:"requested_project_dir,omitempty"`
	WorkspaceRoot       string       `json:"workspace_root,omitempty"`
	TargetReason        string       `json:"target_reason,omitempty"`
	Created             []FileResult `json:"created,omitempty"`
	Skipped             []FileResult `json:"skipped,omitempty"`
	Warnings            []string     `json:"warnings,omitempty"`
	NextSteps           []string     `json:"next_steps,omitempty"`
}

type FileResult struct {
	ID     string `json:"id"`
	Path   string `json:"path"`
	Reason string `json:"reason,omitempty"`
}

func Init(opts Options) (Result, error) {
	projectDir, err := normalizeProjectDir(opts.ProjectDir)
	if err != nil {
		return Result{}, err
	}
	workflow, err := normalizeWorkflow(opts.Workflow)
	if err != nil {
		return Result{}, err
	}
	if workflow != workflowNone {
		opts.Release = true
	}
	resolution := resolveProjectDir(projectDir, opts.ProjectDirExplicit)
	projectDir = resolution.ProjectDir
	result := Result{
		Status:              "noop",
		ProjectDir:          projectDir,
		RequestedProjectDir: resolution.RequestedProjectDir,
		WorkspaceRoot:       resolution.WorkspaceRoot,
		TargetReason:        resolution.TargetReason,
		Warnings:            resolution.Warnings,
	}
	if resolution.SkipScaffold {
		result.NextSteps = initNextSteps(opts.Release, workflow, projectDir, resolution.WorkspaceRoot)
		return result, nil
	}

	coverage, warnings := loadJourneyCoverage(projectDir)
	result.Warnings = append(result.Warnings, warnings...)
	for _, starter := range detectJourneyStarters(projectDir, opts.Release) {
		if coverage.coversStarter(starter) {
			result.Skipped = append(result.Skipped, coverage.skippedStarter(projectDir, starter))
			continue
		}
		if err := ensureStarter(projectDir, starter, &result); err != nil {
			return Result{}, err
		}
	}
	if workflow == workflowGitHubActions {
		if err := ensureStarter(projectDir, githubActionsWorkflowStarter(projectDir), &result); err != nil {
			return Result{}, err
		}
	}

	if len(result.Created) > 0 {
		result.Status = "created"
	} else if len(result.Skipped) > 0 {
		result.Status = "skipped"
	} else {
		result.Warnings = append(result.Warnings, "no supported QA signals detected; no starter files were created")
	}
	result.NextSteps = initNextSteps(opts.Release, workflow, projectDir, resolution.WorkspaceRoot)
	return result, nil
}

func ensureStarter(projectDir string, starter starterFile, result *Result) error {
	path := filepath.Join(projectDir, filepath.FromSlash(starter.RelPath))
	rel, err := filepath.Rel(projectDir, path)
	if err != nil {
		return err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return fmt.Errorf("scaffold path escapes project root")
	}
	file := FileResult{
		ID:   starter.ID,
		Path: filepath.ToSlash(path),
	}
	if _, err := os.Stat(path); err == nil {
		file.Reason = "existing project-local file preserved"
		result.Skipped = append(result.Skipped, file)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := writeNewFile(path, []byte(starter.Body)); err != nil {
		return err
	}
	if starter.ValidateJourney {
		pack, err := journey.LoadFile(path)
		if err != nil {
			return err
		}
		if err := journey.Validate(pack, projectDir); err != nil {
			return err
		}
	}
	if starter.ValidateDomainCatalog {
		catalog, err := domainreadiness.LoadCatalogFile(path)
		if err != nil {
			return err
		}
		if report := domainreadiness.ValidateCatalog(catalog); !report.Valid {
			return fmt.Errorf("invalid domain readiness starter catalog: %#v", report.MissingDomains)
		}
	}
	file.Reason = starter.Reason
	result.Created = append(result.Created, file)
	return nil
}

func normalizeProjectDir(value string) (string, error) {
	if strings.TrimSpace(value) == "" {
		value = "."
	}
	abs, err := filepath.Abs(value)
	if err != nil {
		return "", err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !info.IsDir() {
		return "", fmt.Errorf("project dir is not a directory: %s", value)
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	return real, nil
}

func writeNewFile(path string, body []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	n, err := file.Write(body)
	if err != nil {
		return err
	}
	if n != len(body) {
		return io.ErrShortWrite
	}
	return nil
}

func initNextSteps(release bool, workflow, projectDir, workspaceRoot string) []string {
	projectFlag := projectDirFlag(projectDir, workspaceRoot)
	journeyRoot := ".autopus/qa/journeys/*.yaml"
	catalogPath := ".autopus/qa/domain-readiness/catalog.json"
	workflowPath := ".github/workflows/autopus-qa-release.yml"
	if rel := projectRel(projectDir, workspaceRoot); rel != "" && rel != "." {
		journeyRoot = filepath.ToSlash(filepath.Join(rel, journeyRoot))
		catalogPath = filepath.ToSlash(filepath.Join(rel, catalogPath))
		workflowPath = filepath.ToSlash(filepath.Join(rel, workflowPath))
	}
	steps := []string{
		"Review " + journeyRoot + " and replace starter commands with project-owned deterministic checks before trusting them.",
		"Review " + catalogPath + " and replace starter domains with project-owned readiness scenarios.",
		"Run auto qa plan" + projectFlag + " --format json to inspect runnable lanes and setup gaps.",
	}
	if release {
		steps = append(steps, "Run auto qa release"+projectFlag+" --dry-run --profile release-candidate --format json before enabling the gate on a release branch or tag.")
	}
	if workflow == workflowGitHubActions {
		steps = append(steps, "Review "+workflowPath+" and pin the auto installer version before making it required.")
	}
	return steps
}

func projectDirFlag(projectDir, workspaceRoot string) string {
	rel := projectRel(projectDir, workspaceRoot)
	if rel == "" || rel == "." {
		return ""
	}
	return " --project-dir " + shellToken(filepath.ToSlash(rel))
}

func projectRel(projectDir, workspaceRoot string) string {
	if strings.TrimSpace(workspaceRoot) == "" {
		return ""
	}
	rel, err := filepath.Rel(workspaceRoot, projectDir)
	if err != nil {
		return ""
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return ""
	}
	return filepath.ToSlash(rel)
}

func shellToken(value string) string {
	if value == "" || strings.ContainsAny(value, " \t\n'\"`$\\") {
		return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
	}
	return value
}

func normalizeWorkflow(value string) (string, error) {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		value = workflowNone
	}
	switch value {
	case workflowNone, workflowGitHubActions:
		return value, nil
	default:
		return "", fmt.Errorf("unsupported workflow %q", value)
	}
}
