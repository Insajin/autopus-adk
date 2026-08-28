package companionmanifest

import (
	"reflect"
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
	Env         map[string]any    `yaml:"env"`
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

func recoveryStepNamed(t *testing.T, workflow recoveryWorkflow, name string) recoveryStep {
	t.Helper()
	for _, step := range workflow.Jobs["recover-formula-bridge"].Steps {
		if step.Name == name {
			return step
		}
	}
	t.Fatalf("recovery step %q is missing", name)
	return recoveryStep{}
}

func TestFormulaRecoveryWorkflow_ManualExactA22LeastPrivilege(t *testing.T) {
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
	if len(job.Env) != 0 {
		t.Fatalf("recovery credentials escaped step scope: %#v", job.Env)
	}
	for _, forbidden := range []string{
		"id-token:", "pull_request:", "repository_dispatch:", "schedule:",
		"ADK_RELEASE_ECDSA_PRIVATE_KEY", "COMPANION_SIGNING_KEY", "COMPANION_SIGNER", "ACTIONS_ID_TOKEN_REQUEST_TOKEN",
		"${{ inputs.", "github.event.inputs", "refs/tags/v*", "v0.50.69", "v0.50.70", "v0.50.71", "v0.50.72", "v0.50.73", "v0.50.74", "v0.50.75", "v0.50.76", "v0.50.77", "v0.50.78", "v0.50.79", "v0.50.80", "v0.50.81", "v0.50.82", "v0.50.83", "v0.50.84", "v0.50.85", "v0.50.86", "v0.50.87", "v0.50.88", "v0.50.89", "v0.50.90", "v0.50.91", "v0.50.92",
	} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("recovery workflow contains forbidden expansion %q", forbidden)
		}
	}
	for _, version := range regexp.MustCompile(`v?[0-9]+\.[0-9]+\.[0-9]+`).FindAllString(raw, -1) {
		if version != "v0.50.109" && version != "v4.1.2" && version != "v3.1.2" {
			t.Fatalf("recovery workflow references non-A22 version %q", version)
		}
	}
	// 무장 좌표는 job 가드 한 줄에만 산다 — 나머지는 전부 GITHUB_REF_NAME에서 파생한다.
	if got := strings.Count(raw, "v0.50.109"); got != 1 {
		t.Fatalf("recovery workflow names the release coordinate %d times, want exactly 1", got)
	}
	if regexp.MustCompile(`(?m)^\s+contents:\s+write\s*$`).MatchString(raw) {
		t.Fatal("recovery workflow grants repository contents: write")
	}
}

func TestFormulaRecoveryWorkflow_SelectsHomebrewOpenSSL3BeforeRotationAuthority(t *testing.T) {
	_, workflow := readRecoveryWorkflow(t)
	job := workflow.Jobs["recover-formula-bridge"]
	selectionIndex, authorityIndex, selectionCount := -1, -1, 0
	for index, step := range job.Steps {
		switch step.Name {
		case "Select Homebrew OpenSSL 3":
			selectionIndex, selectionCount = index, selectionCount+1
			assertHomebrewOpenSSL3Selection(t, step.Run, step.Env)
		case "Materialize immutable rotation authority and build bridge verifiers":
			authorityIndex = index
		}
	}
	if selectionCount != 1 || selectionIndex < 0 || selectionIndex >= authorityIndex {
		t.Fatalf("recovery OpenSSL/authority order = %d/%d", selectionIndex, authorityIndex)
	}
}

func TestFormulaRecoveryWorkflow_PinsCheckoutAndTapAppScope(t *testing.T) {
	raw, workflow := readRecoveryWorkflow(t)
	job := workflow.Jobs["recover-formula-bridge"]
	wantUses := []string{
		"actions/checkout@df4cb1c069e1874edd31b4311f1884172cec0e10",
		"sigstore/cosign-installer@6f9f17788090df1f26f669e9d70d6ae9567deba6",
		"actions/setup-go@924ae3a1cded613372ab5595356fb5720e22ba16",
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
		"ref: ${{ github.ref }}", "fetch-depth: 0", "persist-credentials: false",
		"client-id: ${{ vars.HOMEBREW_APP_CLIENT_ID }}",
		"private-key: ${{ secrets.HOMEBREW_APP_PRIVATE_KEY }}",
		"owner: Insajin", "repositories: homebrew-autopus", "permission-contents: write",
		"cosign-release: 'v3.1.2'",
	} {
		if !strings.Contains(raw, exact) {
			t.Fatalf("recovery action contract missing %q", exact)
		}
	}
}

func TestFormulaRecoveryWorkflow_ValidatesRotationSourceCandidateAndImmutableRelease(t *testing.T) {
	raw, workflow := readRecoveryWorkflow(t)
	for _, required := range []string{
		"git rev-parse --verify 'HEAD^{commit}'",
		"COMPANION_RELEASE_TAG_SIGNATURE_REQUIRED=1",
		"ADK_KEY_ROTATION_VERIFIED=1",
		"scripts/companion-release/validate-source.sh",
		"scripts/companion-release/build-omp-context-candidate.sh",
		"OMP_CONTEXT_CANDIDATE_ARTIFACT_SHA256: ${{ steps.candidate.outputs.candidate-artifact-sha256 }}",
		"ADK_KEY_ROTATION_VERIFIER: ${{ runner.temp }}/key-rotation-authority/verify-rotation.sh",
		`scripts/companion-release/verify-current-release.sh "$checksums_path"`,
	} {
		if !strings.Contains(raw, required) {
			t.Fatalf("recovery bridge gate missing %q", required)
		}
	}
	build := recoveryStepNamed(t, workflow,
		"Materialize immutable rotation authority and build bridge verifiers")
	for _, required := range []string{
		`authority="$RUNNER_TEMP/key-rotation-authority"`,
		"materialize-key-rotation-authority.sh",
		`"${REPOSITORY_AUTHORITY_COMMIT:-}" =~ ^[0-9a-f]{40}$`,
		`"${PROTECTED_AUTHORITY_COMMIT:-}" =~ ^[0-9a-f]{40}$`,
		`"$PROTECTED_AUTHORITY_COMMIT" == "$REPOSITORY_AUTHORITY_COMMIT"`,
		`--public "$PROTECTED_AUTHORITY_COMMIT" "$authority"`,
		`.assertion_mode == "public"`,
		`printf 'authority-commit=%s\n' "$authority_commit" >> "$GITHUB_OUTPUT"`,
		`lineage_verifier="$RUNNER_TEMP/auto-omp-context-lineage-verifier"`,
		`manifest_verifier="$RUNNER_TEMP/auto-companion-manifest-verifier"`,
		"./scripts/companion-release/ompcontextlineageverify",
		"./scripts/companion-release/manifestverify",
	} {
		if !strings.Contains(build.Run, required) {
			t.Fatalf("recovery verifier build contract missing %q", required)
		}
	}
	wantAuthorityEnv := map[string]any{
		"REPOSITORY_AUTHORITY_COMMIT": "${{ vars.ADK_KEY_ROTATION_AUTHORITY_COMMIT }}",
		"PROTECTED_AUTHORITY_COMMIT":  "${{ vars.ADK_PROTECTED_KEY_ROTATION_AUTHORITY_COMMIT }}",
	}
	if !reflect.DeepEqual(build.Env, wantAuthorityEnv) {
		t.Fatalf("recovery authority environment = %#v, want %#v", build.Env, wantAuthorityEnv)
	}
	if strings.Count(build.Run, `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$RUNNER_TEMP"`) != 2 {
		t.Fatalf("recovery verifier builds escaped env -i isolation:\n%s", build.Run)
	}
	evidence := recoveryStepNamed(t, workflow,
		"Verify current immutable canonical-full bridge release")
	wantEnv := map[string]any{
		"GITHUB_TOKEN":                          "${{ secrets.GITHUB_TOKEN }}",
		"COMPANION_SOURCE_COMMIT":               "${{ steps.release-source.outputs.source-commit }}",
		"COMPANION_SOURCE_TREE":                 "${{ steps.release-source.outputs.source-tree }}",
		"OMP_CONTEXT_CANDIDATE_ARTIFACT_SHA256": "${{ steps.candidate.outputs.candidate-artifact-sha256 }}",
		"ADK_KEY_ROTATION_VERIFIER":             "${{ runner.temp }}/key-rotation-authority/verify-rotation.sh",
	}
	if !reflect.DeepEqual(evidence.Env, wantEnv) {
		t.Fatalf("recovery evidence environment = %#v, want bridge bindings %#v",
			evidence.Env, wantEnv)
	}
	for _, mutable := range []string{
		"${{ vars.AUTOPUS_ADK_CHANNEL_KEY_ID }}",
		"${{ vars.AUTOPUS_ADK_CHANNEL_PUBLIC_KEY }}",
		"${{ vars.ADK_COMPANION_APPROVED_SOURCE_COMMIT }}",
		"${{ vars.ADK_COMPANION_APPROVED_SOURCE_TREE }}",
		"${{ vars.ADK_COMPANION_KEY_ID }}",
		"${{ vars.ADK_COMPANION_HANDOFF }}",
		"${{ vars.ADK_COMPANION_ROLLBACK_FLOOR }}",
		"${{ vars.ADK_COMPANION_PUBLIC_KEY_SHA256 }}",
	} {
		if strings.Contains(raw, mutable) {
			t.Fatalf("historical recovery depends on mutable live authority %q", mutable)
		}
	}
	for _, forbidden := range []string{
		"OMP_CONTEXT_EVIDENCE_", "OMP_CONTEXT_STATIC_POLICY_B64",
		"omp-context-promotion-report.v1.json", "omp-context-promotion-attestation.v2.json",
		"--mode active", "--mode historical",
	} {
		if strings.Contains(raw, forbidden) {
			t.Fatalf("recovery workflow contains forbidden promotion wiring %q", forbidden)
		}
	}
	helper := readReleaseFile(t, "scripts/companion-release/verify-current-release.sh")
	for _, required := range []string{
		`repos/${RELEASE_REPOSITORY}/releases/tags/${RELEASE_TAG}`,
		`.tag_name == $tag`, `.target_commitish == $commit`, `.draft == false`,
		`.prerelease == false`, `.immutable == true`,
		`[.assets[].name] | unique`, `.state == "uploaded"`, `.size > 0`,
		`Accept: application/octet-stream`, `.digest`, `^sha256:[0-9a-f]{64}$`,
		"canonical-full-bridge", "omp-context-bridge-release.v1.json",
		"adk-key-rotation-v1.json", "adk-key-rotation-v1.sig",
		"verify-rotation-historical",
		"BRIDGE_COMPANION_KEY_ID", "adk-channel-2026-q3-a0",
	} {
		if !strings.Contains(helper, required) {
			t.Fatalf("shared bridge release gate missing %q", required)
		}
	}
	ordered := []string{
		"actions/checkout@", "sigstore/cosign-installer@",
		"scripts/companion-release/validate-source.sh",
		"scripts/companion-release/build-omp-context-candidate.sh",
		"scripts/companion-release/verify-current-release.sh",
		"actions/create-github-app-token@",
		"scripts/companion-release/publish-homebrew-formula-bridge.sh",
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
