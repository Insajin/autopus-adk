package companionmanifest

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
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

// @AX:ANCHOR [AUTO]: Keep the reviewed release-script aggregation shared by all lineage policy tests.
// @AX:REASON [AUTO]: Phase tests must inspect the same sorted production surface after helpers are split.
func releaseScriptsText(t *testing.T) []byte {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join("..", "..", "scripts", "companion-release", "*.sh"))
	if err != nil || len(paths) == 0 {
		t.Fatalf("find companion release scripts: %v", err)
	}
	sort.Strings(paths)
	var combined strings.Builder
	for _, path := range paths {
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			t.Fatalf("read release script %s: %v", path, readErr)
		}
		combined.WriteString("\n# source: " + filepath.Base(path) + "\n")
		combined.Write(data)
	}
	return []byte(combined.String())
}

func releaseProducerSurface(t *testing.T) []byte {
	t.Helper()
	var combined strings.Builder
	for _, path := range []string{
		"scripts/companion-release/produce.sh",
		"scripts/companion-release/produce-public-key-receipt.sh",
	} {
		combined.WriteString("\n# source: " + filepath.Base(path) + "\n")
		combined.Write(releaseSourceFile(t, path))
	}
	return []byte(combined.String())
}

func exactA0TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "69")
}

func exactA2TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "71")
}

func exactA3TagVersionGuard(source string) bool {
	return exactLineageTagVersionGuard(source, "72")
}

func exactLineageTagVersionGuard(source, patch string) bool {
	patterns := []*regexp.Regexp{
		regexp.MustCompile(`GITHUB_REF_NAME.{0,240}v0\.50\.` + patch + `.{0,400}COMPANION_VERSION.{0,240}0\.50\.` + patch),
		regexp.MustCompile(`COMPANION_VERSION.{0,240}0\.50\.` + patch + `.{0,400}GITHUB_REF_NAME.{0,240}v0\.50\.` + patch),
	}
	for _, pattern := range patterns {
		if pattern.MatchString(source) {
			return true
		}
	}
	return false
}

func publicKeyReceiptLineageStep(t *testing.T, workflow publicKeyReceiptWorkflow) (int, publicKeyReceiptWorkflowStep) {
	t.Helper()
	index, step, ok := findPublicKeyReceiptLineageStep(workflow)
	if !ok {
		t.Fatal("missing production contract: release workflow has no prior-release receipt lineage preflight")
	}
	return index, step
}

func findPublicKeyReceiptLineageStep(workflow publicKeyReceiptWorkflow) (int, publicKeyReceiptWorkflowStep, bool) {
	job, ok := workflow.Jobs["release"]
	if !ok {
		return -1, publicKeyReceiptWorkflowStep{}, false
	}
	for index, step := range job.Steps {
		surface := strings.ToLower(step.Name + " " + step.Run)
		if strings.Contains(surface, "public-key") &&
			(strings.Contains(surface, "lineage") || strings.Contains(surface, "prior")) {
			return index, step, true
		}
	}
	return -1, publicKeyReceiptWorkflowStep{}, false
}
