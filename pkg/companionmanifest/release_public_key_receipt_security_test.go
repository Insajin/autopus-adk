package companionmanifest

import (
	"regexp"
	"strings"
	"testing"
)

func TestReleasePublicKeyReceipt_Workflow_SecretsAreStepScopedAndCleanupFailuresObserved(t *testing.T) {
	workflow := releaseWorkflowContract(t)
	if len(workflow.Env) != 0 {
		t.Fatalf("release secrets/credentials must not be workflow-scoped: %#v", workflow.Env)
	}
	allowed := publicKeyReceiptAllowedStepEnv()
	for jobName, job := range workflow.Jobs {
		if len(job.Env) != 0 {
			t.Fatalf("release credentials must not be job-scoped in %s: %#v", jobName, job.Env)
		}
		for _, step := range job.Steps {
			if strings.Contains(step.Run, "${{ secrets.") {
				t.Fatalf("step %q expands a secret directly inside a command instead of step env", step.Name)
			}
			for name := range step.Env {
				if _, ok := allowed[name]; !ok {
					t.Fatalf("step %q receives non-allowlisted environment %q", step.Name, name)
				}
			}
			run := normalizedReleaseText([]byte(step.Run))
			if releaseSensitiveCommand(run) && !strings.Contains(run, "env -i") {
				t.Fatalf("release command step %q does not execute with an empty allowlisted env", step.Name)
			}
		}
	}
	_, sourceStep := releaseWorkflowStepContaining(t, workflow, "Validate exact release source")
	wantSourcePins := map[string]string{
		"COMPANION_APPROVED_SOURCE_COMMIT": "${{ vars.ADK_COMPANION_APPROVED_SOURCE_COMMIT }}",
		"COMPANION_APPROVED_SOURCE_TREE":   "${{ vars.ADK_COMPANION_APPROVED_SOURCE_TREE }}",
	}
	for name, want := range wantSourcePins {
		if got := sourceStep.Env[name]; got != want {
			t.Fatalf("exact release source %s = %q, want repository variable %q", name, got, want)
		}
	}
	_, evidenceStep := releaseWorkflowStepContaining(t, workflow, "verify-current-release.sh")
	if got, want := evidenceStep.Env["COMPANION_SOURCE_COMMIT"],
		"${{ steps.release-source.outputs.source-commit }}"; got != want {
		t.Fatalf("current release evidence source commit = %q, want validated output %q", got, want)
	}
	_, homebrewStep := releaseWorkflowStepContaining(t, workflow, "Publish Homebrew Cask")
	if got, want := homebrewStep.Env["COMPANION_CHECKSUMS_PATH"],
		"${{ steps.release-evidence.outputs.checksums-path }}"; got != want {
		t.Fatalf("Homebrew checksum input = %q, want verified release evidence %q", got, want)
	}
	raw := string(releaseSourceFile(t, ".github/workflows/release.yaml"))
	if strings.Contains(raw, "$GITHUB_ENV") {
		t.Fatal("release workflow promotes credential state through GITHUB_ENV instead of keeping it step-scoped")
	}
	cleanup := releaseCredentialCleanupStep(t, workflow)
	if !strings.Contains(cleanup.If, "always()") {
		t.Fatalf("credential cleanup is not always-run: if=%q", cleanup.If)
	}
	for _, forbidden := range []string{"set +e", "|| true", ">/dev/null 2>&1"} {
		if strings.Contains(cleanup.Run, forbidden) {
			t.Fatalf("credential cleanup hides a failure with %q", forbidden)
		}
	}
	for _, required := range []string{"cleanup_status", `exit "$cleanup_status"`} {
		if !strings.Contains(cleanup.Run, required) {
			t.Fatalf("always-run cleanup does not observe and return failure through %q", required)
		}
	}
}

func releaseSensitiveCommand(run string) bool {
	for _, command := range []string{
		"validate-environment.sh", "public-key", "goreleaser release --clean",
		"verify-current-release.sh", "publish-homebrew-formula-bridge.sh",
	} {
		if strings.Contains(strings.ToLower(run), strings.ToLower(command)) {
			return true
		}
	}
	return false
}

func releaseCredentialCleanupStep(t *testing.T, workflow publicKeyReceiptWorkflow) publicKeyReceiptWorkflowStep {
	t.Helper()
	job, ok := workflow.Jobs["release"]
	if !ok {
		t.Fatal("release workflow has no release job")
	}
	for _, step := range job.Steps {
		name := strings.ToLower(step.Name)
		if strings.Contains(name, "credentials") &&
			(strings.Contains(name, "cleanup") || strings.Contains(name, "remove")) {
			return step
		}
	}
	t.Fatal("release workflow has no explicit credential cleanup step")
	return publicKeyReceiptWorkflowStep{}
}

func TestReleasePublicKeyReceipt_Workflow_ExternalActionsAreImmutableSHAPinned(t *testing.T) {
	workflow := releaseWorkflowContract(t)
	pin := regexp.MustCompile(`^[^@[:space:]]+@[0-9a-f]{40}$`)
	for jobName, job := range workflow.Jobs {
		if job.Uses != "" && !strings.HasPrefix(job.Uses, "./") && !pin.MatchString(job.Uses) {
			t.Fatalf("external reusable workflow %s uses mutable ref %q; exact 40-hex SHA required", jobName, job.Uses)
		}
		for _, step := range job.Steps {
			if step.Uses == "" || strings.HasPrefix(step.Uses, "./") {
				continue
			}
			if !pin.MatchString(step.Uses) {
				t.Fatalf("external Action in step %q uses mutable ref %q; exact 40-hex SHA required", step.Name, step.Uses)
			}
		}
	}
}

// Parser fixtures and RFC vectors are test inputs only; they can never become A0 release evidence.
func TestReleasePublicKeyReceipt_FixturesAreParserOnlyAndCannotCreateFakeA0Pass(t *testing.T) {
	workflow := string(releaseSourceFile(t, ".github/workflows/release.yaml"))
	goReleaser := string(releaseSourceFile(t, ".goreleaser.yaml"))
	scripts := string(releaseScriptsText(t))
	production := workflow + "\n" + goReleaser + "\n" + scripts
	for _, forbidden := range []string{
		"testdata/", "fixture/", "fixtures/", "_test.go", "GO_WANT_", "file://", "localhost",
	} {
		if strings.Contains(strings.ToLower(production), strings.ToLower(forbidden)) {
			t.Fatalf("parser/test input %q is wired into production release evidence", forbidden)
		}
	}
	localEvidence := regexp.MustCompile(`(?i)(A0|PRIOR|PREVIOUS)[A-Z0-9_]*(PATH|FILE)|--(a0|prior|previous)[-a-z]*path`)
	if match := localEvidence.FindString(production); match != "" {
		t.Fatalf("caller-supplied local evidence path %q can fake immutable A0 provenance", match)
	}
	callerBypass := regexp.MustCompile(`(?i)(SKIP|ALLOW|TRUST|IS)[A-Z0-9_]*(A0|PRIOR|LINEAGE)|(A0|PRIOR)[A-Z0-9_]*(PASS|OK|VALID)=`)
	if match := callerBypass.FindString(production); match != "" {
		t.Fatalf("caller boolean %q can fake A0/prior-evidence PASS", match)
	}
	normalizedScripts := normalizedReleaseText([]byte(scripts))
	for _, required := range []string{
		"fixture_or_local_evidence_forbidden",
		"immutable A0 GitHub release",
	} {
		if !strings.Contains(normalizedScripts, required) {
			t.Fatalf("missing production guard %q: fixtures are parser inputs only and never release evidence", required)
		}
	}
}

func publicKeyReceiptAllowedStepEnv() map[string]struct{} {
	names := []string{
		"APPLE_API_ISSUER_SECRET", "APPLE_API_KEY_SECRET", "APPLE_API_KEY_P8",
		"APPLE_CERTIFICATE", "APPLE_CERTIFICATE_PASSWORD",
		"ADK_COMPANION_ED25519_PRIVATE_KEY", "ADK_RELEASE_ECDSA_PRIVATE_KEY",
		"GITHUB_TOKEN", "HOMEBREW_TAP_TOKEN",
		"APPLE_API_ISSUER", "APPLE_API_KEY", "APPLE_API_KEY_PATH",
		"APPLE_SIGNING_IDENTITY", "APPLE_SIGNING_KEYCHAIN",
		"COMPANION_PLATFORM", "COMPANION_BUILD_PROVENANCE", "COMPANION_HANDOFF",
		"COMPANION_ROLLBACK_FLOOR", "COMPANION_ISSUED_AT", "COMPANION_EXPIRES_AT",
		"COMPANION_KEY_ID", "COMPANION_RELEASE_PRODUCTION", "COMPANION_SIGNING_KEY_FILE",
		"ADK_RELEASE_ECDSA_PRIVATE_KEY_FILE",
		"COMPANION_APPROVED_SOURCE_COMMIT", "COMPANION_APPROVED_SOURCE_TREE",
		"COMPANION_SOURCE_COMMIT", "COMPANION_CHECKSUMS_PATH",
		"COMPANION_SIGNER", "COMPANION_MANIFEST_VERIFIER",
		"COMPANION_RELEASE_TIME_VALIDATION_REQUIRED",
		"COMPANION_PUBLIC_KEY_RECEIPT_ISSUED_AT",
		"COMPANION_PUBLIC_KEY_RECEIPT_EXPIRES_AT",
		"COMPANION_PUBLIC_KEY_RECEIPT_MINIMUM_LIFETIME_SECONDS",
	}
	allowed := make(map[string]struct{}, len(names))
	for _, name := range names {
		allowed[name] = struct{}{}
	}
	return allowed
}
