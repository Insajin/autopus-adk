package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestJSONContract_ExistingJSONCommandsRequireEnvelopeAlignment(t *testing.T) {
	dir := makeJSONContractWorkspace(t)
	helperPath := writeRuntimeHelperScript(t, `
command="$0"
for arg in "$@"; do
	command="$command $arg"
done
timestamp="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
cat <<EOF
{
  "schema_version": "1.0.0",
  "command": "$command",
  "status": "ok",
  "generated_at": "$timestamp",
  "data": {
    "helper": true
  }
}
EOF
`)
	tests := []struct {
		name        string
		workdir     string
		args        []string
		wantCommand string
	}{
		{name: "permission detect", args: []string{"permission", "detect", "--json"}, wantCommand: "auto permission detect"},
		{name: "test run", args: []string{"test", "run", "--project-dir", dir, "--json"}, wantCommand: "auto test run"},
		{name: "desktop status", args: []string{"desktop", "status", "--json"}, wantCommand: "auto desktop status"},
		{name: "worker status", args: []string{"worker", "status", "--json"}, wantCommand: "auto worker status"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			env := []string{
				runtimeHelperOverrideEnv + "=" + helperPath,
			}
			out, err := executeRootWithEnv(t, tt.workdir, env, tt.args...)
			require.NoError(t, err)
			assertCommonJSONEnvelope(t, decodeJSONMap(t, out), tt.wantCommand)
		})
	}
}

func TestJSONContract_PhaseOneCommandsRequireEnvelopeSupport(t *testing.T) {
	dir := makeJSONContractWorkspace(t)
	tests := []struct {
		name        string
		workdir     string
		args        []string
		wantCommand string
	}{
		{name: "status", args: []string{"status", "--dir", dir, "--json"}, wantCommand: "auto status"},
		{name: "setup status", args: []string{"setup", "status", dir, "--json"}, wantCommand: "auto setup status"},
		{name: "setup validate", args: []string{"setup", "validate", dir, "--json"}, wantCommand: "auto setup validate"},
		{name: "doctor", args: []string{"doctor", "--dir", dir, "--json"}, wantCommand: "auto doctor"},
		{name: "telemetry summary", workdir: dir, args: []string{"telemetry", "summary", "--json"}, wantCommand: "auto telemetry summary"},
		{name: "telemetry cost", workdir: dir, args: []string{"telemetry", "cost", "--json"}, wantCommand: "auto telemetry cost"},
		{name: "telemetry compare", workdir: dir, args: []string{"telemetry", "compare", "--json"}, wantCommand: "auto telemetry compare"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			out, err := executeRoot(t, tt.workdir, tt.args...)
			require.NoError(t, err)
			assertCommonJSONEnvelope(t, decodeJSONMap(t, out), tt.wantCommand)
		})
	}
}
