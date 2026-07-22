package companionmanifest

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func darwinReleaseToolEnv(t *testing.T, dir string) []string {
	t.Helper()
	tools := filepath.Join(dir, "tools")
	if err := os.MkdirAll(tools, 0o700); err != nil {
		t.Fatal(err)
	}
	apiKey := filepath.Join(dir, "AuthKey_FIXTURE.p8")
	if err := os.WriteFile(apiKey, []byte("fixture-api-key"), 0o600); err != nil {
		t.Fatal(err)
	}
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	values := []string{
		"FAKE_DARWIN_EVENTS=" + filepath.Join(dir, "events"),
		"APPLE_SIGNING_IDENTITY=Developer ID Application: Fixture (GP2PFA2PUV)",
		"APPLE_API_KEY=FIXTUREKEY",
		"APPLE_API_ISSUER=123e4567-e89b-42d3-a456-426614174000",
		"APPLE_API_KEY_PATH=" + apiKey,
	}
	for _, tool := range []string{"codesign", "ditto", "xcrun", "plutil", "shasum"} {
		path := filepath.Join(tools, tool)
		script := fmt.Sprintf("#!/usr/bin/env bash\nexec %q -test.run=TestDarwinReleaseToolHelper -- %s \"$@\"\n", executable, tool)
		if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
			t.Fatal(err)
		}
		values = append(values, "COMPANION_"+strings.ToUpper(tool)+"_TOOL="+path)
	}
	execSmokeGate := filepath.Join(tools, "exec-smoke-gate")
	execSmokeScript := fmt.Sprintf(`#!/usr/bin/env bash
set -euo pipefail
[[ "$#" -eq 8 && "$1" == '--artifact' && -x "$2" &&
   "$3" == '--expected-version' && -n "$4" &&
   "$5" == '--architecture' && "$6" =~ ^(amd64|arm64)$ &&
   "$7" == '--timeout' && "$8" == '15s' ]] || exit 91
scenario=''; [[ ! -f %q ]] || scenario="$(cat %q)"
[[ "$scenario" != 'execution_smoke_failure' ]] || exit 42
[[ "$scenario" != 'execution_smoke_mutation' ]] || printf mutated >> "$2"
printf 'execution_smoke\n' >> %q
`, filepath.Join(dir, "scenario"), filepath.Join(dir, "scenario"), filepath.Join(dir, "events"))
	if err := os.WriteFile(execSmokeGate, []byte(execSmokeScript), 0o700); err != nil {
		t.Fatal(err)
	}
	values = append(values, "COMPANION_EXEC_SMOKE_GATE="+execSmokeGate)
	return values
}

func appendDarwinReleaseEvent(t *testing.T, event string) {
	t.Helper()
	file, err := os.OpenFile(os.Getenv("FAKE_DARWIN_EVENTS"), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	if _, err := fmt.Fprintln(file, event); err != nil {
		t.Fatal(err)
	}
}

func writeDarwinReleaseScenario(t *testing.T, dir, scenario string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "scenario"), []byte(scenario), 0o600); err != nil {
		t.Fatal(err)
	}
}
