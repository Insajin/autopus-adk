package run

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestGUIGuardScriptCoversMutatingActions pins the Go list against the embedded
// preload. If the two drift, the harness would either claim it cannot enforce an
// action the guard actually blocks, or stay silent about one it cannot.
func TestGUIGuardScriptCoversMutatingActions(t *testing.T) {
	t.Parallel()

	script := guiPolicyGuardScript()
	for _, action := range guiMutatingActions() {
		assert.Contains(t, strings.ToLower(script), `"`+action+`"`,
			"guard script must patch or expand %q", action)
	}
	assert.Contains(t, script, `forbiddenActions.has("`+guiMutationActionClass+`")`,
		"the mutation class is the only label with expansion power")
}

func TestUnenforceableForbiddenActions(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		declared []string
		want     []string
	}{
		{"mutation class only", []string{"mutation"}, nil},
		{"exact method names", []string{"click", "fill", "dragTo"}, nil},
		{"case and space tolerant", []string{"  Mutation ", "SelectOption"}, nil},
		{"business labels", []string{"mutation", "payment", "email_send"}, []string{"payment", "email_send"}},
		{"deduplicated", []string{"payment", "payment"}, []string{"payment"}},
		{"blanks ignored", []string{"mutation", "  "}, nil},
	}
	for _, testCase := range cases {
		testCase := testCase
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, testCase.want, unenforceableForbiddenActions(testCase.declared))
		})
	}
}

// TestExecuteGUICaptureReportsUnenforceableForbiddenActions proves the gap reaches
// evidence: the journey still passes, but the check names what cannot be enforced.
func TestExecuteGUICaptureReportsUnenforceableForbiddenActions(t *testing.T) {
	dir := fixtureGUICaptureProject(t, nil)
	patchGUIPackForbiddenActions(t, dir, []string{"mutation", "payment"})
	prependFakeCaptureProducer(t, dir, captureProducerScript(""))

	result, err := Execute(Options{ProjectDir: dir, Profile: "local", Lane: "gui-explore", Output: filepath.Join(dir, "runs")})

	require.NoError(t, err)
	assert.Equal(t, "passed", result.Status, "an unenforceable label must not block a journey")
	check := manifestCheck(t, loadManifest(t, result.ManifestPaths[0]), guiPolicyRuntimeCheckID)
	assert.Equal(t, "passed", check.Status)
	assert.Contains(t, check.Actual, "unenforceable_forbidden_actions=payment")
}

func patchGUIPackForbiddenActions(t *testing.T, dir string, actions []string) {
	t.Helper()
	path := filepath.Join(dir, ".autopus", "qa", "journeys", "gui-capture-smoke.yaml")
	body, err := os.ReadFile(path)
	require.NoError(t, err)
	var pack map[string]any
	require.NoError(t, json.Unmarshal(body, &pack))
	pack["gui"].(map[string]any)["forbidden_actions"] = actions
	patched, err := json.Marshal(pack)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, patched, 0o644))
}
