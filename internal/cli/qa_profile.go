package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

const qaProfileCheckSchemaVersion = "qamesh.profile_check.v1"

type qaProfileCheckOptions struct {
	ProjectDir string
	Profile    string
	JSONOut    bool
	Format     string
}

type qaProfileCheckPayload struct {
	SchemaVersion         string                  `json:"schema_version"`
	ProjectDir            string                  `json:"project_dir"`
	Profile               string                  `json:"profile"`
	Status                string                  `json:"status"`
	JourneyCount          int                     `json:"journey_count"`
	AvailableCapabilities []string                `json:"available_capabilities"`
	RequiredCapabilities  []string                `json:"required_capabilities"`
	MissingCapabilities   []string                `json:"missing_capabilities,omitempty"`
	Journeys              []qaProfileCheckJourney `json:"journeys,omitempty"`
	NextCommands          []string                `json:"next_commands"`
}

type qaProfileCheckJourney struct {
	JourneyID            string   `json:"journey_id"`
	RequiredCapabilities []string `json:"required_capabilities,omitempty"`
	MissingCapabilities  []string `json:"missing_capabilities,omitempty"`
	Status               string   `json:"status"`
}

func newQAProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Inspect QA test profile capability readiness",
	}
	cmd.AddCommand(newQAProfileCheckCmd())
	return cmd
}

func newQAProfileCheckCmd() *cobra.Command {
	var opts qaProfileCheckOptions
	cmd := &cobra.Command{
		Use:   "check",
		Short: "Check Journey Pack capability requirements against a test profile",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runQAProfileCheck(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.ProjectDir, "project-dir", ".", "Project directory")
	cmd.Flags().StringVar(&opts.Profile, "profile", config.TestProfileCI, "Test profile (standalone|local|ci|prod)")
	addJSONFlags(cmd, &opts.JSONOut, &opts.Format)
	return cmd
}

func runQAProfileCheck(cmd *cobra.Command, opts qaProfileCheckOptions) error {
	jsonMode, err := resolveJSONMode(opts.JSONOut, opts.Format)
	if err != nil {
		return err
	}
	payload, err := buildQAProfileCheckPayload(opts)
	if err != nil {
		return qaCommandError(cmd, jsonMode, err, "qa_profile_check_failed", map[string]any{"project_dir": opts.ProjectDir, "profile": opts.Profile})
	}
	if jsonMode {
		status := jsonStatusOK
		if payload.Status != "ready" {
			status = jsonStatusWarn
		}
		return writeJSONResult(cmd, status, payload, nil, nil)
	}
	fmt.Fprintf(cmd.OutOrStdout(), "qa profile %s profile=%s journeys=%d missing=%d\n", payload.Status, payload.Profile, payload.JourneyCount, len(payload.MissingCapabilities))
	for _, next := range payload.NextCommands {
		fmt.Fprintf(cmd.OutOrStdout(), "next: %s\n", next)
	}
	return nil
}

func buildQAProfileCheckPayload(opts qaProfileCheckOptions) (qaProfileCheckPayload, error) {
	profile := strings.ToLower(strings.TrimSpace(opts.Profile))
	if !config.IsValidTestProfile(profile) {
		return qaProfileCheckPayload{}, fmt.Errorf("invalid test profile %q", opts.Profile)
	}
	cfg, err := config.LoadPreview(opts.ProjectDir)
	if err != nil {
		return qaProfileCheckPayload{}, err
	}
	packs, err := journey.LoadDir(opts.ProjectDir)
	if err != nil {
		return qaProfileCheckPayload{}, err
	}
	available := sortedUnique(cfg.AvailableTestCapabilities(profile))
	availableSet := stringSet(available)
	required := []string{}
	missing := []string{}
	rows := make([]qaProfileCheckJourney, 0, len(packs))
	for _, pack := range packs {
		req := sortedUnique(pack.ProfileRequirements.Capabilities)
		rowMissing := missingCapabilities(req, availableSet)
		required = append(required, req...)
		missing = append(missing, rowMissing...)
		status := "ready"
		if len(rowMissing) > 0 {
			status = "setup_gap"
		}
		rows = append(rows, qaProfileCheckJourney{JourneyID: pack.ID, RequiredCapabilities: req, MissingCapabilities: rowMissing, Status: status})
	}
	payload := qaProfileCheckPayload{
		SchemaVersion:         qaProfileCheckSchemaVersion,
		ProjectDir:            opts.ProjectDir,
		Profile:               profile,
		JourneyCount:          len(packs),
		AvailableCapabilities: available,
		RequiredCapabilities:  sortedUnique(required),
		MissingCapabilities:   sortedUnique(missing),
		Journeys:              rows,
	}
	payload.Status = "ready"
	if payload.JourneyCount == 0 || len(payload.MissingCapabilities) > 0 {
		payload.Status = "setup_gap"
	}
	payload.NextCommands = qaProfileCheckNextCommands(opts, payload)
	return payload, nil
}

func qaProfileCheckNextCommands(opts qaProfileCheckOptions, payload qaProfileCheckPayload) []string {
	commands := []string{}
	if payload.JourneyCount == 0 {
		commands = append(commands, qaFullCommandString(qaFullOptions{ProjectDir: opts.ProjectDir, Bootstrap: true}, true))
	}
	if len(payload.MissingCapabilities) > 0 && payload.Profile != config.TestProfileLocal {
		commands = append(commands, "auto qa profile check --project-dir "+shellWord(opts.ProjectDir)+" --profile local --format json")
	}
	commands = append(commands, qaFullCommandString(qaFullOptions{ProjectDir: opts.ProjectDir}, true))
	return uniqueCommands(commands)
}

func missingCapabilities(required []string, available map[string]bool) []string {
	missing := []string{}
	for _, value := range required {
		if !available[strings.ToLower(strings.TrimSpace(value))] {
			missing = append(missing, value)
		}
	}
	return sortedUnique(missing)
}

func stringSet(values []string) map[string]bool {
	set := map[string]bool{}
	for _, value := range values {
		set[strings.ToLower(strings.TrimSpace(value))] = true
	}
	return set
}

func sortedUnique(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		normalized := strings.ToLower(strings.TrimSpace(value))
		if normalized == "" || seen[normalized] {
			continue
		}
		seen[normalized] = true
		out = append(out, normalized)
	}
	sort.Strings(out)
	return out
}
