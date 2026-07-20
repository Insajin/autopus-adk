package companionmanifest

import (
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

type ciStabilityStep struct {
	Name string         `yaml:"name"`
	Uses string         `yaml:"uses"`
	Run  string         `yaml:"run"`
	With map[string]any `yaml:"with"`
}

type ciStabilityJob struct {
	Name           string            `yaml:"name"`
	RunsOn         string            `yaml:"runs-on"`
	TimeoutMinutes int               `yaml:"timeout-minutes"`
	Steps          []ciStabilityStep `yaml:"steps"`
}

type ciStabilityWorkflow struct {
	Jobs map[string]ciStabilityJob `yaml:"jobs"`
}

func TestCIWorkflow_StableChecksHaveBoundedTimeouts(t *testing.T) {
	workflow := readCIStabilityWorkflow(t, ".github/workflows/ci.yaml")
	want := map[string]int{
		"test": 35, "e2e": 10, "lint": 10, "static-contracts": 10, "macos-runtime": 15,
	}
	for id, timeout := range want {
		job, ok := workflow.Jobs[id]
		if !ok {
			t.Fatalf("CI job %s is missing", id)
		}
		if job.Name != id {
			t.Fatalf("CI job %s check name = %q, want stable %q", id, job.Name, id)
		}
		if job.TimeoutMinutes != timeout {
			t.Fatalf("CI job %s timeout = %d, want %d", id, job.TimeoutMinutes, timeout)
		}
	}
	linter := ciStep(t, workflow.Jobs["lint"], "golangci-lint")
	if linter.Uses != "golangci/golangci-lint-action@1e7e51e771db61008b38414a730f564565cf7c20" ||
		linter.With["version"] != "v2.12.2" {
		t.Fatalf("golangci-lint gate drifted: uses=%q version=%v", linter.Uses, linter.With["version"])
	}
	testRun := ciStepRun(t, workflow.Jobs["test"], "Test with Coverage")
	for _, required := range []string{"-timeout=25m", "-tags integration", "-coverprofile=coverage.out", "COVERAGE_THRESHOLD: \"83\""} {
		if !strings.Contains(readReleaseFile(t, ".github/workflows/ci.yaml"), required) &&
			!strings.Contains(testRun, required) {
			t.Fatalf("CI coverage contract missing %q", required)
		}
	}
}

func TestCIWorkflow_StaticAndMacOSContractsArePullRequestSafe(t *testing.T) {
	workflow := readCIStabilityWorkflow(t, ".github/workflows/ci.yaml")
	for _, id := range []string{"test", "macos-runtime"} {
		checkout := ciUsesStep(t, workflow.Jobs[id], "actions/checkout@")
		if checkout.With["fetch-depth"] != 0 {
			t.Fatalf("CI job %s checkout fetch-depth = %v, want 0 for release ancestry checks",
				id, checkout.With["fetch-depth"])
		}
	}
	static := workflow.Jobs["static-contracts"]
	if run := ciStepRun(t, static, "Validate GitHub Actions workflows"); run != "go run github.com/rhysd/actionlint/cmd/actionlint@v1.7.7" {
		t.Fatalf("actionlint command = %q", run)
	}
	goReleaser := ciStep(t, static, "Validate GoReleaser configuration")
	if goReleaser.Uses != "goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94" ||
		goReleaser.With["version"] != "v2.17.0" || goReleaser.With["args"] != "check" {
		t.Fatalf("GoReleaser static gate drifted: uses=%q with=%v", goReleaser.Uses, goReleaser.With)
	}
	shellRun := ciStepRun(t, static, "Validate shell syntax")
	for _, required := range []string{"git ls-files -z '*.sh'", `bash -n "$script"`} {
		if !strings.Contains(shellRun, required) {
			t.Fatalf("shell static gate missing %q", required)
		}
	}
	macOS := workflow.Jobs["macos-runtime"]
	macOSRaw := readCIStabilityJob(t, ".github/workflows/ci.yaml", "macos-runtime")
	if macOS.RunsOn != "macos-14" || strings.Contains(macOSRaw, "${{ secrets.") {
		t.Fatalf("macOS PR runtime boundary runner=%q contains-secrets=%t",
			macOS.RunsOn, strings.Contains(macOSRaw, "${{ secrets."))
	}
	macRun := ciStepRun(t, macOS, "Test macOS companion runtime contracts")
	for _, required := range []string{"-timeout=8m", "./internal/companionmanifest/...", "./pkg/companionmanifest/..."} {
		if !strings.Contains(macRun, required) {
			t.Fatalf("macOS runtime gate missing %q", required)
		}
	}
	if strings.Contains(macRun, "-tags integration") {
		t.Fatal("macOS PR runtime gate must not run networked executable release fixtures")
	}
}

func TestSecurityWorkflow_JobsHaveBoundedTimeouts(t *testing.T) {
	workflow := readCIStabilityWorkflow(t, ".github/workflows/security.yml")
	want := map[string]int{"gitleaks": 10, "govulncheck": 15}
	for id, timeout := range want {
		job := workflow.Jobs[id]
		if job.TimeoutMinutes != timeout {
			t.Fatalf("security job %s timeout = %d, want %d", id, job.TimeoutMinutes, timeout)
		}
	}
	install := ciStepRun(t, workflow.Jobs["govulncheck"], "Install govulncheck")
	if install != "go install golang.org/x/vuln/cmd/govulncheck@v1.6.0" {
		t.Fatalf("govulncheck install command = %q", install)
	}
	for path, mutable := range map[string]string{
		".github/workflows/ci.yaml":      "version: latest",
		".github/workflows/security.yml": "govulncheck@latest",
	} {
		if strings.Contains(readReleaseFile(t, path), mutable) {
			t.Fatalf("%s reintroduced mutable selector %q", path, mutable)
		}
	}
}

func readCIStabilityWorkflow(t *testing.T, path string) ciStabilityWorkflow {
	t.Helper()
	var workflow ciStabilityWorkflow
	if err := yaml.Unmarshal([]byte(readReleaseFile(t, path)), &workflow); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	return workflow
}

func readCIStabilityJob(t *testing.T, path, id string) string {
	t.Helper()
	var workflow struct {
		Jobs map[string]any `yaml:"jobs"`
	}
	if err := yaml.Unmarshal([]byte(readReleaseFile(t, path)), &workflow); err != nil {
		t.Fatalf("parse %s: %v", path, err)
	}
	job, ok := workflow.Jobs[id]
	if !ok {
		t.Fatalf("CI job %s is missing", id)
	}
	encoded, err := yaml.Marshal(job)
	if err != nil {
		t.Fatalf("encode CI job %s: %v", id, err)
	}
	return string(encoded)
}

func ciStepRun(t *testing.T, job ciStabilityJob, name string) string {
	t.Helper()
	return ciStep(t, job, name).Run
}

func ciStep(t *testing.T, job ciStabilityJob, name string) ciStabilityStep {
	t.Helper()
	for _, step := range job.Steps {
		if step.Name == name {
			return step
		}
	}
	t.Fatalf("CI step %q is missing", name)
	return ciStabilityStep{}
}

func ciUsesStep(t *testing.T, job ciStabilityJob, prefix string) ciStabilityStep {
	t.Helper()
	for _, step := range job.Steps {
		if strings.HasPrefix(step.Uses, prefix) {
			return step
		}
	}
	t.Fatalf("CI uses step %q is missing", prefix)
	return ciStabilityStep{}
}
