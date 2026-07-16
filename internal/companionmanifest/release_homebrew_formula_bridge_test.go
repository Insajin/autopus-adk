package companionmanifest

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const priorFormulaBlob = "df2d8e25636f8a3db842948d119d46f31afd94ab"

var bridgeDigests = []string{
	strings.Repeat("1", 64), strings.Repeat("2", 64),
	strings.Repeat("3", 64), strings.Repeat("4", 64),
}

func TestHomebrewFormulaBridge_PublishesThenIsIdempotent(t *testing.T) {
	fixture := newHomebrewBridgeFixture(t)
	output, err := fixture.run(nil)
	if err != nil {
		t.Fatalf("publish Formula bridge: %v\n%s", err, output)
	}
	formula := fixture.formulaContent(t)
	for _, required := range []string{
		`version "0.50.71"`,
		`deprecate! date: "2026-07-16", because: "is distributed as a signed cask", replacement_cask: "auto"`,
		`bin.install "auto"`, `brew uninstall --formula auto`,
		`brew install --cask Insajin/autopus/auto`,
		`assert_match version.to_s, shell_output("#{bin}/auto version")`,
	} {
		if !strings.Contains(formula, required) {
			t.Fatalf("published Formula missing %q", required)
		}
	}
	if got := strings.Count(formula, "releases/download/v0.50.71/"); got != 4 {
		t.Fatalf("published Formula archive URL count = %d, want 4", got)
	}
	if got := fixture.updateCount(t); got != "1" {
		t.Fatalf("Formula update count = %q, want 1", got)
	}
	output, err = fixture.run(nil)
	if err != nil || !strings.Contains(string(output), "already current") {
		t.Fatalf("idempotent run: %v\n%s", err, output)
	}
	if got := fixture.updateCount(t); got != "1" {
		t.Fatalf("idempotent update count = %q, want 1", got)
	}
	output, err = fixture.run(map[string]string{"HOMEBREW_TAP_TOKEN": "", "GH_TOKEN": "fallback-token"})
	if err != nil {
		t.Fatalf("GH_TOKEN fallback: %v\n%s", err, output)
	}
}

func TestHomebrewFormulaBridge_RejectsRemoteDriftAndCaskMismatch(t *testing.T) {
	t.Run("prior Formula drift", func(t *testing.T) {
		fixture := newHomebrewBridgeFixture(t)
		fixture.writeAPIContent(t, "formula.json", strings.Repeat("a", 40), "drifted Formula\n")
		output, err := fixture.run(nil)
		if err == nil || !strings.Contains(string(output), "drifted from the pinned prior blob") {
			t.Fatalf("Formula drift result: %v\n%s", err, output)
		}
	})
	t.Run("published Cask mismatch", func(t *testing.T) {
		fixture := newHomebrewBridgeFixture(t)
		mismatch := strings.Replace(fixture.cask, bridgeDigests[0], strings.Repeat("f", 64), 1)
		fixture.writeAPIContent(t, "cask.json", strings.Repeat("c", 40), mismatch)
		output, err := fixture.run(nil)
		if err == nil || !strings.Contains(string(output), "differs from canonical GoReleaser v2.17.0 output") {
			t.Fatalf("Cask mismatch result: %v\n%s", err, output)
		}
		if got := fixture.updateCount(t); got != "0" {
			t.Fatalf("Cask mismatch performed %s updates", got)
		}
	})
}

func TestHomebrewFormulaBridge_RejectsChecksumAmbiguity(t *testing.T) {
	for _, test := range []struct {
		name   string
		mutate func(string) string
		want   string
	}{
		{"duplicate", func(value string) string { return value + strings.Split(value, "\n")[0] + "\n" }, "duplicate archive"},
		{"missing", func(value string) string { return strings.Join(strings.Split(value, "\n")[1:], "\n") }, "missing an exact bridge archive"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newHomebrewBridgeFixture(t)
			if err := os.WriteFile(fixture.checksums, []byte(test.mutate(fixture.checksumText)), 0o600); err != nil {
				t.Fatal(err)
			}
			output, err := fixture.run(nil)
			if err == nil || !strings.Contains(string(output), test.want) {
				t.Fatalf("checksum %s result: %v\n%s", test.name, err, output)
			}
		})
	}
}

func TestHomebrewFormulaBridge_RejectsIdentityMismatchWithoutCredentialLeak(t *testing.T) {
	for _, test := range []struct{ name, key, value string }{
		{"tag", "GITHUB_REF_NAME", "v0.50.72"},
		{"version", "COMPANION_VERSION", "0.50.72"},
	} {
		t.Run(test.name, func(t *testing.T) {
			fixture := newHomebrewBridgeFixture(t)
			output, err := fixture.run(map[string]string{test.key: test.value})
			if err == nil || !strings.Contains(string(output), "one-release Formula bridge policy") {
				t.Fatalf("identity mismatch result: %v\n%s", err, output)
			}
		})
	}
	t.Run("credential", func(t *testing.T) {
		fixture := newHomebrewBridgeFixture(t)
		secret := "tap-token-must-never-appear"
		output, err := fixture.run(map[string]string{
			"HOMEBREW_TAP_TOKEN": secret, "MOCK_FAIL_WITH_TOKEN": "1",
		})
		if err == nil || strings.Contains(string(output), secret) {
			t.Fatalf("credential failure leaked token: %v\n%s", err, output)
		}
	})
}

type homebrewBridgeFixture struct {
	root, state, checksums, checksumText, cask string
}

func newHomebrewBridgeFixture(t *testing.T) homebrewBridgeFixture {
	t.Helper()
	root := t.TempDir()
	state := filepath.Join(root, "state")
	bin := filepath.Join(root, "bin")
	for _, path := range []string{state, bin, filepath.Join(root, "tmp")} {
		if err := os.Mkdir(path, 0o700); err != nil {
			t.Fatal(err)
		}
	}
	fixture := homebrewBridgeFixture{root: root, state: state}
	fixture.cask = homebrewBridgeCask()
	fixture.checksumText = homebrewBridgeChecksums()
	fixture.checksums = filepath.Join(root, "checksums.txt")
	if err := os.WriteFile(fixture.checksums, []byte(fixture.checksumText), 0o600); err != nil {
		t.Fatal(err)
	}
	fixture.writeAPIContent(t, "cask.json", strings.Repeat("c", 40), fixture.cask)
	fixture.writeAPIContent(t, "formula.json", priorFormulaBlob, "legacy Formula\n")
	writeExecutable(t, filepath.Join(bin, "gh"), homebrewBridgeMockGH)
	return fixture
}

func (fixture homebrewBridgeFixture) run(overrides map[string]string) ([]byte, error) {
	environment := map[string]string{
		"PATH":   filepath.Join(fixture.root, "bin") + string(os.PathListSeparator) + os.Getenv("PATH"),
		"TMPDIR": filepath.Join(fixture.root, "tmp"), "MOCK_STATE": fixture.state,
		"GITHUB_REF_NAME": "v0.50.71", "COMPANION_VERSION": "0.50.71",
		"COMPANION_CHECKSUMS_PATH": fixture.checksums,
		"HOMEBREW_TAP_TOKEN":       "fixture-tap-token", "GH_TOKEN": "",
	}
	for name, value := range overrides {
		environment[name] = value
	}
	command := exec.Command("bash", filepath.Join(repositoryRootForBridge(),
		"scripts/companion-release/publish-homebrew-formula-bridge.sh"))
	command.Env = makeBridgeEnvironment(environment)
	return command.CombinedOutput()
}

func makeBridgeEnvironment(values map[string]string) []string {
	blocked := map[string]bool{}
	for name := range values {
		blocked[name] = true
	}
	environment := make([]string, 0, len(os.Environ())+len(values))
	for _, entry := range os.Environ() {
		name := strings.SplitN(entry, "=", 2)[0]
		if !blocked[name] {
			environment = append(environment, entry)
		}
	}
	for name, value := range values {
		environment = append(environment, name+"="+value)
	}
	return environment
}

func repositoryRootForBridge() string {
	root, _ := filepath.Abs(filepath.Join("..", ".."))
	return root
}

func (fixture homebrewBridgeFixture) writeAPIContent(t *testing.T, name, sha, content string) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{
		"sha": sha, "content": base64.StdEncoding.EncodeToString([]byte(content)),
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(fixture.state, name), payload, 0o600); err != nil {
		t.Fatal(err)
	}
}

func (fixture homebrewBridgeFixture) formulaContent(t *testing.T) string {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join(fixture.state, "formula.json"))
	if err != nil {
		t.Fatal(err)
	}
	var response map[string]string
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatal(err)
	}
	decoded, err := base64.StdEncoding.DecodeString(response["content"])
	if err != nil {
		t.Fatal(err)
	}
	return string(decoded)
}

func (fixture homebrewBridgeFixture) updateCount(t *testing.T) string {
	t.Helper()
	value, err := os.ReadFile(filepath.Join(fixture.state, "updates"))
	if os.IsNotExist(err) {
		return "0"
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(value))
}

func homebrewBridgeChecksums() string {
	names := []string{"darwin_amd64", "darwin_arm64", "linux_amd64", "linux_arm64"}
	var output strings.Builder
	for index, name := range names {
		fmt.Fprintf(&output, "%s  autopus-adk_0.50.71_%s.tar.gz\n", bridgeDigests[index], name)
	}
	fmt.Fprintf(&output, "%s  autopus-adk_0.50.71_windows_amd64.zip\n", strings.Repeat("5", 64))
	return output.String()
}

const homebrewBridgeMockGH = `#!/usr/bin/env bash
set -euo pipefail
[[ "$1" == 'api' ]]
shift
method='GET' input='' endpoint=''
while [[ "$#" -gt 0 ]]; do
  case "$1" in
    --method) method="$2"; shift 2 ;;
    --input) input="$2"; shift 2 ;;
    -H) shift 2 ;;
    *) endpoint="$1"; shift ;;
  esac
done
if [[ "${MOCK_FAIL_WITH_TOKEN-}" == '1' ]]; then
  printf 'mock diagnostic included %s\n' "$GH_TOKEN" >&2
  exit 72
fi
case "$method:$endpoint" in
  GET:*Casks/auto.rb*) exec cat "$MOCK_STATE/cask.json" ;;
  GET:*Formula/auto.rb*) exec cat "$MOCK_STATE/formula.json" ;;
  PUT:*Formula/auto.rb)
    current_sha=$(jq -er '.sha' "$MOCK_STATE/formula.json")
    [[ "$(jq -er '.sha' "$input")" == "$current_sha" ]]
    [[ "$(jq -er '.branch' "$input")" == 'main' ]]
    jq -er '.content' "$input" | tr -d '\r\n' | base64 --decode >"$MOCK_STATE/formula.rb"
    jq -n --rawfile body "$MOCK_STATE/formula.rb" \
      '{sha:"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",content:($body|@base64)}' \
      >"$MOCK_STATE/formula.json"
    count=$(cat "$MOCK_STATE/updates" 2>/dev/null || printf '0')
    printf '%s\n' "$((count + 1))" >"$MOCK_STATE/updates"
    printf '{}\n'
    ;;
  *) exit 64 ;;
esac
`
