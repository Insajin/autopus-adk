package run

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func TestDesktopObservationRunner_LocalWithoutOrcaSkipsLookupResolveConstructorAndSubprocess(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	require.NoError(t, os.MkdirAll(binDir, 0o755))
	counterPath := filepath.Join(dir, "orca-subprocess-count")
	sentinel := []byte("#!/bin/sh\nprintf x >> \"$ORCA_SENTINEL_COUNTER\"\nexit 1\n")
	require.NoError(t, os.WriteFile(filepath.Join(binDir, "orca"), sentinel, 0o700))
	t.Setenv("PATH", binDir)
	t.Setenv("ORCA_SENTINEL_COUNTER", counterPath)

	resolver := &fakeDesktopProviderResolver{
		local: newFakeDesktopClient(desktopobserve.RuntimeProviderLocal),
		orca:  newFakeDesktopClient(desktopobserve.RuntimeProviderOrca),
	}
	runner := newDesktopObservationRunnerWithResolver(resolver)
	outcome, err := runner.Run(context.Background(), desktopRunRequest(desktopobserve.RuntimeProviderLocal))
	require.NoError(t, err)
	assert.Equal(t, desktopobserve.VerdictPassed, outcome.Verdict)
	assert.Equal(t, "autopus-desktop-local", outcome.RuntimeReceipt.Provider.Name)
	assert.Equal(t, 1, resolver.localResolveCalls)
	assert.Equal(t, []string{"handshake", "capabilities", "permissions", "list_apps", "list_windows", "get_state"}, resolver.local.calls)
	assert.Zero(t, resolver.orcaLookupCalls)
	assert.Zero(t, resolver.orcaResolveCalls)
	assert.Zero(t, resolver.orcaConstructorCalls)
	assert.Empty(t, resolver.orca.calls)
	assert.NoFileExists(t, counterPath)
}

func TestDesktopObservationRunner_ProviderResolutionIsBoundedWithoutFallback(t *testing.T) {
	t.Parallel()
	resolver := &blockingDesktopProviderResolver{}
	runner := newDesktopObservationRunnerWithResolver(resolver)
	runner.timeout = 10 * time.Millisecond
	started := time.Now()
	outcome, err := runner.Run(
		context.Background(), desktopRunRequest(desktopobserve.RuntimeProviderLocal),
	)
	require.NoError(t, err)
	require.NotNil(t, outcome.ReasonCode)
	assert.Equal(t, desktopobserve.ReasonProviderUnavailable, *outcome.ReasonCode)
	assert.Less(t, time.Since(started), 500*time.Millisecond)
	assert.Equal(t, 1, resolver.localResolveCalls)
	assert.Zero(t, resolver.orcaLookupCalls)
}

func TestExecuteDesktopObservation_ProductionConstructorRunsControlledProviderProcess(t *testing.T) {
	if !secureDesktopSpawnSupported() {
		t.Skip("production process execution requires secure Darwin cgo spawn")
	}
	if desktopRaceEnabled {
		t.Skip("race instrumentation distorts the strict two-second subprocess integration clock")
	}
	dir := t.TempDir()
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	packBody, err := json.Marshal(desktopObservationPack())
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, "desktop-observe.yaml"), packBody, 0o644))
	artifactPath, callsPath := writeControlledDesktopProvider(t, dir)

	output := filepath.Join(dir, ".autopus", "qa", "runs")
	result, err := Execute(Options{
		ProjectDir: dir, Lane: "desktop-native", JourneyID: "desktop-accessibility-observe",
		AdapterID: "desktop-accessibility-observe", RuntimeProvider: desktopobserve.RuntimeProviderLocal,
		Output: output, desktopArtifactPath: artifactPath,
		desktopArtifactVerifier: func(context.Context, string) error { return nil },
	})
	require.NoError(t, err)
	assert.Equal(t, "passed", result.Status)
	require.Len(t, result.AdapterResults, 1)
	require.NotNil(t, result.AdapterResults[0].DesktopObservation)
	assert.NoDirExists(t, filepath.Join(output, result.RunID, "_raw"))
	calls, err := os.ReadFile(callsPath)
	require.NoError(t, err)
	assert.Equal(t, []string{
		"capabilities", "permissions", "list_apps", "list_windows", "get_state",
	}, strings.Fields(string(calls)))
}

func writeControlledDesktopProvider(t *testing.T, root string) (string, string) {
	t.Helper()
	artifactPath := filepath.Join(root, "Autopus Desktop.app")
	executableDir := filepath.Join(artifactPath, "Contents", "MacOS")
	require.NoError(t, os.MkdirAll(executableDir, 0o755))
	projectionBody := controlledDesktopV2Projection(t)
	if secureDesktopSpawnSupported() {
		source := strings.Replace(controlledDesktopProviderSource,
			"__PROJECTION__", string(projectionBody), 1)
		sourcePath := filepath.Join(executableDir, "provider.go")
		require.NoError(t, os.WriteFile(sourcePath, []byte(source), 0o600))
		executable := filepath.Join(executableDir, desktopProviderExecutableName)
		command := exec.Command("go", "build", "-o", executable, sourcePath)
		output, err := command.CombinedOutput()
		require.NoError(t, err, string(output))
		return artifactPath, filepath.Join(executableDir, "provider-calls")
	}

	script := strings.NewReplacer(
		"__PROJECTION__", string(projectionBody),
	).Replace(`#!/bin/sh
if [ "$#" -ne 3 ] || [ "$1" != "--desktop-observe-spike-provider" ] || [ "$2" != "--protocol-version" ] || [ "$3" != "2" ]; then
  exit 64
fi
if [ "$AUTOPUS_Q12_PROVENANCE_MARKER" != "autopus.desktop-observe.candidate.release.v2" ] || [ "$AUTOPUS_Q12_SUPERVISOR_RECIPE" != "autopus.desktop-observe.supervisor-recipe.v2" ]; then
  exit 78
fi
IFS= read -r request || exit 65
value=${request#*\"request_id\":\"}
request_id=${value%%\"*}
value=${request#*\"operation\":\"}
operation=${value%%\"*}
log=${0%/*}/provider-calls
printf '%s\n' "$operation" >> "$log"
case "$operation" in
  capabilities) payload='{"capabilities":["capabilities","get_state","list_apps","list_windows","permissions"]}' ;;
  permissions) payload='{"accessibility":"granted","next_step":null}' ;;
  list_apps) payload='{"app_refs":["autopus-desktop"]}' ;;
  list_windows) payload='{"window_refs":["main-window"]}' ;;
  get_state) payload='{"semantic_projection":__PROJECTION__}' ;;
  *) exit 65 ;;
esac
case "$operation" in
  list_windows) scope='{"kind":"application","public_ref":"autopus-desktop"}' ;;
  get_state) scope='{"kind":"window","public_ref":"main-window"}' ;;
  *) scope='{"kind":"provider","public_ref":"provider_selected"}' ;;
esac
redaction=not_required
if [ "$operation" = "get_state" ]; then redaction=applied; fi
capabilities='[{"name":"capabilities","status":"supported"},{"name":"get_state","status":"supported"},{"name":"list_apps","status":"supported"},{"name":"list_windows","status":"supported"},{"name":"permissions","status":"supported"}]'
printf '{"protocol_version":2,"request_id":"%s","status":"ok","runtime_receipt":{"schema_version":"qamesh.runtime_receipt.v1","provider":{"name":"rust-go","version":"0.0.1","protocol_version":2},"scope":%s,"capability_summary":%s,"reason_code":null,"next_step":null,"redaction":{"status":"%s"},"quarantine":{"status":"empty"}},"payload":%s}\n' "$request_id" "$scope" "$capabilities" "$redaction" "$payload"
`)
	executable := filepath.Join(executableDir, desktopProviderExecutableName)
	require.NoError(t, os.WriteFile(executable, []byte(script), 0o700))
	return artifactPath, filepath.Join(executableDir, "provider-calls")
}

const controlledDesktopProviderSource = `package main
import ("encoding/json"; "os"; "path/filepath")
type request struct { RequestID string ` + "`json:\"request_id\"`" + `; Operation string ` + "`json:\"operation\"`" + ` }
func main() {
  if len(os.Args) != 4 || os.Args[1] != "--desktop-observe-spike-provider" ||
    os.Args[2] != "--protocol-version" || os.Args[3] != "2" ||
    os.Getenv("AUTOPUS_Q12_PROVENANCE_MARKER") != "autopus.desktop-observe.candidate.release.v2" ||
    os.Getenv("AUTOPUS_Q12_SUPERVISOR_RECIPE") != "autopus.desktop-observe.supervisor-recipe.v2" { os.Exit(64) }
  var input request
  if json.NewDecoder(os.Stdin).Decode(&input) != nil { os.Exit(65) }
  executable, _ := os.Executable()
  logPath := filepath.Join(filepath.Dir(executable), "provider-calls")
  log, _ := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
  if log != nil { _, _ = log.WriteString(input.Operation+"\n"); _ = log.Close() }
  payloads := map[string]json.RawMessage{
    "capabilities": json.RawMessage(` + "`{\"capabilities\":[\"capabilities\",\"get_state\",\"list_apps\",\"list_windows\",\"permissions\"]}`" + `),
    "permissions": json.RawMessage(` + "`{\"accessibility\":\"granted\",\"next_step\":null}`" + `),
    "list_apps": json.RawMessage(` + "`{\"app_refs\":[\"autopus-desktop\"]}`" + `),
    "list_windows": json.RawMessage(` + "`{\"window_refs\":[\"main-window\"]}`" + `),
    "get_state": json.RawMessage(` + "`{\"semantic_projection\":__PROJECTION__}`" + `),
  }
  payload, ok := payloads[input.Operation]; if !ok { os.Exit(65) }
  scopeKind, scopeRef, redaction := "provider", "provider_selected", "not_required"
  if input.Operation == "list_windows" { scopeKind, scopeRef = "application", "autopus-desktop" }
  if input.Operation == "get_state" { scopeKind, scopeRef, redaction = "window", "main-window", "applied" }
  capabilities := []map[string]string{{"name":"capabilities","status":"supported"},
    {"name":"get_state","status":"supported"},{"name":"list_apps","status":"supported"},
    {"name":"list_windows","status":"supported"},{"name":"permissions","status":"supported"}}
  output := map[string]any{"protocol_version":2,"request_id":input.RequestID,"status":"ok","payload":payload,
    "runtime_receipt":map[string]any{"schema_version":"qamesh.runtime_receipt.v1",
      "provider":map[string]any{"name":"rust-go","version":"0.0.1","protocol_version":2},
      "scope":map[string]string{"kind":scopeKind,"public_ref":scopeRef},"capability_summary":capabilities,
      "reason_code":nil,"next_step":nil,"redaction":map[string]string{"status":redaction},
      "quarantine":map[string]string{"status":"empty"}}}
  if json.NewEncoder(os.Stdout).Encode(output) != nil { os.Exit(66) }
}
`

func controlledDesktopV2Projection(t *testing.T) []byte {
	t.Helper()
	body := []byte(`{"schema_version":"autopus.desktop-observe.semantic-projection.v2","provider_ref":"provider_rust_go","app_ref":"autopus-desktop","window_ref":"main-window","state_ref":"state_v2_0000000000000000000000000000000000000000000000000000000000000000","digest":"0000000000000000000000000000000000000000000000000000000000000000","nodes":[{"advertised_actions":[],"children":[{"advertised_actions":[],"children":[{"advertised_actions":["AXPress"],"children":[],"name":"Disclosure","node_ref":"disclosure_controlled_00","occurrence":0,"parent_node_ref":"window_controlled_00","role":"AXButton","semantic_state":{"enabled":true,"expanded":false}}],"name":"Autopus","node_ref":"window_controlled_00","occurrence":0,"parent_node_ref":"application_controlled_00","role":"AXWindow","semantic_state":{"focused":true}}],"name":"Autopus","node_ref":"application_controlled_00","occurrence":0,"parent_node_ref":null,"role":"AXApplication","semantic_state":{"enabled":true}}]}`)
	projection, err := decodeDesktopV2Projection(body)
	require.NoError(t, err)
	canonical, err := desktopV2CanonicalBytes(projection)
	require.NoError(t, err)
	digest := fmt.Sprintf("%x", sha256.Sum256(canonical))
	return []byte(strings.Replace(string(body), strings.Repeat("0", 64)+`","nodes`, digest+`","nodes`, 1))
}

type blockingDesktopProviderResolver struct {
	localResolveCalls int
	orcaLookupCalls   int
}

func (resolver *blockingDesktopProviderResolver) ResolveLocal(ctx context.Context) (desktopProviderClient, error) {
	resolver.localResolveCalls++
	<-ctx.Done()
	return nil, ctx.Err()
}

func (resolver *blockingDesktopProviderResolver) LookPath(context.Context, string) (string, error) {
	resolver.orcaLookupCalls++
	return "", context.Canceled
}

func (*blockingDesktopProviderResolver) ResolveOrca(context.Context, string) (string, error) {
	return "", context.Canceled
}

func (*blockingDesktopProviderResolver) NewOrcaClient(context.Context, string) (desktopProviderClient, error) {
	return nil, context.Canceled
}

type fakeDesktopProviderResolver struct {
	local *fakeDesktopClient
	orca  *fakeDesktopClient

	localResolveCalls    int
	orcaLookupCalls      int
	orcaResolveCalls     int
	orcaConstructorCalls int
}

func (resolver *fakeDesktopProviderResolver) ResolveLocal(context.Context) (desktopProviderClient, error) {
	resolver.localResolveCalls++
	return resolver.local, nil
}

func (resolver *fakeDesktopProviderResolver) LookPath(context.Context, string) (string, error) {
	resolver.orcaLookupCalls++
	return "orca", nil
}

func (resolver *fakeDesktopProviderResolver) ResolveOrca(context.Context, string) (string, error) {
	resolver.orcaResolveCalls++
	return "orca://resolved", nil
}

func (resolver *fakeDesktopProviderResolver) NewOrcaClient(context.Context, string) (desktopProviderClient, error) {
	resolver.orcaConstructorCalls++
	return resolver.orca, nil
}
