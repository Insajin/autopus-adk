package companionmanifest

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"
)

func requireCurrentReceiptContract(t *testing.T, step publicKeyReceiptWorkflowStep) {
	t.Helper()
	configured := map[string]string{
		"COMPANION_KEY_ID":           "${{ vars.ADK_COMPANION_KEY_ID }}",
		"COMPANION_SIGNING_KEY_FILE": "${{ runner.temp }}/adk-release-credentials/companion-ed25519-private-key",
		"COMPANION_HANDOFF":          "${{ vars.ADK_COMPANION_HANDOFF }}",
		"COMPANION_ROLLBACK_FLOOR":   "${{ vars.ADK_COMPANION_ROLLBACK_FLOOR }}",
	}
	for name, want := range configured {
		if got := step.Env[name]; got != want {
			t.Fatalf("step %q %s = %q, want current receipt contract %q", step.Name, name, got, want)
		}
		if !strings.Contains(step.Run, name+`="$`+name+`"`) {
			t.Fatalf("step %q does not pass configured current receipt claim %s", step.Name, name)
		}
	}
}

func runPublicKeyReceiptLineagePhase(t *testing.T, tag string) (string, error) {
	t.Helper()
	path, err := filepath.Abs(filepath.Join("..", "..", "scripts", "companion-release", "verify-public-key-lineage.sh"))
	if err != nil {
		t.Fatal(err)
	}
	command := exec.Command("bash", path)
	command.Env = []string{"GITHUB_REF_NAME=" + tag, "PATH=" + os.Getenv("PATH")}
	output, runErr := command.CombinedOutput()
	return string(output), runErr
}

func requirePublicKeyReceiptLineagePhaseFailure(t *testing.T, tag, code string) {
	t.Helper()
	output, err := runPublicKeyReceiptLineagePhase(t, tag)
	if err == nil {
		t.Fatalf("lineage phase %s passed without immutable A0 pins\n%s", tag, output)
	}
	if !strings.Contains(output, code) {
		t.Fatalf("lineage phase %s failure = %q, want %s", tag, output, code)
	}
}

func a0LineagePinsProvisioned(t *testing.T, source string, api publicKeyReceiptArtifactAPI) bool {
	t.Helper()
	pins := map[string]int{"COMMIT": 40, "RECEIPT": 64, "RECORD": 64, "PUBLIC_KEY": 64}
	if api.bundle {
		pins["SIGNATURE"] = 64
	} else {
		pins["ENVELOPE"] = 64
	}
	allProvisioned := true
	for kind, length := range pins {
		pattern := regexp.MustCompile(`(?i)(?:readonly )?[A-Z0-9_]*A0[A-Z0-9_]*` + kind +
			`[A-Z0-9_]* *= *['"]([^'"]*)['"]`)
		match := pattern.FindStringSubmatch(source)
		if len(match) != 2 {
			t.Fatalf("A0 %s source pin declaration is missing", strings.ToLower(kind))
		}
		if match[1] == "" {
			allProvisioned = false
			continue
		}
		valid := regexp.MustCompile(`^[0-9a-f]{` + strconv.Itoa(length) + `}$`).MatchString(match[1])
		if !valid || strings.Trim(match[1], "0") == "" {
			t.Fatalf("A0 %s must be blank before bootstrap or a non-zero literal %d-hex pin", strings.ToLower(kind), length)
		}
	}
	return allProvisioned
}
