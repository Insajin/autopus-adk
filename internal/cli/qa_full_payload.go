package cli

import (
	"os"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/domainreadiness"
	qarelease "github.com/insajin/autopus-adk/pkg/qa/release"
	qascaffold "github.com/insajin/autopus-adk/pkg/qa/scaffold"
)

const qaFullSchemaVersion = "qamesh.full.v1"

type qaFullPayload struct {
	SchemaVersion     string                   `json:"schema_version"`
	Mode              string                   `json:"mode"`
	Profile           string                   `json:"profile"`
	ProjectDir        string                   `json:"project_dir"`
	QAPolicy          qaFullPolicy             `json:"qa_policy"`
	Bootstrap         *qascaffold.Result       `json:"bootstrap,omitempty"`
	Summary           qaFullSummary            `json:"summary"`
	NextCommands      []string                 `json:"next_commands"`
	ProjectCandidates []qaFullProjectCandidate `json:"project_candidates,omitempty"`
	ReleasePlan       *qarelease.Plan          `json:"release_plan,omitempty"`
	ReleaseResult     *qarelease.Index         `json:"release_result,omitempty"`
	ReleaseIndexRef   string                   `json:"release_index_ref,omitempty"`
	DomainReadiness   qaFullDomainReadiness    `json:"domain_readiness"`
}

type qaFullSummary struct {
	Status              string   `json:"status"`
	Action              string   `json:"action"`
	SelectedLanes       []string `json:"selected_lanes"`
	MustLanes           []string `json:"must_lanes,omitempty"`
	JourneyPackCount    int      `json:"journey_pack_count"`
	SetupGapCount       int      `json:"setup_gap_count"`
	BlockingSetupGaps   int      `json:"blocking_setup_gaps"`
	DomainScenarioCount int      `json:"domain_scenario_count"`
	DomainSetupGap      bool     `json:"domain_setup_gap"`
	RootBlockerLane     string   `json:"root_blocker_lane,omitempty"`
	RootBlockerReason   string   `json:"root_blocker_reason,omitempty"`
	RootFailedJourneyID string   `json:"root_failed_journey_id,omitempty"`
	RootFailureSummary  string   `json:"root_failure_summary,omitempty"`
}

type qaFullProjectCandidate struct {
	ProjectDir string `json:"project_dir"`
	RelPath    string `json:"rel_path"`
	Score      int    `json:"score"`
	Reason     string `json:"reason"`
}

type qaFullDomainReadiness struct {
	Status      string                          `json:"status"`
	CatalogPath string                          `json:"catalog_path"`
	SetupGap    string                          `json:"setup_gap,omitempty"`
	Plan        *domainreadiness.CompileSummary `json:"plan,omitempty"`
}

func buildQAFullPlanPayload(opts qaFullOptions, plan qarelease.Plan, domain qaFullDomainReadiness, bootstrap *qascaffold.Result) qaFullPayload {
	summary := qaFullSummary{
		Status:              fullPlanStatus(plan, domain),
		Action:              "plan",
		SelectedLanes:       plan.SelectedLanes,
		MustLanes:           plan.BlockerRules.MustLanes,
		JourneyPackCount:    len(plan.JourneyPacks),
		SetupGapCount:       len(plan.SetupGaps),
		BlockingSetupGaps:   countBlockingSetupGaps(plan.SetupGaps),
		DomainScenarioCount: domainScenarioCount(domain),
		DomainSetupGap:      domain.Status != "ready",
	}
	return qaFullPayload{
		SchemaVersion:   qaFullSchemaVersion,
		Mode:            "plan",
		Profile:         plan.Profile,
		ProjectDir:      opts.ProjectDir,
		QAPolicy:        qaFullPolicyForPlan(plan),
		Bootstrap:       bootstrap,
		Summary:         summary,
		NextCommands:    qaFullNextCommands(opts, plan, domain),
		ReleasePlan:     &plan,
		DomainReadiness: domain,
	}
}

func buildQAFullRunPayload(opts qaFullOptions, result qarelease.ExecutionPayload, domain qaFullDomainReadiness, bootstrap *qascaffold.Result) qaFullPayload {
	status := "passed"
	if result.Status == qarelease.GateStatusWarn {
		status = "warn"
	}
	if result.Status == qarelease.GateStatusBlocked {
		status = "blocked"
	}
	if domain.Status != "ready" && status == "passed" {
		status = "setup_gap"
	}
	index := result.Index
	summary := buildQAFullRunSummary(status, result, domain)
	return qaFullPayload{
		SchemaVersion:   qaFullSchemaVersion,
		Mode:            "run",
		Profile:         result.Profile,
		ProjectDir:      opts.ProjectDir,
		QAPolicy:        qaFullPolicyForRun(result.Index),
		Bootstrap:       bootstrap,
		Summary:         summary,
		NextCommands:    qaFullRunNextCommands(opts, result, domain),
		ReleaseResult:   &index,
		ReleaseIndexRef: result.ReleaseIndexPath,
		DomainReadiness: domain,
	}
}

func buildQAFullSelectProjectPayload(opts qaFullOptions, targets []qascaffold.WorkspaceQATarget, hasChildRepos bool) qaFullPayload {
	candidates := make([]qaFullProjectCandidate, 0, len(targets))
	for _, target := range targets {
		candidates = append(candidates, qaFullProjectCandidate{
			ProjectDir: target.ProjectDir,
			RelPath:    target.RelPath,
			Score:      target.Score,
			Reason:     strings.Join(target.Reasons, ", "),
		})
	}
	action := "select_project"
	status := "setup_gap"
	if !hasChildRepos {
		action = "plan"
	}
	setupGap := "multiple project repositories detected; choose the project under test with --project-dir"
	if len(candidates) == 0 {
		setupGap = "multi-repo workspace detected, but no supported QA target candidates were found"
	}
	return qaFullPayload{
		SchemaVersion: qaFullSchemaVersion,
		Mode:          "select_project",
		Profile:       opts.Profile,
		ProjectDir:    opts.ProjectDir,
		QAPolicy:      qaFullPolicyForCandidates(candidates),
		Summary: qaFullSummary{
			Status: status,
			Action: action,
		},
		ProjectCandidates: candidates,
		NextCommands:      qaFullProjectCandidateCommands(opts, candidates),
		DomainReadiness: qaFullDomainReadiness{
			Status:   "setup_gap",
			SetupGap: setupGap,
		},
	}
}

func loadQAFullDomainReadiness(projectDir string) qaFullDomainReadiness {
	catalogPath := domainreadiness.ResolveCatalogPath(projectDir, domainreadiness.DefaultCatalogPath)
	catalog, err := domainreadiness.LoadCatalogFile(catalogPath)
	if err != nil {
		status := "error"
		setupGap := err.Error()
		if os.IsNotExist(err) {
			status = "setup_gap"
			setupGap = "domain readiness catalog is missing"
		}
		return qaFullDomainReadiness{Status: status, CatalogPath: catalogPath, SetupGap: setupGap}
	}
	plan, err := domainreadiness.CompileCatalog(catalog, domainreadiness.CompileOptions{ProjectDir: projectDir, Lane: "full"})
	if err != nil {
		return qaFullDomainReadiness{Status: "error", CatalogPath: catalogPath, SetupGap: err.Error()}
	}
	status := "ready"
	if !plan.Validation.Valid || len(plan.MissingDomains) > 0 || len(plan.RejectedScenarios) > 0 {
		status = "setup_gap"
	}
	return qaFullDomainReadiness{Status: status, CatalogPath: catalogPath, Plan: &plan}
}

func fullPlanStatus(plan qarelease.Plan, domain qaFullDomainReadiness) string {
	if countBlockingSetupGaps(plan.SetupGaps) > 0 {
		return "blocked"
	}
	if len(plan.SetupGaps) > 0 || domain.Status != "ready" {
		return "setup_gap"
	}
	return "ready"
}

func countBlockingSetupGaps(gaps []qarelease.SetupGapRow) int {
	count := 0
	for _, gap := range gaps {
		if gap.Blocking {
			count++
		}
	}
	return count
}

func domainScenarioCount(domain qaFullDomainReadiness) int {
	if domain.Plan == nil {
		return 0
	}
	return domain.Plan.ScenarioCount
}

func qaFullNextCommands(opts qaFullOptions, plan qarelease.Plan, domain qaFullDomainReadiness) []string {
	commands := []string{qaFullCommandString(qaFullOptions{ProjectDir: opts.ProjectDir, Profile: plan.Profile, Output: opts.Output, RunOutputRoot: opts.RunOutputRoot, Run: true, RuntimeProviders: opts.RuntimeProviders}, false)}
	if len(plan.JourneyPacks) == 0 || len(plan.SetupGaps) > 0 {
		commands = append(commands, qaFullCommandString(qaFullOptions{ProjectDir: opts.ProjectDir, Profile: plan.Profile, Output: opts.Output, RunOutputRoot: opts.RunOutputRoot, Bootstrap: true, RuntimeProviders: opts.RuntimeProviders}, false))
		commands = append(commands, "auto qa init --project-dir "+shellWord(opts.ProjectDir)+" --format json")
	}
	if domain.Status != "ready" {
		commands = append(commands, "auto qa domain-readiness init --project-dir "+shellWord(opts.ProjectDir)+" --format json")
	}
	return uniqueCommands(commands)
}

func qaFullRunNextCommands(opts qaFullOptions, result qarelease.ExecutionPayload, domain qaFullDomainReadiness) []string {
	commands := []string{}
	if result.Status == qarelease.GateStatusBlocked {
		commands = append(commands, "auto qa feedback --to codex --evidence <failed-manifest> --format json")
	}
	if domain.Status != "ready" {
		commands = append(commands, "auto qa domain-readiness init --project-dir "+shellWord(opts.ProjectDir)+" --format json")
	}
	return commands
}

func qaFullProjectCandidateCommands(opts qaFullOptions, candidates []qaFullProjectCandidate) []string {
	commands := []string{}
	for i, candidate := range candidates {
		if i >= 5 {
			break
		}
		project := candidate.RelPath
		if project == "" {
			project = candidate.ProjectDir
		}
		commands = append(commands, qaFullCommandString(qaFullOptions{ProjectDir: project, Profile: opts.Profile, Bootstrap: true, RuntimeProviders: opts.RuntimeProviders}, true))
		commands = append(commands, qaFullCommandString(qaFullOptions{ProjectDir: project, Profile: opts.Profile, RuntimeProviders: opts.RuntimeProviders}, true))
	}
	return uniqueCommands(commands)
}

func qaFullCommandString(opts qaFullOptions, jsonMode bool) string {
	parts := []string{"auto", "qa", "full"}
	if opts.Run {
		parts = append(parts, "--run")
	}
	if opts.Bootstrap {
		parts = append(parts, "--bootstrap")
	}
	if opts.ProjectDir != "" && opts.ProjectDir != "." {
		parts = append(parts, "--project-dir", shellWord(opts.ProjectDir))
	}
	if opts.Profile != "" && opts.Profile != "prelaunch" {
		parts = append(parts, "--profile", shellWord(opts.Profile))
	}
	if opts.Output != "" {
		parts = append(parts, "--output", shellWord(opts.Output))
	}
	if opts.RunOutputRoot != "" {
		parts = append(parts, "--run-output", shellWord(opts.RunOutputRoot))
	}
	if len(opts.RuntimeProviders) == 1 {
		parts = append(parts, "--runtime-provider", opts.RuntimeProviders[0])
	}
	if jsonMode {
		parts = append(parts, "--format", "json")
	}
	return strings.Join(parts, " ")
}

func shellWord(value string) string {
	if value == "" {
		return "''"
	}
	if !strings.ContainsAny(value, " \t\n'\"`$\\") {
		return value
	}
	return "'" + strings.ReplaceAll(value, "'", "'\\''") + "'"
}

func uniqueCommands(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
