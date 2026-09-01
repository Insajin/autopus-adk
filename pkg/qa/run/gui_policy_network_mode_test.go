package run

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGUIGuardScriptEnforcesNetworkModes pins the runtime behavior each mode
// promises. The three values used to be validated labels with identical effect,
// so a pack could declare local-only and still reach any origin.
func TestGUIGuardScriptEnforcesNetworkModes(t *testing.T) {
	t.Parallel()

	script := guiPolicyGuardScript()
	assert.Contains(t, script, `page.route("**/*", networkRouteHandler)`,
		"non-observational modes must install route interception")
	assert.Contains(t, script, `networkMode === "summary-only"`,
		"summary-only must remain non-intercepting")
	assert.Contains(t, script, `networkMode === "blocked"`,
		"blocked must additionally stop data traffic")
	assert.Contains(t, script, `route.abort("blockedbyclient")`)
	assert.Contains(t, script, `await networkReady`,
		"the first navigation must not outrun route registration")
	// data:, blob:, and file: never leave the machine and must not be aborted.
	assert.Contains(t, script, "isNetworkScheme")
}

func TestReadGuardReceipt_ParsesRequestEvents(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "gui-policy-guard-receipt.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(strings.Join([]string{
		`{"t":"goto","origin":"http://127.0.0.1:4173","allowed":true}`,
		`{"t":"request","origin":"http://127.0.0.1:4173","resource_type":"document","blocked":false}`,
		`{"t":"request","origin":"http://127.0.0.1:4174","resource_type":"fetch","blocked":true,"reason":"off_origin"}`,
		`{"t":"request","origin":"http://127.0.0.1:4174","resource_type":"xhr","blocked":true,"reason":"off_origin"}`,
		`{"t":"request","origin":"http://127.0.0.1:4173","resource_type":"fetch","blocked":true,"reason":"data_traffic_blocked"}`,
		`{"t":"action","method":"click","blocked":true}`,
		"not json at all",
	}, "\n")+"\n"), 0o644))

	receipt, err := readGuardReceipt(path)
	require.NoError(t, err)
	assert.Equal(t, 1, receipt.Navigations)
	assert.Equal(t, 4, receipt.Requests)
	assert.Equal(t, 1, receipt.Actions)
	// Repeated aborts of the same origin and reason collapse into one finding.
	assert.Equal(t, []string{
		"data_traffic_blocked:http://127.0.0.1:4173",
		"off_origin:http://127.0.0.1:4174",
	}, receipt.BlockedRequests)
	assert.Equal(t, []string{"forbidden_action:click"}, receipt.BlockedActions)
	assert.True(t, receipt.Enforced())
}

// TestReadGuardReceipt_RequestsAloneProveEnforcement covers a journey that only
// loaded a page: interception is still demonstrated by the routed requests.
func TestReadGuardReceipt_RequestsAloneProveEnforcement(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "receipt.jsonl")
	require.NoError(t, os.WriteFile(path, []byte(
		`{"t":"request","origin":"http://127.0.0.1:4173","resource_type":"document","blocked":false}`+"\n"), 0o644))

	receipt, err := readGuardReceipt(path)
	require.NoError(t, err)
	assert.True(t, receipt.Enforced())
	assert.Empty(t, receipt.BlockedRequests)
}

// TestExecuteGUICaptureReportsStoppedRequests proves a route-stopped request
// reaches the evidence without failing the journey: aborting under local-only is
// the pack's declared intent, not a violation of it.
func TestExecuteGUICaptureReportsStoppedRequests(t *testing.T) {
	dir := fixtureGUICaptureProject(t, nil)
	prependFakeCaptureProducer(t, dir, captureProducerScript("stopped-request"))

	result, err := Execute(Options{ProjectDir: dir, Profile: "local", Lane: "gui-explore", Output: filepath.Join(dir, "runs")})

	require.NoError(t, err)
	assert.Equal(t, "passed", result.Status)
	check := manifestCheck(t, loadManifest(t, projectPath(dir, result.ManifestPaths[0])), guiPolicyRuntimeCheckID)
	assert.Equal(t, "passed", check.Status)
	assert.Contains(t, check.Actual, "network_stopped=off_origin:http://cdn.example")
	assert.Contains(t, check.Actual, "runtime_policy_enforced=true")
}
