package scaffold

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/capture"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// TestCaptureFixtureProducesRedactedShard drives the emitted fixture with a stub
// Playwright module and a fake page, then merges the shard with the emitted
// reporter and gates the result through the Go contract.
//
// This is the only check that exercises the fixture's derivation rules. A typed
// URL, a fill value, or a below-threshold console line leaking into the shard
// would otherwise reach published evidence, and a url_ref shaped like a URL
// would reject the whole index at run time.
func TestCaptureFixtureProducesRedactedShard(t *testing.T) {
	t.Parallel()
	node := requireNode(t)
	dir := playwrightProject(t)
	_, err := Init(Options{ProjectDir: dir, ProjectDirExplicit: true})
	require.NoError(t, err)

	var pack journey.Pack
	require.NoError(t, yaml.Unmarshal([]byte(desktopGUIExplorePackExample(projectSignals{PackageManager: "npm"})), &pack))
	captureDir := filepath.Join(dir, ".autopus", "qa", "runs", "local", "capture")
	policyPath := writeCapturePolicy(t, captureDir, pack)
	writeFile(t, filepath.Join(dir, "e2e", "login.spec.ts"), "export default {};\n")
	assets := filepath.Join(dir, ".autopus", "qa", "capture")
	indexPath := filepath.Join(captureDir, capture.IndexFileName)

	command := exec.Command(node, "-e", captureFixtureDriver)
	command.Dir = dir
	command.Env = append(os.Environ(),
		"AUTOPUS_TEST_FIXTURE="+filepath.Join(assets, "autopus-capture.fixture.cjs"),
		"AUTOPUS_TEST_REPORTER="+filepath.Join(assets, "autopus-capture.reporter.cjs"),
		"AUTOPUS_TEST_ROOT="+dir,
		"AUTOPUS_QAMESH_GUI_CAPTURE_POLICY_PATH="+policyPath,
		"AUTOPUS_QAMESH_GUI_CAPTURE_DIR="+captureDir,
		"AUTOPUS_QAMESH_GUI_CAPTURE_INDEX_PATH="+indexPath,
	)
	output, err := command.CombinedOutput()
	require.NoErrorf(t, err, "capture fixture driver: %s", output)

	index, err := capture.LoadIndex(indexPath)
	require.NoError(t, err)
	require.NoError(t, capture.Validate(index))
	assert.Empty(t, capture.Conform(index, pack.GUI.Capture))
	assert.Empty(t, capture.VerifyLocalMedia(index, captureDir))
	require.NoError(t, capture.AssertPublishable(capture.Sanitize(index)))

	require.Len(t, index.Steps, 1)
	step := index.Steps[0]
	assert.Equal(t, capture.StatusPassed, step.Status)
	assert.Equal(t, []capture.Action{
		{API: "goto", TargetRef: "origin:0/login"},
		{API: "fill", TargetRef: "#password"},
	}, clearDurations(step.Actions))

	// The info line is counted but not retained: the pack declared a warning
	// threshold, and retaining quieter output fails policy conformance.
	require.NotNil(t, step.Console)
	assert.Equal(t, 1, step.Console.Warnings)
	assert.Equal(t, 1, step.Console.Infos)
	require.Len(t, step.Console.Messages, 1)
	assert.Equal(t, capture.SeverityWarning, step.Console.Messages[0].Severity)
	assert.Equal(t, "origin:0/src/app.js", step.Console.Messages[0].SourceRef)

	require.NotNil(t, step.Network)
	assert.Equal(t, 1, step.Network.Requests)
	assert.Equal(t, 1, step.Network.Failures)
	require.Len(t, step.Network.Entries, 1)
	assert.Equal(t, "GET", step.Network.Entries[0].Method)
	assert.Equal(t, "origin:0/api/session", step.Network.Entries[0].URLRef)
	assert.Equal(t, 401, step.Network.Entries[0].Status)

	require.NotNil(t, step.Screenshot)
	assert.Equal(t, capture.RetentionLocalOnly, step.Screenshot.Retention)
	assert.Equal(t, 1280, step.Screenshot.Width)
	require.Len(t, index.Media, 1)
	assert.Equal(t, capture.StreamTrace, index.Media[0].Kind)
	assert.Equal(t, step.StepID, index.Media[0].StepID)
	require.NotNil(t, index.Replay)
	assert.Equal(t, []string{"e2e/login.spec.ts"}, index.Replay.SpecRefs)

	// Nothing the producer saw may reach the index verbatim.
	body := readString(t, indexPath)
	for _, secret := range []string{"hunter2", "token=secret", "http://127.0.0.1:1420"} {
		assert.NotContains(t, body, secret)
	}
	assert.NotContains(t, strings.ToLower(body), "capture_error")
}

func clearDurations(actions []capture.Action) []capture.Action {
	cleared := make([]capture.Action, 0, len(actions))
	for _, action := range actions {
		action.DurationMS = 0
		cleared = append(cleared, action)
	}
	return cleared
}

// captureFixtureDriver stands in for a Playwright worker: it stubs the runner
// module so the fixture can be required without Playwright installed, replays one
// test through the automatic fixture, then runs the reporter over the shard.
const captureFixtureDriver = `
const fs = require('node:fs');
const path = require('node:path');
const Module = require('node:module');

let definition = null;
const originalLoad = Module._load;
Module._load = function (request) {
  if (request === '@playwright/test') {
    return { test: { extend: function (value) { definition = value; return {}; } }, expect: {} };
  }
  return originalLoad.apply(Module, arguments);
};
require(process.env.AUTOPUS_TEST_FIXTURE);
Module._load = originalLoad;

const listeners = {};
const page = {
  on: function (event, handler) { (listeners[event] = listeners[event] || []).push(handler); },
  context: function () {
    return { tracing: {
      start: async function () {},
      stop: async function (options) { fs.writeFileSync(options.path, 'trace-bytes'); },
    } };
  },
  viewportSize: function () { return { width: 1280, height: 720 }; },
  screenshot: async function (options) { fs.writeFileSync(options.path, 'png-bytes'); },
  goto: async function (url) { return url; },
  fill: async function (selector) { return selector; },
};

function consoleMessage(type, text, url) {
  return {
    type: function () { return type; },
    text: function () { return text; },
    location: function () { return { url: url }; },
  };
}

function request() {
  return {
    method: function () { return 'get'; },
    url: function () { return 'http://127.0.0.1:1420/api/session?token=secret'; },
    resourceType: function () { return 'xhr'; },
    timing: function () { return { responseEnd: 33.4 }; },
    response: async function () { return { status: function () { return 401; } }; },
    sizes: async function () { return { responseBodySize: 12 }; },
  };
}

function emit(event, value) { (listeners[event] || []).forEach(function (handler) { handler(value); }); }

const root = process.env.AUTOPUS_TEST_ROOT;
const testInfo = {
  title: 'login shows the dashboard',
  titlePath: ['chromium', 'e2e/login.spec.ts', 'login shows the dashboard'],
  file: path.join(root, 'e2e', 'login.spec.ts'),
  config: { rootDir: root },
  status: 'passed',
  duration: 4000,
  errors: [],
  repeatEachIndex: 0,
  retry: 0,
};

(async function () {
  await definition.autopusCapture[0]({ page: page }, async function () {
    await page.goto('http://127.0.0.1:1420/login?token=secret#frag');
    await page.fill('#password', 'hunter2');
    emit('console', consoleMessage('warning', 'slow hydration', 'http://127.0.0.1:1420/src/app.js'));
    emit('console', consoleMessage('log', 'chatty debug line', ''));
    emit('response', { status: function () { return 401; } });
    emit('requestfinished', request());
  }, testInfo);
  const Reporter = require(process.env.AUTOPUS_TEST_REPORTER);
  const reporter = new Reporter({});
  reporter.onBegin({ rootDir: root }, {});
  reporter.onEnd({ status: 'passed' });
})().catch(function (error) {
  process.stderr.write(String((error && error.stack) || error) + '\n');
  process.exit(1);
});
`
