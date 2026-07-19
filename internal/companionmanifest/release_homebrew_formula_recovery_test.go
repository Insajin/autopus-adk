package companionmanifest

import (
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

const formulaRecoveryWorkflow = ".github/workflows/homebrew-formula-bridge-recovery.yaml"

type recoveryDispatch struct {
	Inputs map[string]any `yaml:"inputs"`
}

type recoveryStep struct {
	Name string         `yaml:"name"`
	Uses string         `yaml:"uses"`
	With map[string]any `yaml:"with"`
	Env  map[string]any `yaml:"env"`
	Run  string         `yaml:"run"`
}

type recoveryJob struct {
	Environment struct {
		Name string `yaml:"name"`
	} `yaml:"environment"`
	Permissions map[string]string `yaml:"permissions"`
	RunsOn      string            `yaml:"runs-on"`
	Steps       []recoveryStep    `yaml:"steps"`
}

type recoveryWorkflow struct {
	On          map[string]recoveryDispatch `yaml:"on"`
	Permissions map[string]string           `yaml:"permissions"`
	Jobs        map[string]recoveryJob      `yaml:"jobs"`
}

func readRecoveryWorkflow(t *testing.T) (string, recoveryWorkflow) {
	t.Helper()
	raw := readReleaseFile(t, formulaRecoveryWorkflow)
	var workflow recoveryWorkflow
	if err := yaml.Unmarshal([]byte(raw), &workflow); err != nil {
		t.Fatalf("parse recovery workflow: %v", err)
	}
	return raw, workflow
}

func TestFormulaRecoveryWorkflow_ManualExactA6LeastPrivilege(t *testing.T) {
	raw, workflow := readRecoveryWorkflow(t)
	if len(workflow.On) != 1 {
		t.Fatalf("recovery triggers = %v, want workflow_dispatch only", workflow.On)
	}
	dispatch, ok := workflow.On["workflow_dispatch"]
	if !ok || len(dispatch.Inputs) != 0 {
		t.Fatalf("recovery dispatch accepts inputs or is missing: %#v", dispatch)
	}
	if len(workflow.Permissions) != 1 || workflow.Permissions["contents"] != "read" {
		t.Fatalf("global recovery permissions = %v, want contents: read only", workflow.Permissions)
	}
	if len(workflow.Jobs) != 1 {
		t.Fatalf("recovery job count = %d, want 1", len(workflow.Jobs))
	}
	job, ok := workflow.Jobs["recover-formula-bridge"]
	if !ok {
		t.Fatal("recover-formula-bridge job is missing")
	}
	if len(job.Permissions) != 1 || job.Permissions["contents"] != "read" {
		t.Fatalf("job recovery permissions = %v, want contents: read only", job.Permissions)
	}
	if job.Environment.Name != "adk-companion-release" || job.RunsOn != "macos-14" {
		t.Fatalf("recovery boundary environment=%q runner=%q", job.Environment.Name, job.RunsOn)
	}
	for _, forbidden := range []string{
		"id-token:", "pull_request:", "repository_dispatch:", "schedule:",
		"${{ inputs.", "github.event.inputs", "refs/tags/v*", "v0.50.69", "v0.50.70", "v0.50.71", "v0.50.72", "v0.50.73", "v0.50.74",
	} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("recovery workflow contains forbidden expansion %q", forbidden)
		}
	}
	for _, version := range regexp.MustCompile(`v?[0-9]+\.[0-9]+\.[0-9]+`).FindAllString(raw, -1) {
		if version != "v0.50.75" && version != "0.50.75" {
			t.Fatalf("recovery workflow references non-A6 version %q", version)
		}
	}
	if regexp.MustCompile(`(?m)^\s+contents:\s+write\s*$`).MatchString(raw) {
		t.Fatal("recovery workflow grants repository contents: write")
	}
}

func TestFormulaRecoveryWorkflow_PinsCheckoutAndTapAppScope(t *testing.T) {
	raw, workflow := readRecoveryWorkflow(t)
	job := workflow.Jobs["recover-formula-bridge"]
	wantUses := []string{
		"actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10",
		"actions/create-github-app-token@bcd2ba49218906704ab6c1aa796996da409d3eb1",
	}
	var gotUses []string
	immutable := regexp.MustCompile(`^[^@[:space:]]+@[0-9a-f]{40}$`)
	var appStep recoveryStep
	for _, step := range job.Steps {
		if step.Uses == "" {
			continue
		}
		gotUses = append(gotUses, step.Uses)
		if strings.HasPrefix(step.Uses, "actions/create-github-app-token@") {
			appStep = step
		}
		if !immutable.MatchString(step.Uses) {
			t.Fatalf("recovery uses mutable action %q", step.Uses)
		}
	}
	if strings.Join(gotUses, "\n") != strings.Join(wantUses, "\n") {
		t.Fatalf("recovery actions = %v, want %v", gotUses, wantUses)
	}
	wantAppScope := map[string]any{
		"client-id":   "${{ vars.HOMEBREW_APP_CLIENT_ID }}",
		"private-key": "${{ secrets.HOMEBREW_APP_PRIVATE_KEY }}",
		"owner":       "Insajin", "repositories": "homebrew-autopus",
		"permission-contents": "write",
	}
	if len(appStep.With) != len(wantAppScope) {
		t.Fatalf("Homebrew App scope = %v, want tap contents:write only", appStep.With)
	}
	for name, want := range wantAppScope {
		if appStep.With[name] != want {
			t.Fatalf("Homebrew App input %s = %#v, want %#v", name, appStep.With[name], want)
		}
	}
	for _, exact := range []string{
		"ref: refs/tags/v0.50.75", "fetch-depth: 0", "persist-credentials: false",
		"client-id: ${{ vars.HOMEBREW_APP_CLIENT_ID }}",
		"private-key: ${{ secrets.HOMEBREW_APP_PRIVATE_KEY }}",
		"owner: Insajin", "repositories: homebrew-autopus", "permission-contents: write",
	} {
		if !strings.Contains(raw, exact) {
			t.Fatalf("recovery action contract missing %q", exact)
		}
	}
}

func TestFormulaRecoveryWorkflow_ValidatesSourceAndImmutableReleaseEvidence(t *testing.T) {
	raw, _ := readRecoveryWorkflow(t)
	for _, required := range []string{
		"git rev-parse --verify 'HEAD^{commit}'", "mktemp", "GITHUB_REF_NAME='v0.50.75'",
		"GITHUB_REF_TYPE='tag'", `GITHUB_SHA="$actual_head"`,
		`GITHUB_OUTPUT="$validation_output"`, "scripts/companion-release/validate-source.sh",
		"COMPANION_SOURCE_PIN_REQUIRED=1", "ADK_COMPANION_APPROVED_SOURCE_COMMIT",
		"ADK_COMPANION_APPROVED_SOURCE_TREE",
		`repos/Insajin/autopus-adk/releases/tags/v0.50.75`,
		`.tag_name == $tag`, `.target_commitish == $commit`, `.draft == false`,
		`.prerelease == false`, `.immutable == true`, `.name == $name`, `length == 1`,
		`Accept: application/octet-stream`, `.digest`, `^sha256:[0-9a-f]{64}$`,
		"shasum -a 256", `[[ "sha256:${local_digest}" == "$asset_digest" ]]`,
	} {
		if !strings.Contains(raw, required) {
			t.Fatalf("recovery evidence gate missing %q", required)
		}
	}
	ordered := []string{
		"actions/checkout@", "scripts/companion-release/validate-source.sh",
		"releases/tags/v0.50.75", "Accept: application/octet-stream",
		"actions/create-github-app-token@", "scripts/companion-release/publish-homebrew-formula-bridge.sh",
	}
	previous := -1
	for _, marker := range ordered {
		index := strings.Index(raw, marker)
		if index <= previous {
			t.Fatalf("recovery order invalid at %q: prior=%d current=%d", marker, previous, index)
		}
		previous = index
	}
}

func TestFormulaRecoveryWorkflow_RunsOnlyIdempotentA6CaskWithAllowlistedEnvironment(t *testing.T) {
	_, workflow := readRecoveryWorkflow(t)
	var bridge recoveryStep
	for _, step := range workflow.Jobs["recover-formula-bridge"].Steps {
		if step.Name == "Reconcile Homebrew Cask" {
			bridge = step
		}
	}
	wantBridge := `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$RUNNER_TEMP" \
  GITHUB_REF_NAME='v0.50.75' \
  COMPANION_VERSION='0.50.75' \
  COMPANION_HOMEBREW_POLICY='cask-only' \
  COMPANION_CHECKSUMS_PATH="$COMPANION_CHECKSUMS_PATH" \
  HOMEBREW_TAP_TOKEN="$HOMEBREW_TAP_TOKEN" \
  scripts/companion-release/publish-homebrew-formula-bridge.sh`
	if !strings.Contains(bridge.Run, wantBridge) {
		t.Fatalf("recovery bridge environment is not the exact allowlist:\n%s", bridge.Run)
	}
	if len(bridge.Env) != 2 || bridge.Env["COMPANION_CHECKSUMS_PATH"] == nil ||
		bridge.Env["HOMEBREW_TAP_TOKEN"] == nil {
		t.Fatalf("recovery bridge step environment = %v, want checksum path and tap token only", bridge.Env)
	}
	mutation := regexp.MustCompile(`(?i)goreleaser|gh[[:space:]]+release|git[[:space:]]+(tag|push)|--method[=[:space:]]+(post|patch|put|delete)|curl[^\n]+-[Xx][[:space:]]*(post|patch|put|delete)`)
	for _, step := range workflow.Jobs["recover-formula-bridge"].Steps {
		if mutation.MatchString(step.Run) {
			t.Fatalf("recovery step %q can mutate the immutable GitHub release", step.Name)
		}
		for _, forbidden := range []string{"--input", "--field", "--raw-field"} {
			if strings.Contains(step.Run, forbidden) {
				t.Fatalf("recovery step %q contains API mutation input %q", step.Name, forbidden)
			}
		}
	}
	if strings.Contains(bridge.Run, "GITHUB_TOKEN") || strings.Contains(bridge.Run, "GH_TOKEN") {
		t.Fatal("repository token is forwarded to the tap bridge")
	}
}
