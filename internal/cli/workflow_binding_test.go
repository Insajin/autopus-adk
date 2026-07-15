package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/experiment"
	"github.com/insajin/autopus-adk/pkg/workflow"
	"github.com/spf13/cobra"
)

type workflowBindingTestReceipt struct {
	Quality                json.RawMessage `json:"quality"`
	Risk                   string          `json:"risk"`
	SelectionReason        string          `json:"selection_reason"`
	ReviewVotes            int             `json:"review_votes"`
	SecurityReviewRequired bool            `json:"security_review_required"`
	Synthesis              bool            `json:"synthesis"`
	FanOutCap              int             `json:"fan_out_cap"`
	EffortDownshifted      bool            `json:"effort_downshifted"`
}

func TestWorkflowBinding_RiskPolicy_SelectsCompactOrFullUltra(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		files      []string
		risk       string
		extraArgs  []string
		wantRisk   string
		wantReason string
		wantVotes  int
		wantSynth  bool
	}{
		{name: "documentation is compact", files: []string{"docs/guide.md"}, wantRisk: "low", wantReason: "eligible_compact", wantVotes: 1},
		{name: "ordinary source is compact", files: []string{"pkg/example/value.go"}, wantRisk: "medium", wantReason: "eligible_compact", wantVotes: 1},
		{name: "worker path is full", files: []string{"pkg/worker/loop.go"}, wantRisk: "high", wantReason: "risk_requires_full", wantVotes: 3, wantSynth: true},
		{name: "public API is full", files: []string{"api/v1/users.go"}, wantRisk: "high", wantReason: "risk_requires_full", wantVotes: 3, wantSynth: true},
		{name: "authentication is full", files: []string{"internal/auth/token.go"}, wantRisk: "critical", wantReason: "risk_requires_full", wantVotes: 3, wantSynth: true},
		{name: "explicit unknown is full", risk: "unknown", wantRisk: "unknown", wantReason: "unknown_risk", wantVotes: 3, wantSynth: true},
		{name: "audit overrides eligible risk", files: []string{"docs/guide.md"}, extraArgs: []string{"--full-depth-audit"}, wantRisk: "low", wantReason: "audit_sample", wantVotes: 3, wantSynth: true},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			args := []string{"--quality", "ultra", "--format", "json"}
			if tt.risk != "" {
				args = append(args, "--risk-tier", tt.risk)
			} else {
				args = append(args, "--risk-tier", "auto", "--files-file", writeBindingFiles(t, tt.files))
			}
			if tt.wantVotes == 1 {
				root := writeWorkflowContextProject(t)
				args = append(args, "--rollout-receipt", writeCanaryReceipt(t, tt.wantRisk),
					"--context-manifest", writeVerifiedContextManifest(t, root),
					"--context-spec-dir", deliveryCLISpecDir)
			}
			args = append(args, tt.extraArgs...)

			got := executeBindingCommand(t, newWorkflowBindingCmd(nil), args...)
			assertBindingShape(t, got, tt.wantRisk, tt.wantReason, tt.wantVotes, tt.wantSynth)
		})
	}
}

func TestWorkflowBinding_LowRiskDefaultsToFullUltraShadow(t *testing.T) {
	got := executeBindingCommand(t, newWorkflowBindingCmd(nil), "--quality", "ultra", "--risk-tier", "auto",
		"--files-file", writeBindingFiles(t, []string{"docs/guide.md"}), "--format", "json")
	assertBindingShape(t, got, "low", "shadow_full", 3, true)
}

func TestWorkflowBinding_OnlyVerifiedCanaryMayCompact(t *testing.T) {
	for _, tc := range []struct {
		name    string
		receipt experiment.RolloutReceipt
	}{
		{name: "shadow", receipt: rolloutReceiptFixture("shadow", "SHADOW", "full_ultra", true, "low")},
		{name: "blocked", receipt: rolloutReceiptFixture("canary", "BLOCKED", "full_ultra", true, "low")},
		{name: "rollback", receipt: rolloutReceiptFixture("rollback", "ROLLBACK", "full_ultra", true, "low")},
		{name: "risk mismatch", receipt: rolloutReceiptFixture("canary", "CANARY", "compact_ultra", false, "medium")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := executeBindingCommand(t, newWorkflowBindingCmd(nil), "--quality", "ultra", "--risk-tier", "auto",
				"--files-file", writeBindingFiles(t, []string{"docs/guide.md"}), "--rollout-receipt", writeRolloutReceipt(t, tc.receipt))
			assertBindingShape(t, got, "low", "shadow_full", 3, true)
		})
	}
}

func TestWorkflowBinding_ChangesOnlyUltraReviewDepth(t *testing.T) {
	t.Parallel()

	root := writeWorkflowContextProject(t)
	compact := executeBindingCommand(t, newWorkflowBindingCmd(nil),
		"--quality", "ultra", "--risk-tier", "auto",
		"--files-file", writeBindingFiles(t, []string{"docs/guide.md"}),
		"--rollout-receipt", writeCanaryReceipt(t, "low"),
		"--context-manifest", writeVerifiedContextManifest(t, root),
		"--context-spec-dir", deliveryCLISpecDir, "--format", "json")
	full := resolveTeamQualityBinding("ultra", "")

	compactPhases := decodeBindingPhases(t, compact.Quality)
	for phase, fullBinding := range full.Phases {
		compactBinding, ok := compactPhases[phase]
		if !ok {
			t.Fatalf("compact binding is missing phase %q", phase)
		}
		if phase != "review" {
			if !reflect.DeepEqual(compactBinding, fullBinding) {
				t.Fatalf("phase %q changed: compact=%+v full=%+v", phase, compactBinding, fullBinding)
			}
			continue
		}
		if compactBinding.Model != fullBinding.Model || compactBinding.Effort != fullBinding.Effort || compactBinding.FanOutCap != fullBinding.FanOutCap {
			t.Fatalf("review model/effort/fan-out changed: compact=%+v full=%+v", compactBinding, fullBinding)
		}
		if compactBinding.VerifyVotes != 1 || compactBinding.Synthesis {
			t.Fatalf("compact review = %+v, want one vote and no synthesis", compactBinding)
		}
	}
}

func TestWorkflowBinding_BalancedQualityBytesRemainCanonical(t *testing.T) {
	t.Parallel()

	want, err := serializeTeamQualityBinding(resolveTeamQualityBinding("balanced", ""))
	if err != nil {
		t.Fatal(err)
	}
	for _, files := range [][]string{{"docs/guide.md"}, {"pkg/worker/loop.go"}, {"internal/auth/token.go"}} {
		got := executeBindingCommand(t, newWorkflowBindingCmd(nil),
			"--quality", "balanced", "--risk-tier", "auto",
			"--files-file", writeBindingFiles(t, files), "--format", "json")
		if string(got.Quality) != want {
			t.Fatalf("balanced binding changed for %v:\n got %s\nwant %s", files, got.Quality, want)
		}
	}
}

func TestWorkflowBinding_IsRegisteredUnderWorkflowCommand(t *testing.T) {
	t.Parallel()

	path := writeBindingFiles(t, []string{"docs/guide.md"})
	contextRoot := writeWorkflowContextProject(t)
	cmd := NewWorkflowCmd(nil, nil)
	got := executeBindingCommand(t, cmd,
		"binding", "--quality", "ultra", "--risk-tier", "auto",
		"--files-file", path, "--rollout-receipt", writeCanaryReceipt(t, "low"),
		"--context-manifest", writeVerifiedContextManifest(t, contextRoot),
		"--context-spec-dir", deliveryCLISpecDir, "--format", "json")
	assertBindingShape(t, got, "low", "eligible_compact", 1, false)
}

func executeBindingCommand(t *testing.T, cmd *cobra.Command, args ...string) workflowBindingTestReceipt {
	t.Helper()
	receipt, _ := executeBindingCommandRaw(t, cmd, args...)
	return receipt
}

func executeBindingCommandRaw(t *testing.T, cmd *cobra.Command, args ...string) (workflowBindingTestReceipt, string) {
	t.Helper()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("workflow binding %v failed: %v\n%s", args, err, out.String())
	}
	var receipt workflowBindingTestReceipt
	if err := json.Unmarshal(out.Bytes(), &receipt); err != nil {
		t.Fatalf("decode workflow binding output %q: %v", out.String(), err)
	}
	return receipt, out.String()
}

func assertBindingShape(t *testing.T, got workflowBindingTestReceipt, risk, reason string, votes int, synthesis bool) {
	t.Helper()
	if got.Risk != risk || got.SelectionReason != reason {
		t.Fatalf("risk receipt = risk %q reason %q, want %q/%q", got.Risk, got.SelectionReason, risk, reason)
	}
	if got.ReviewVotes != votes || got.Synthesis != synthesis {
		t.Fatalf("review shape = votes %d synthesis %v, want %d/%v", got.ReviewVotes, got.Synthesis, votes, synthesis)
	}
	if !got.SecurityReviewRequired {
		t.Fatal("security review must remain required")
	}
	if got.FanOutCap != workflow.MaxFanOut {
		t.Fatalf("fan_out_cap = %d, want %d", got.FanOutCap, workflow.MaxFanOut)
	}
	if got.EffortDownshifted {
		t.Fatal("risk allocation must not downshift effort")
	}
	phases := decodeBindingPhases(t, got.Quality)
	review := phases["review"]
	if review.VerifyVotes != votes || review.Synthesis != synthesis {
		t.Fatalf("bare review binding = %+v, want votes=%d synthesis=%v", review, votes, synthesis)
	}
	implementation := phases["implementation"]
	if implementation.FanOutCap != workflow.MaxFanOut {
		t.Fatalf("implementation fan_out_cap = %d, want %d", implementation.FanOutCap, workflow.MaxFanOut)
	}
	canonical := resolveTeamQualityBinding("ultra", "")
	wantImplementation := canonical.Phases["implementation"]
	if implementation.Model != wantImplementation.Model || implementation.Effort != wantImplementation.Effort {
		t.Fatalf("implementation model/effort changed: got %+v want %+v", implementation, wantImplementation)
	}
	wantReview := canonical.Phases["review"]
	if review.Model != wantReview.Model || review.Effort != wantReview.Effort {
		t.Fatalf("review model/effort changed: got %+v want %+v", review, wantReview)
	}
}

func decodeBindingPhases(t *testing.T, raw json.RawMessage) map[string]workflow.PhaseBinding {
	t.Helper()
	var phases map[string]workflow.PhaseBinding
	if err := json.Unmarshal(raw, &phases); err != nil {
		t.Fatalf("quality must be a bare phase map, got %q: %v", raw, err)
	}
	return phases
}

func writeBindingFiles(t *testing.T, files []string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "changed-files.json")
	data, err := json.Marshal(files)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func writeCanaryReceipt(t *testing.T, risk string) string {
	t.Helper()
	return writeRolloutReceipt(t, rolloutReceiptFixture("canary", "CANARY", "compact_ultra", false, risk))
}

func rolloutReceiptFixture(kind, decision, profile string, fullDepth bool, risk string) experiment.RolloutReceipt {
	hash := "sha256:" + strings.Repeat("a", 64)
	return experiment.RolloutReceipt{
		Version: 1, ExperimentID: "exp-1", TaskCorpusHash: hash, PolicyHash: hash, ConfigHash: hash,
		ReceiptKind: kind, Decision: decision, ActiveProfile: profile, FullDepth: fullDepth, RiskTier: risk,
	}
}

func writeRolloutReceipt(t *testing.T, receipt experiment.RolloutReceipt) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "rollout.json")
	data, _ := json.Marshal(receipt)
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
