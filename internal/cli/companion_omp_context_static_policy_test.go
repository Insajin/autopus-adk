package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestCompanionOMPContextStaticPolicyCommand_IsRegistered(t *testing.T) {
	root := NewRootCmd()
	command, _, err := root.Find([]string{"companion-manifest", "omp-context-static-policy"})
	if err != nil || command.Name() != "omp-context-static-policy" {
		t.Fatalf("registered command=%v error=%v", command, err)
	}
	for _, required := range []string{
		"report", "target", "release-lineage-key-id", "release-lineage-handoff", "minimum-rollback-floor",
	} {
		if command.Flags().Lookup(required) == nil {
			t.Fatalf("required flag %q is not registered", required)
		}
	}
}

func TestCompanionOMPContextStaticPolicy_RejectsInvalidReportWithoutOutput(t *testing.T) {
	for name, body := range map[string]string{
		"incomplete": `{}`,
		"duplicate":  `{"schema_version":"first","schema_version":"second"}`,
	} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "report.json")
			if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
				t.Fatal(err)
			}
			command := newCompanionOMPContextStaticPolicyCmd()
			var output bytes.Buffer
			command.SetOut(&output)
			command.SetArgs([]string{
				"--report", path,
				"--target", "darwin-arm64",
				"--release-lineage-key-id", "release-lineage-2026-q3-k1",
				"--release-lineage-handoff", "v1",
				"--minimum-rollback-floor", "5096",
			})
			if err := command.Execute(); err == nil {
				t.Fatal("invalid report produced a static policy")
			}
			if output.Len() != 0 {
				t.Fatalf("invalid report wrote output %q", output.String())
			}
		})
	}
}
