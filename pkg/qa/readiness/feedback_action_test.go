package readiness_test

import (
	"reflect"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/readiness"
)

func TestFeedbackAction_EnabledOnlyForFailedDeterministicRedactedSupportedTargets(t *testing.T) {
	t.Parallel()

	base := readiness.EvidenceForFeedback{
		Status:                 readiness.StatusFailed,
		DeterministicAuthority: true,
		RedactionStatus:        readiness.RedactionPassed,
		ManifestPath:           "qa/evidence/manifests/login.json",
	}

	tests := []struct {
		name       string
		evidence   readiness.EvidenceForFeedback
		target     string
		wantEnable bool
		wantReason string
	}{
		{name: "failed deterministic redacted codex", evidence: base, target: "codex", wantEnable: true},
		{name: "failed deterministic redacted claude", evidence: base, target: "claude", wantEnable: true},
		{name: "passed disabled", evidence: withStatus(base, readiness.StatusPassed), target: "codex", wantReason: "not_failed"},
		{name: "skipped disabled", evidence: withStatus(base, readiness.StatusSkipped), target: "codex", wantReason: "not_failed"},
		{name: "deferred disabled", evidence: withStatus(base, readiness.StatusDeferred), target: "codex", wantReason: "not_failed"},
		{name: "unsupported target disabled", evidence: base, target: "copilot", wantReason: "unsupported_target"},
		{name: "non deterministic disabled", evidence: withDeterministic(base, false), target: "codex", wantReason: "not_deterministic"},
		{name: "redaction failed disabled", evidence: withRedaction(base, readiness.RedactionFailed), target: "codex", wantReason: "redaction_failed"},
		{name: "shell metacharacter manifest disabled", evidence: withManifestPath(base, "qa/evidence/manifests/login.json && deploy"), target: "codex", wantReason: "unsafe_manifest_path"},
		{name: "absolute local manifest disabled", evidence: withManifestPath(base, "/Users/alice/private/manifest.json"), target: "codex", wantReason: "unsafe_manifest_path"},
		{name: "token url manifest disabled", evidence: withManifestPath(base, "https://example.test/manifest.json?token=secret"), target: "codex", wantReason: "unsafe_manifest_path"},
		{name: "traversal manifest disabled", evidence: withManifestPath(base, "../private/manifest.json"), target: "codex", wantReason: "unsafe_manifest_path"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			action := readiness.DeriveFeedbackAction(tt.evidence, tt.target)
			if action.Enabled != tt.wantEnable {
				t.Fatalf("Enabled = %v, want %v for %#v", action.Enabled, tt.wantEnable, tt)
			}
			if tt.wantEnable {
				wantCommand := []string{"auto", "qa", "feedback", "--to", tt.target, "--evidence", "qa/evidence/manifests/login.json"}
				if !reflect.DeepEqual(action.Command, wantCommand) {
					t.Fatalf("Command = %#v, want %#v", action.Command, wantCommand)
				}
				if action.CommandDisplay != "auto qa feedback --to "+tt.target+" --evidence qa/evidence/manifests/login.json" {
					t.Fatalf("CommandDisplay = %q", action.CommandDisplay)
				}
				if action.DisabledReason != "" {
					t.Fatalf("DisabledReason = %q, want empty for enabled action", action.DisabledReason)
				}
				return
			}
			if action.Command != nil || action.CommandDisplay != "" {
				t.Fatalf("disabled action exposed command material: %#v", action)
			}
			if action.DisabledReason != tt.wantReason {
				t.Fatalf("DisabledReason = %q, want %q", action.DisabledReason, tt.wantReason)
			}
		})
	}
}

func withStatus(e readiness.EvidenceForFeedback, status readiness.Status) readiness.EvidenceForFeedback {
	e.Status = status
	return e
}

func withDeterministic(e readiness.EvidenceForFeedback, deterministic bool) readiness.EvidenceForFeedback {
	e.DeterministicAuthority = deterministic
	return e
}

func withRedaction(e readiness.EvidenceForFeedback, status readiness.RedactionStatus) readiness.EvidenceForFeedback {
	e.RedactionStatus = status
	return e
}

func withManifestPath(e readiness.EvidenceForFeedback, path string) readiness.EvidenceForFeedback {
	e.ManifestPath = path
	return e
}
