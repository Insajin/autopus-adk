package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

type canaryOptions struct {
	projectDir  string
	url         string
	frontendURL string
	apiURL      string
	watch       string
	compare     string
	dryRun      bool
	jsonOut     bool
	format      string
}

const (
	defaultCanaryFrontendURL = "https://staging.autopus.co"
	defaultCanaryAPIURL      = "https://autopus-staging.up.railway.app"
)

type canaryTargets struct {
	FrontendURL string
	APIURL      string
}

type canaryResult struct {
	Timestamp string               `json:"timestamp"`
	Project   string               `json:"project"`
	Mode      string               `json:"mode"`
	Verdict   string               `json:"verdict"`
	Build     string               `json:"build"`
	E2E       string               `json:"e2e"`
	Doctor    string               `json:"doctor"`
	Endpoint  string               `json:"endpoint"`
	Browser   string               `json:"browser"`
	Targets   []canaryTargetResult `json:"targets,omitempty"`
	Skipped   []canarySkippedCheck `json:"skipped,omitempty"`
	Flags     map[string]string    `json:"flags,omitempty"`
	Summary   map[string]string    `json:"summary"`
}

type canaryTargetResult struct {
	ID      string `json:"id"`
	Command string `json:"command,omitempty"`
	Status  string `json:"status"`
	Detail  string `json:"detail,omitempty"`
}

type canarySkippedCheck struct {
	Area   string `json:"area"`
	Reason string `json:"reason"`
}

func newCanaryCmd() *cobra.Command {
	opts := canaryOptions{}
	cmd := &cobra.Command{
		Use:   "canary",
		Short: "Run post-deploy health checks",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCanaryCmd(cmd, opts)
		},
	}
	cmd.Flags().StringVar(&opts.projectDir, "project-dir", ".", "Project root directory")
	cmd.Flags().StringVar(&opts.url, "url", "", "Legacy deployment URL override for both endpoint and browser checks")
	cmd.Flags().StringVar(&opts.frontendURL, "frontend-url", defaultCanaryFrontendURL, "Frontend URL for browser checks")
	cmd.Flags().StringVar(&opts.apiURL, "api-url", defaultCanaryAPIURL, "API URL for endpoint checks")
	cmd.Flags().StringVar(&opts.watch, "watch", "", "Repeat interval such as 5m (reserved)")
	cmd.Flags().StringVar(&opts.compare, "compare", "", "Commit SHA to compare against (reserved)")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Plan canary checks without executing project commands")
	addJSONFlags(cmd, &opts.jsonOut, &opts.format)
	return cmd
}

func runCanaryCmd(cmd *cobra.Command, opts canaryOptions) error {
	jsonMode, err := resolveJSONMode(opts.jsonOut, opts.format)
	if err != nil {
		return err
	}
	result, err := runCanary(cmd.Context(), opts)
	if err != nil && result.Verdict != "FAIL" {
		result.Verdict = "FAIL"
		result.Summary = canarySummary(result)
	}
	if jsonMode {
		status := jsonStatusOK
		if result.Verdict == "WARN" {
			status = jsonStatusWarn
		}
		if result.Verdict == "FAIL" {
			return writeJSONResultAndExit(cmd, jsonStatusError, errOrDefault(err, "canary failed"), "canary_failed", result, nil, canaryChecks(result))
		}
		return writeJSONResult(cmd, status, result, nil, canaryChecks(result))
	}
	printCanaryText(cmd, result)
	if result.Verdict == "FAIL" {
		return errOrDefault(err, "canary failed")
	}
	return nil
}

func runCanary(ctx context.Context, opts canaryOptions) (canaryResult, error) {
	projectDir, err := filepath.Abs(defaultString(opts.projectDir, "."))
	if err != nil {
		return canaryResult{}, err
	}
	targets := resolveCanaryTargets(opts)
	result := canaryResult{
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Project:   filepath.Base(projectDir),
		Mode:      "staging-canary",
		Verdict:   "PASS",
		Build:     "PASS",
		E2E:       "PASS",
		Doctor:    "PASS",
		Endpoint:  "SKIPPED",
		Browser:   "SKIPPED",
		Flags: map[string]string{
			"url":          opts.url,
			"frontend_url": targets.FrontendURL,
			"api_url":      targets.APIURL,
			"watch":        opts.watch,
			"compare":      opts.compare,
		},
	}
	if opts.dryRun {
		result.Mode = "dry-run"
		result.Build = "SKIPPED"
		result.E2E = "SKIPPED"
		result.Doctor = "SKIPPED"
		result.Skipped = append(result.Skipped,
			canarySkippedCheck{"build", "dry-run"},
			canarySkippedCheck{"e2e", "dry-run"},
			canarySkippedCheck{"doctor", "dry-run"},
			canarySkippedCheck{"endpoint", "dry-run"},
			canarySkippedCheck{"browser", "dry-run"},
		)
		result.Summary = canarySummary(result)
		return result, writeCanaryLatest(projectDir, result)
	}

	for _, target := range canaryBuildTargets(projectDir) {
		run := runCanaryExternal(ctx, target.ID, target.Command, target.Dir, target.Args...)
		result.Targets = append(result.Targets, run)
		if run.Status != "PASS" {
			result.Build = "FAIL"
			result.Verdict = "FAIL"
			result.Summary = canarySummary(result)
			_ = writeCanaryLatest(projectDir, result)
			return result, fmt.Errorf("%s failed: %s", run.ID, run.Detail)
		}
	}

	exe, _ := os.Executable()
	if exe == "" {
		exe = "auto"
	}
	for _, target := range []struct {
		id   string
		args []string
	}{
		{"S1", []string{"test", "run", "--project-dir", projectDir, "--scenario", "version", "--format", "json", "--timeout", "60s"}},
		{"doctor", []string{"doctor"}},
	} {
		run := runCanaryExternal(ctx, target.id, exe+" "+strings.Join(target.args, " "), projectDir, append([]string{exe}, target.args...)...)
		result.Targets = append(result.Targets, run)
		if run.Status != "PASS" {
			if target.id == "doctor" {
				result.Doctor = "FAIL"
			} else {
				result.E2E = "FAIL"
			}
			result.Verdict = "FAIL"
			result.Summary = canarySummary(result)
			_ = writeCanaryLatest(projectDir, result)
			return result, fmt.Errorf("%s failed: %s", run.ID, run.Detail)
		}
	}

	result.Endpoint = runCanaryEndpointChecks(ctx, targets.APIURL, &result)
	result.Browser = runCanaryRemoteBrowser(ctx, projectDir, targets.FrontendURL, &result)
	if result.Endpoint == "FAIL" || result.Browser == "FAIL" {
		result.Verdict = "FAIL"
	}
	result.Summary = canarySummary(result)
	return result, writeCanaryLatest(projectDir, result)
}

func resolveCanaryTargets(opts canaryOptions) canaryTargets {
	frontendURL := defaultString(opts.frontendURL, defaultCanaryFrontendURL)
	apiURL := defaultString(opts.apiURL, defaultCanaryAPIURL)
	if legacyURL := strings.TrimSpace(opts.url); legacyURL != "" {
		frontendURL = legacyURL
		apiURL = legacyURL
	}
	return canaryTargets{
		FrontendURL: strings.TrimRight(frontendURL, "/"),
		APIURL:      strings.TrimRight(apiURL, "/"),
	}
}
