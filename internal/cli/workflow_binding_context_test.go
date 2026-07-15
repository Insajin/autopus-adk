package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func TestWorkflowBinding_CompactUltraRequiresVerifiedContextManifest(t *testing.T) {
	root := writeWorkflowContextProject(t)
	manifest := writeVerifiedContextManifest(t, root)
	got := executeBindingCommand(t, newWorkflowBindingCmd(nil),
		"--quality", "ultra", "--risk-tier", "auto",
		"--files-file", writeBindingFiles(t, []string{"docs/guide.md"}),
		"--rollout-receipt", writeCanaryReceipt(t, "low"),
		"--context-manifest", manifest, "--context-spec-dir", deliveryCLISpecDir, "--format", "json")
	assertBindingShape(t, got, "low", "eligible_compact", 1, false)
}

func TestWorkflowBinding_MissingContextManifestFallsBackToFullUltra(t *testing.T) {
	got := executeBindingCommand(t, newWorkflowBindingCmd(nil),
		"--quality", "ultra", "--risk-tier", "auto",
		"--files-file", writeBindingFiles(t, []string{"docs/guide.md"}),
		"--rollout-receipt", writeCanaryReceipt(t, "low"), "--format", "json")
	assertBindingShape(t, got, "low", "context_integrity_missing", 3, true)
}

func TestWorkflowBinding_InvalidContextManifestFallsBackWithStableReason(t *testing.T) {
	tests := []struct {
		name  string
		build func(t *testing.T, root string) string
	}{
		{name: "malformed", build: func(t *testing.T, root string) string {
			path := filepath.Join(root, "context-manifest.json")
			require.NoError(t, os.WriteFile(path, []byte("{"), 0o600))
			return path
		}},
		{name: "tampered", build: func(t *testing.T, root string) string {
			result := buildVerifiedContextResult(t, root)
			result.SnapshotHash = "sha256:" + strings.Repeat("0", 64)
			return writeContextManifestResult(t, root, result)
		}},
		{name: "stale", build: func(t *testing.T, root string) string {
			path := writeVerifiedContextManifest(t, root)
			require.NoError(t, os.WriteFile(filepath.Join(root, "AGENTS.md"), []byte("changed after manifest"), 0o600))
			return path
		}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := writeWorkflowContextProject(t)
			got := executeBindingCommand(t, newWorkflowBindingCmd(nil),
				"--quality", "ultra", "--risk-tier", "auto",
				"--files-file", writeBindingFiles(t, []string{"docs/guide.md"}),
				"--rollout-receipt", writeCanaryReceipt(t, "low"),
				"--context-manifest", tt.build(t, root), "--context-spec-dir", deliveryCLISpecDir, "--format", "json")
			assertBindingShape(t, got, "low", "context_integrity_failed", 3, true)
		})
	}
}

func TestWorkflowBinding_ContextManifestMustMatchRequestedSpec(t *testing.T) {
	root := writeWorkflowContextProject(t)
	got := executeBindingCommand(t, newWorkflowBindingCmd(nil),
		"--quality", "ultra", "--risk-tier", "auto",
		"--files-file", writeBindingFiles(t, []string{"docs/guide.md"}),
		"--rollout-receipt", writeCanaryReceipt(t, "low"),
		"--context-manifest", writeVerifiedContextManifest(t, root),
		"--context-spec-dir", ".autopus/specs/SPEC-DIFFERENT-001", "--format", "json")
	assertBindingShape(t, got, "low", "context_integrity_failed", 3, true)
}

func TestWorkflowBinding_ContextManifestMustMatchTaskRequiredReferenceSet(t *testing.T) {
	root := writeWorkflowContextProject(t)
	reference := "docs/task-contract.md"
	path := filepath.Join(root, filepath.FromSlash(reference))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("task contract"), 0o600))

	complete, err := promptlayer.BuildContextDelivery(promptlayer.ContextDeliveryOptions{
		Root: root, Command: "go", SpecDir: deliveryCLISpecDir,
		RequiredReferences: []string{reference},
	})
	require.NoError(t, err)
	got := executeBindingCommand(t, newWorkflowBindingCmd(nil),
		"--quality", "ultra", "--risk-tier", "low",
		"--rollout-receipt", writeCanaryReceipt(t, "low"),
		"--context-manifest", writeContextManifestResult(t, root, complete),
		"--context-spec-dir", deliveryCLISpecDir,
		"--context-required-document", reference, "--format", "json")
	assertBindingShape(t, got, "low", "eligible_compact", 1, false)

	omitted := buildVerifiedContextResult(t, root)
	got = executeBindingCommand(t, newWorkflowBindingCmd(nil),
		"--quality", "ultra", "--risk-tier", "low",
		"--rollout-receipt", writeCanaryReceipt(t, "low"),
		"--context-manifest", writeContextManifestResult(t, root, omitted),
		"--context-spec-dir", deliveryCLISpecDir,
		"--context-required-document", reference, "--format", "json")
	assertBindingShape(t, got, "low", "context_integrity_failed", 3, true)
}

func TestWorkflowBinding_HighAndCriticalRiskRemainFullWithVerifiedContext(t *testing.T) {
	for _, risk := range []string{"high", "critical"} {
		t.Run(risk, func(t *testing.T) {
			root := writeWorkflowContextProject(t)
			got := executeBindingCommand(t, newWorkflowBindingCmd(nil),
				"--quality", "ultra", "--risk-tier", risk,
				"--rollout-receipt", writeCanaryReceipt(t, risk),
				"--context-manifest", writeVerifiedContextManifest(t, root),
				"--context-spec-dir", deliveryCLISpecDir, "--format", "json")
			assertBindingShape(t, got, risk, "risk_requires_full", 3, true)
		})
	}
}

func writeVerifiedContextManifest(t *testing.T, root string) string {
	t.Helper()
	return writeContextManifestResult(t, root, buildVerifiedContextResult(t, root))
}

func buildVerifiedContextResult(t *testing.T, root string) promptlayer.ContextDeliveryResult {
	t.Helper()
	result, err := promptlayer.BuildContextDelivery(promptlayer.ContextDeliveryOptions{
		Root: root, Command: "go", SpecDir: deliveryCLISpecDir,
	})
	require.NoError(t, err)
	return result
}

func writeContextManifestResult(t *testing.T, root string, result promptlayer.ContextDeliveryResult) string {
	t.Helper()
	path := filepath.Join(root, "context-manifest.json")
	data, err := json.Marshal(result)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, data, 0o600))
	return path
}
