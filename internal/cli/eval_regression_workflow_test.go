package cli

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func readRunbook(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "EVAL_REGRESSION_REQUIRED_CHECK.md"))
	if err != nil {
		t.Fatalf("read readiness runbook: %v", err)
	}
	return string(data)
}

func requireContainsAll(t *testing.T, subject string, required ...string) {
	t.Helper()
	for _, needle := range required {
		if !strings.Contains(subject, needle) {
			t.Fatalf("expected content to contain %q", needle)
		}
	}
}

func requireImmutableAction(t *testing.T, workflow, action string) {
	t.Helper()
	pattern := regexp.MustCompile(`(?m)^\s*uses:\s+` + regexp.QuoteMeta(action) + `@[0-9a-f]{40}\s*$`)
	if !pattern.MatchString(workflow) {
		t.Fatalf("expected %s to use an immutable 40-hex revision", action)
	}
}

func requireImmutableADKRevision(t *testing.T, workflow string) {
	t.Helper()
	if !regexp.MustCompile(`(?m)^\s*adk_revision="[0-9a-f]{40}"\s*$`).MatchString(workflow) {
		t.Fatal("Autopus gate must pin the ADK verifier to an immutable 40-hex revision")
	}
	if !strings.Contains(workflow, `go install "github.com/insajin/autopus-adk/cmd/auto@${adk_revision}"`) {
		t.Fatal("Autopus gate must install the pinned ADK verifier revision")
	}
}

// The adk repository owns the verifier library/CLI only. The live gate moved to
// the sibling Autopus repository, so keeping an adk workflow would create a
// dormant duplicate gate with stale trust assumptions.
func TestEvalRegressionADKWorkflowIsRetired(t *testing.T) {
	path := filepath.Join("..", "..", ".github", "workflows", "eval-regression-gate.yml")
	_, err := os.Stat(path)
	if err == nil {
		t.Fatalf("adk eval-regression workflow must be retired: %s still exists", path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stat adk eval-regression workflow: %v", err)
	}
}

// S9/S13 review oracle: when the sibling Autopus gate YAML is present, validate
// the exact same-SHA replay-safe fetch shape. When this adk checkout cannot see
// that sibling YAML, require the runbook to carry the same dry-run oracle so the
// gate review remains deterministic and network-free.
func TestEvalRegressionAutopusGateFetchSelectionReviewOracle(t *testing.T) {
	siblingGate := filepath.Join("..", "..", "..", "Autopus", ".github", "workflows", "eval-regression-gate.yml")
	data, err := os.ReadFile(siblingGate)
	if err == nil {
		assertAutopusGateFetchSelection(t, string(data))
		assertAutopusProducerTrustedCheckout(t)
		return
	}
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("read sibling Autopus gate workflow: %v", err)
	}

	doc := readRunbook(t)
	requireContainsAll(t, doc,
		"Autopus/.github/workflows/eval-regression-gate.yml",
		"gh run list",
		"--workflow eval-regression-producer.yml",
		"--event pull_request_target",
		"--status success",
		"--json databaseId,displayTitle,createdAt",
		"Eval Regression Producer PR #<pr_number> head <head_sha>",
		".displayTitle == $expected",
		"gh run download <selected_run_id>",
		"eval-regression-report-pr-<pr_number>-<head_sha>",
		"artifact_missing",
		"same-SHA",
	)
}

func assertAutopusProducerTrustedCheckout(t *testing.T) {
	t.Helper()
	siblingProducer := filepath.Join("..", "..", "..", "Autopus", ".github", "workflows", "eval-regression-producer.yml")
	data, err := os.ReadFile(siblingProducer)
	if err != nil {
		t.Fatalf("read sibling Autopus producer workflow: %v", err)
	}
	yaml := string(data)
	requireContainsAll(t, yaml,
		"pull_request_target:",
		"Checkout trusted base producer code",
		"ref: ${{ github.event.pull_request.base.sha }}",
		"persist-credentials: false",
		"github.event.pull_request.head.repo.full_name == github.repository",
		"EVAL_REGRESSION_SOURCE_REVISION: ${{ github.event.pull_request.head.sha }}",
		"go run ./cmd/eval-regression-export",
		"name: eval-regression-report-pr-${{ github.event.pull_request.number }}-${{ github.event.pull_request.head.sha }}",
		"eval_regression_report.v1",
		"eval_regression_attestation.v2",
	)
	requireImmutableAction(t, yaml, "actions/checkout")
	requireImmutableAction(t, yaml, "actions/setup-go")
	requireImmutableAction(t, yaml, "actions/upload-artifact")
	if strings.Contains(yaml, "ref: ${{ github.event.pull_request.head.sha }}") ||
		strings.Contains(yaml, "ref: ${{ env.HEAD_SHA }}") {
		t.Fatalf("Autopus producer workflow must not run PR-head producer code with signing secrets")
	}
	if strings.Contains(yaml, "workflow_dispatch:") || strings.Contains(yaml, "\n  pull_request:\n") {
		t.Fatalf("Autopus producer workflow must use only pull_request_target and expose no workflow_dispatch")
	}
}

func assertAutopusGateFetchSelection(t *testing.T, yaml string) {
	t.Helper()
	requireContainsAll(t, yaml,
		"pull_request_target:",
		"gh run list",
		"--workflow eval-regression-producer.yml",
		"--event pull_request_target",
		"--json databaseId,displayTitle,createdAt",
		"EXPECTED_PRODUCER_RUN:",
		"Eval Regression Producer PR #${{ github.event.pull_request.number }} head ${{ github.event.pull_request.head.sha }}",
		".displayTitle == $expected",
		"github.event.pull_request.head.sha",
		"--status success",
		"run-id: ${{ steps.trusted-run.outputs.run_id }}",
		"github-token: ${{ github.token }}",
		"eval_regression_report.v1",
		"eval_regression_attestation.v2",
		"auto check --eval-regression",
	)
	requireImmutableAction(t, yaml, "actions/setup-go")
	requireImmutableAction(t, yaml, "actions/download-artifact")
	requireImmutableADKRevision(t, yaml)
	if strings.Contains(yaml, "actions/checkout@") || strings.Contains(yaml, "--commit") {
		t.Fatalf("Autopus gate must not execute repository code or select pull_request_target runs by base commit")
	}
	if strings.Contains(yaml, "--warn-only") {
		t.Fatalf("Autopus gate workflow must not contain --warn-only")
	}
	if strings.Contains(yaml, "hashFiles('.autopus/artifacts/eval_regression_report.json') != ''") {
		t.Fatalf("Autopus gate workflow must not skip on hashFiles; absence must fail closed")
	}
}

// S12 — the required-check readiness runbook exists, targets the Autopus repo,
// and carries the operator-only key, secret, branch-protection, and workflow-path
// enforcement checklist. The test is a reviewable dry-run: it performs no
// privileged GitHub action and invokes no network command.
func TestEvalRegressionRequiredCheckRunbookExists(t *testing.T) {
	doc := readRunbook(t)

	requireContainsAll(t, doc,
		"Autopus repo",
		"branches/main/protection",
		"required_status_checks[contexts][]",
		"eval-regression",
		"check-runs",
		"EVAL_REGRESSION_SIGNING_KEY",
		"EVAL_REGRESSION_SIGNING_KEY_ID",
		"DB/network credentials",
		"key_id",
		"rotation overlap",
		"CODEOWNERS",
		"org ruleset",
		".github/workflows/**",
	)
}
