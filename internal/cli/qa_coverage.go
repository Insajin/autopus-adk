package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	qarelease "github.com/insajin/autopus-adk/pkg/qa/release"
	qarun "github.com/insajin/autopus-adk/pkg/qa/run"
)

const qaCoverageSchemaVersion = "qamesh.coverage.v1"

type qaCoverageOptions struct {
	ProjectDir       string
	RunIndexPath     string
	ReleaseIndexPath string
	JSONOut          bool
	Format           string
}

type qaCoveragePayload struct {
	SchemaVersion   string                `json:"schema_version"`
	ProjectDir      string                `json:"project_dir"`
	Status          string                `json:"status"`
	RunIndexRef     string                `json:"run_index_ref,omitempty"`
	ReleaseIndexRef string                `json:"release_index_ref,omitempty"`
	Summary         qaCoverageSummary     `json:"summary"`
	Lanes           []qaCoverageLane      `json:"lanes,omitempty"`
	Journeys        []qaCoverageJourney   `json:"journeys,omitempty"`
	DomainReadiness qaFullDomainReadiness `json:"domain_readiness"`
	NextCommands    []string              `json:"next_commands"`
	manifestSeen    map[string]bool       `json:"-"`
}

type qaCoverageSummary struct {
	LaneCount           int `json:"lane_count"`
	JourneyCount        int `json:"journey_count"`
	ManifestCount       int `json:"manifest_count"`
	FailedCheckCount    int `json:"failed_check_count"`
	SetupGapCount       int `json:"setup_gap_count"`
	DomainScenarioCount int `json:"domain_scenario_count"`
}

type qaCoverageLane struct {
	Lane          string `json:"lane"`
	Policy        string `json:"policy"`
	Status        string `json:"status"`
	Verdict       string `json:"verdict"`
	RunIndexPath  string `json:"run_index_path,omitempty"`
	ManifestCount int    `json:"manifest_count"`
}

type qaCoverageJourney struct {
	JourneyID    string `json:"journey_id"`
	Lane         string `json:"lane,omitempty"`
	Adapter      string `json:"adapter,omitempty"`
	Status       string `json:"status,omitempty"`
	ManifestPath string `json:"manifest_path,omitempty"`
	Source       string `json:"source,omitempty"`
}

func newQACoverageCmd() *cobra.Command {
	var opts qaCoverageOptions
	cmd := &cobra.Command{
		Use:   "coverage",
		Short: "Summarize QAMESH lane, journey, evidence, and domain readiness coverage",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQACoverage(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", ".", "Project directory")
	cmd.Flags().StringVar(&opts.RunIndexPath, "run-index", "", "QAMESH run index path; defaults to latest")
	cmd.Flags().StringVar(&opts.ReleaseIndexPath, "release-index", "", "QAMESH release index path; defaults to latest")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

func runQACoverage(cmd *cobra.Command, opts qaCoverageOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	payload, err := buildQACoveragePayload(opts)
	if err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_coverage_failed", map[string]any{"project_dir": opts.ProjectDir})
	}
	if jsonMode {
		status := jsonStatusOK
		if payload.Status != "ready" {
			status = jsonStatusWarn
		}
		return writeJSONResult(cmd, status, payload, nil, nil)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "qa coverage %s project=%s lanes=%d journeys=%d manifests=%d setup_gaps=%d failed_checks=%d\n", payload.Status, payload.ProjectDir, payload.Summary.LaneCount, payload.Summary.JourneyCount, payload.Summary.ManifestCount, payload.Summary.SetupGapCount, payload.Summary.FailedCheckCount)
	for _, next := range payload.NextCommands {
		fmt.Fprintf(cmd.OutOrStdout(), "next: %s\n", next)
	}
	return nil
}

func buildQACoveragePayload(opts qaCoverageOptions) (qaCoveragePayload, error) {
	var runIndex qarun.Index
	var releaseIndex qarelease.Index
	runPath, err := resolveCoverageIndex(opts.ProjectDir, opts.RunIndexPath, filepath.Join(".autopus", "qa", "runs"), "run-index.json")
	if err != nil {
		return qaCoveragePayload{}, err
	}
	releasePath, err := resolveCoverageIndex(opts.ProjectDir, opts.ReleaseIndexPath, filepath.Join(".autopus", "qa", "releases"), "release-index.json")
	if err != nil {
		return qaCoveragePayload{}, err
	}
	hasRun, err := loadCoverageJSON(runPath, &runIndex)
	if err != nil {
		return qaCoveragePayload{}, err
	}
	hasRelease, err := loadCoverageJSON(releasePath, &releaseIndex)
	if err != nil {
		return qaCoveragePayload{}, err
	}
	domain := loadQAFullDomainReadiness(opts.ProjectDir)
	payload := qaCoveragePayload{
		SchemaVersion:   qaCoverageSchemaVersion,
		ProjectDir:      opts.ProjectDir,
		RunIndexRef:     runPath,
		ReleaseIndexRef: releasePath,
		DomainReadiness: domain,
		manifestSeen:    map[string]bool{},
	}
	if hasRun {
		addRunCoverage(&payload, runIndex)
	}
	if hasRelease {
		addReleaseCoverage(&payload, releaseIndex)
	}
	payload.Summary.LaneCount = len(payload.Lanes)
	payload.Summary.JourneyCount = len(payload.Journeys)
	payload.Summary.DomainScenarioCount = domainScenarioCount(domain)
	payload.Status = qaCoverageStatus(payload, hasRun, hasRelease)
	payload.NextCommands = qaCoverageNextCommands(opts, payload)
	return payload, nil
}

func resolveCoverageIndex(projectDir, explicit, relDir, filename string) (string, error) {
	if strings.TrimSpace(explicit) != "" {
		return explicit, nil
	}
	return latestIndexPath(projectDir, relDir, filename)
}

func loadCoverageJSON(path string, target any) (bool, error) {
	if strings.TrimSpace(path) == "" {
		return false, nil
	}
	body, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, json.Unmarshal(body, target)
}

func addRunCoverage(payload *qaCoveragePayload, index qarun.Index) {
	addCoverageManifests(payload, index.ManifestPaths)
	payload.Summary.SetupGapCount += len(index.SetupGaps)
	for _, check := range index.Checks {
		if check.Status != "" && check.Status != "passed" && check.Status != "skipped" {
			payload.Summary.FailedCheckCount++
		}
		if check.JourneyID != "" {
			payload.Journeys = upsertCoverageJourney(payload.Journeys, qaCoverageJourney{JourneyID: check.JourneyID, Lane: index.Lane, Adapter: check.Adapter, Status: check.Status, Source: "run"})
		}
	}
	for _, result := range index.AdapterResults {
		addCoverageManifests(payload, []string{result.QAMESHManifestPath})
		payload.Journeys = upsertCoverageJourney(payload.Journeys, qaCoverageJourney{JourneyID: result.JourneyID, Lane: index.Lane, Adapter: result.Adapter, Status: result.Status, ManifestPath: result.QAMESHManifestPath, Source: "run"})
	}
}

func addReleaseCoverage(payload *qaCoveragePayload, index qarelease.Index) {
	payload.Summary.SetupGapCount += len(index.SetupGaps)
	for _, row := range index.LaneRows {
		manifestCount := addCoverageManifests(payload, row.ManifestPaths)
		payload.Lanes = append(payload.Lanes, qaCoverageLane{
			Lane:          row.Lane,
			Policy:        string(row.LanePolicy),
			Status:        string(row.Status),
			Verdict:       string(row.LaneVerdict),
			RunIndexPath:  row.RunIndexPath,
			ManifestCount: manifestCount,
		})
		addReleaseJourneyCoverage(payload, row)
	}
}

func addCoverageManifests(payload *qaCoveragePayload, paths []string) int {
	count := 0
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" || payload.manifestSeen[path] {
			continue
		}
		payload.manifestSeen[path] = true
		payload.Summary.ManifestCount++
		count++
	}
	return count
}

func qaCoverageStatus(payload qaCoveragePayload, hasRun, hasRelease bool) string {
	if !hasRun && !hasRelease {
		return "setup_gap"
	}
	if payload.Summary.FailedCheckCount > 0 || coverageHasFailedLane(payload.Lanes) {
		return "failed"
	}
	if payload.Summary.JourneyCount > 0 && payload.Summary.ManifestCount == 0 {
		return "missing_evidence"
	}
	if payload.Summary.SetupGapCount > 0 || payload.DomainReadiness.Status != "ready" {
		return "setup_gap"
	}
	return "ready"
}

func coverageHasFailedLane(lanes []qaCoverageLane) bool {
	for _, lane := range lanes {
		switch lane.Status {
		case "failed", "blocked":
			return true
		}
	}
	return false
}

func qaCoverageNextCommands(opts qaCoverageOptions, payload qaCoveragePayload) []string {
	commands := []string{qaFullCommandString(qaFullOptions{ProjectDir: opts.ProjectDir}, true)}
	if payload.Status == "setup_gap" {
		commands = append(commands, qaFullCommandString(qaFullOptions{ProjectDir: opts.ProjectDir, Bootstrap: true}, true))
	}
	if payload.Status == "missing_evidence" || payload.Status == "failed" {
		commands = append(commands, qaFullCommandString(qaFullOptions{ProjectDir: opts.ProjectDir, Run: true}, true))
	}
	return uniqueCommands(commands)
}

func upsertCoverageJourney(values []qaCoverageJourney, next qaCoverageJourney) []qaCoverageJourney {
	if strings.TrimSpace(next.JourneyID) == "" {
		return values
	}
	for i, existing := range values {
		if existing.JourneyID == next.JourneyID {
			if next.Lane != "" {
				values[i].Lane = next.Lane
			}
			if next.Adapter != "" {
				values[i].Adapter = next.Adapter
			}
			if next.Status != "" {
				values[i].Status = next.Status
			}
			if next.ManifestPath != "" {
				values[i].ManifestPath = next.ManifestPath
			}
			if next.Source != "" {
				values[i].Source = next.Source
			}
			return values
		}
	}
	return append(values, next)
}
