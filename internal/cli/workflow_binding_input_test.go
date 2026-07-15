package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkflowBinding_UntrustedEvidence_FailsClosedWithStableReason(t *testing.T) {
	t.Parallel()

	t.Run("empty files", func(t *testing.T) {
		got := executeBindingCommand(t, newWorkflowBindingCmd(nil), "--quality", "ultra", "--risk-tier", "auto",
			"--files-file", writeBindingFiles(t, nil), "--format", "json")
		assertBindingShape(t, got, "unknown", "missing_risk_evidence", 3, true)
	})

	t.Run("malformed files JSON", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "changed-files.json")
		if err := os.WriteFile(path, []byte(`{"files":`), 0o600); err != nil {
			t.Fatal(err)
		}
		got := executeBindingCommand(t, newWorkflowBindingCmd(nil), "--quality", "ultra", "--risk-tier", "auto",
			"--files-file", path, "--format", "json")
		assertBindingShape(t, got, "unknown", "malformed_risk_input", 3, true)
	})

	t.Run("trailing malformed JSON", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "changed-files.json")
		if err := os.WriteFile(path, []byte(`["docs/guide.md"] trailing`), 0o600); err != nil {
			t.Fatal(err)
		}
		got := executeBindingCommand(t, newWorkflowBindingCmd(nil), "--quality", "ultra", "--risk-tier", "auto", "--files-file", path)
		assertBindingShape(t, got, "unknown", "malformed_risk_input", 3, true)
	})

	t.Run("oversized files JSON", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "changed-files.json")
		if err := os.WriteFile(path, []byte(`[`+strings.Repeat(`"a",`, maxBindingJSONBytes)+`"z"]`), 0o600); err != nil {
			t.Fatal(err)
		}
		got := executeBindingCommand(t, newWorkflowBindingCmd(nil), "--quality", "ultra", "--risk-tier", "auto", "--files-file", path)
		assertBindingShape(t, got, "unknown", "malformed_risk_input", 3, true)
	})

	t.Run("too many files", func(t *testing.T) {
		files := make([]string, maxBindingFiles+1)
		for i := range files {
			files[i] = fmt.Sprintf("f%d.go", i)
		}
		got := executeBindingCommand(t, newWorkflowBindingCmd(nil), "--quality", "ultra", "--risk-tier", "auto", "--files-file", writeBindingFiles(t, files))
		assertBindingShape(t, got, "unknown", "malformed_risk_input", 3, true)
	})

	t.Run("path too long", func(t *testing.T) {
		got := executeBindingCommand(t, newWorkflowBindingCmd(nil), "--quality", "ultra", "--risk-tier", "auto",
			"--files-file", writeBindingFiles(t, []string{strings.Repeat("a", maxBindingPathBytes+1)}))
		assertBindingShape(t, got, "unknown", "malformed_risk_input", 3, true)
	})

	t.Run("malformed explicit risk tier", func(t *testing.T) {
		got := executeBindingCommand(t, newWorkflowBindingCmd(nil), "--quality", "ultra", "--risk-tier", "nonsense", "--format", "json")
		assertBindingShape(t, got, "unknown", "malformed_risk_input", 3, true)
	})

	t.Run("binding validation failure", func(t *testing.T) {
		got := executeBindingCommand(t, newWorkflowBindingCmd(nil), "--quality", "invalid", "--risk-tier", "auto",
			"--files-file", writeBindingFiles(t, []string{"docs/guide.md"}), "--format", "json")
		assertBindingShape(t, got, "low", "binding_validation_failed", 3, true)
	})

	t.Run("changed-file discovery error", func(t *testing.T) {
		const privilegedPath = "/Users/private/repository"
		discover := func() ([]string, error) { return nil, errors.New("git unavailable at " + privilegedPath) }
		got, raw := executeBindingCommandRaw(t, newWorkflowBindingCmd(discover), "--quality", "ultra", "--risk-tier", "auto", "--format", "json")
		assertBindingShape(t, got, "unknown", "risk_discovery_failed", 3, true)
		if strings.Contains(raw, privilegedPath) {
			t.Fatalf("binding receipt leaked privileged discovery path: %q", raw)
		}
	})
}
