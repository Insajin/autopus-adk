package companionmanifest

import (
	"regexp"
	"strings"
	"testing"
)

func TestFormulaRecoveryWorkflow_RunsOnlyIdempotentA23CaskWithAllowlistedEnvironment(t *testing.T) {
	_, workflow := readRecoveryWorkflow(t)
	var bridge recoveryStep
	for _, step := range workflow.Jobs["recover-formula-bridge"].Steps {
		if step.Name == "Reconcile Homebrew Cask" {
			bridge = step
		}
	}
	wantBridge := `env -i PATH="$PATH" HOME="$HOME" TMPDIR="$RUNNER_TEMP" \
  GITHUB_REF_NAME="$GITHUB_REF_NAME" \
  COMPANION_VERSION="${GITHUB_REF_NAME#v}" \
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
	mutation := regexp.MustCompile(`(?i)goreleaser|gh[[:space:]]+(release|variable|secret)|git[[:space:]]+(tag|push)|--method[=[:space:]]+(post|patch|put|delete)|curl[^\n]+-[Xx][[:space:]]*(post|patch|put|delete)`)
	for _, step := range workflow.Jobs["recover-formula-bridge"].Steps {
		if mutation.MatchString(step.Run) {
			t.Fatalf("recovery step %q can mutate release, evidence, tag, or variables", step.Name)
		}
		for _, forbidden := range []string{"--input", "--field", "--raw-field", "vars set", "secret set"} {
			if strings.Contains(step.Run, forbidden) {
				t.Fatalf("recovery step %q contains mutation input %q", step.Name, forbidden)
			}
		}
	}
	if strings.Contains(bridge.Run, "GITHUB_TOKEN") || strings.Contains(bridge.Run, "GH_TOKEN") {
		t.Fatal("repository token is forwarded to the tap bridge")
	}
}
