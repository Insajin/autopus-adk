package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// readWorkflowYAML loads the committed eval-regression gate workflow relative to
// the internal/cli package directory.
func readWorkflowYAML(t *testing.T) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "workflows", "eval-regression-gate.yml"))
	if err != nil {
		t.Fatalf("S9: read workflow: %v", err)
	}
	return string(data)
}

// S9 — the hardened workflow pins the verifier and never silently skips. Concrete
// substring oracle over the committed yaml (SPEC-EVAL-REGRESSION-PROV-001,
// REQ-EVP-PIN-001 / REQ-EVP-GATE-001 / REQ-EVP-PINACT-001).
func TestEvalRegressionWorkflowPinsVerifierAndFailsClosed(t *testing.T) {
	yaml := readWorkflowYAML(t)

	// Trigger is present.
	if !strings.Contains(yaml, "pull_request") {
		t.Fatalf("S9: workflow must trigger on pull_request")
	}

	// L2: the verifier is obtained via a PINNED released binary, not a PR-head
	// build. Assert the pinned-install fetch is present.
	if !strings.Contains(yaml, "go install github.com/insajin/autopus-adk/cmd/auto@") {
		t.Fatalf("S9: workflow must fetch a pinned verifier via 'go install github.com/insajin/autopus-adk/cmd/auto@<tag>'")
	}

	// The removed L2 hole: the PR-head verifier build must be gone.
	if strings.Contains(yaml, "go build -o bin/auto ./cmd/auto") {
		t.Fatalf("S9: workflow must NOT build the verifier from PR head ('go build -o bin/auto ./cmd/auto')")
	}

	// No advisory/warn mode anywhere — a blocked verdict must fail the check.
	if strings.Contains(yaml, "--warn-only") {
		t.Fatalf("S9: workflow must contain NO --warn-only substring")
	}

	// No bare hashFiles() skip guarding the gate — a missing/unsigned artifact
	// must fail closed rather than be skipped.
	if strings.Contains(yaml, "hashFiles('.autopus/artifacts/eval_regression_report.json') != ''") {
		t.Fatalf("S9: workflow must NOT contain a bare hashFiles() skip guarding the gate")
	}

	// Pinned actions (REQ-EVP-PINACT-001): fixed major refs are present.
	if !strings.Contains(yaml, "actions/checkout@v4") || !strings.Contains(yaml, "actions/setup-go@v5") {
		t.Fatalf("S9: workflow must pin actions/checkout@v4 and actions/setup-go@v5")
	}
}

// S11 (Should, readiness) — the required-check readiness runbook exists, targets
// the branch-protection endpoint, and references the gate check context name
// (REQ-EVP-READY-001). Lightweight existence + substring assertion.
func TestEvalRegressionRequiredCheckRunbookExists(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".github", "EVAL_REGRESSION_REQUIRED_CHECK.md"))
	if err != nil {
		t.Fatalf("S11: read readiness runbook: %v", err)
	}
	doc := string(data)

	if !strings.Contains(doc, "branches/main/protection") {
		t.Fatalf("S11: runbook must reference the repos/:owner/:repo/branches/main/protection endpoint")
	}
	if !strings.Contains(doc, "required_status_checks[contexts][]") {
		t.Fatalf("S11: runbook must add the gate context to required_status_checks[contexts][]")
	}
	if !strings.Contains(doc, "eval-regression") {
		t.Fatalf("S11: runbook must reference the 'eval-regression' gate check context name")
	}
	if !strings.Contains(doc, "check-runs") {
		t.Fatalf("S11: runbook must instruct the operator to confirm the rendered context via check-runs")
	}
}
