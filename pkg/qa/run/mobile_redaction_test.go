package run

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// handleEchoRunner simulates the real-world leak vector: maestro/adb/simctl echo
// the concrete device handle (MAESTRO_DEVICE / `adb -s <handle>`) into stdout,
// which is captured as a published sanitized_log artifact. Resolve always
// resolves to the configured handle so the spine reaches RunFlow.
type handleEchoRunner struct{ handle string }

func (r handleEchoRunner) Resolve(req mobileResolveRequest) (string, bool) { return r.handle, true }

func (r handleEchoRunner) InstallApp(ctx context.Context, req mobileInstallRequest) error { return nil }

func (r handleEchoRunner) RunFlow(ctx context.Context, req mobileFlowRequest) commandResult {
	_ = os.MkdirAll(req.ArtifactDir, 0o755)
	stdout := filepath.Join(req.ArtifactDir, "stdout.log")
	stderr := filepath.Join(req.ArtifactDir, "stderr.log")
	_ = os.WriteFile(stdout, []byte("Running flow on device "+req.Handle+"\nadb -s "+req.Handle+" shell am start\nflow ok\n"), 0o644)
	_ = os.WriteFile(stderr, []byte("warning from target "+req.Handle+"\n"), 0o644)
	now := time.Now().UTC()
	return commandResult{
		Status:     "passed",
		ExitCode:   0,
		StdoutPath: stdout,
		StderrPath: stderr,
		StartedAt:  now,
		EndedAt:    now.Add(time.Second),
		DurationMS: 1000,
		Command:    "maestro test " + req.Pack.Mobile.FlowPath,
	}
}

// TestExecuteMobilePackRedactsHandleFromPublishedLogs is the regression test for
// the device-handle redaction boundary (INV-Q8-002 / REQ-QAMESH8-RESOLVE-01):
// the concrete handle echoed into the flow stdout must never survive into the
// published run tree. Covers the two formats the publication regex missed:
// the default Android emulator handle and a dashed iOS simulator UDID.
func TestExecuteMobilePackRedactsHandleFromPublishedLogs(t *testing.T) {
	for _, handle := range []string{"emulator-5554", "A1B2C3D4-E5F6-7890-ABCD-EF1234567890"} {
		handle := handle
		t.Run(handle, func(t *testing.T) {
			dir := t.TempDir()
			writeReadyMobileReadiness(t, dir)
			runDir := filepath.Join(dir, "runs", "qa-redact")
			pack := mobileScriptedJourneyPack(".autopus/qa/mobile/flows/smoke.yaml")

			result, manifestPath, _ := executeMobilePack(
				Options{ProjectDir: dir, Lane: "mobile-scripted", deviceRunner: handleEchoRunner{handle: handle}},
				pack, filepath.Join(runDir, "_raw"), runDir,
			)

			require.Equal(t, "passed", result.Status)
			require.NotEmpty(t, manifestPath)

			// The concrete handle must appear in NO file under the run tree
			// (raw artifacts + copied published manifest artifacts).
			found, placeholderSeen := scanTreeForHandle(t, runDir, handle, redactedDeviceHandle)
			assert.Emptyf(t, found, "handle %q leaked into published files: %v", handle, found)
			// The placeholder must be present, proving redaction ran (not mere absence).
			assert.Truef(t, placeholderSeen, "expected %q in a redacted log for handle %q", redactedDeviceHandle, handle)
		})
	}
}

// scanTreeForHandle walks root and returns the relative paths of any files whose
// body contains needle, plus whether the placeholder was observed anywhere.
func scanTreeForHandle(t *testing.T, root, needle, placeholder string) ([]string, bool) {
	t.Helper()
	var leaked []string
	placeholderSeen := false
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return nil
		}
		text := string(body)
		if strings.Contains(text, needle) {
			rel, _ := filepath.Rel(root, path)
			leaked = append(leaked, rel)
		}
		if strings.Contains(text, placeholder) {
			placeholderSeen = true
		}
		return nil
	})
	require.NoError(t, err)
	return leaked, placeholderSeen
}
