package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQARunCmd_MobileScriptedLaneExecutesAndKeepsReadinessGapsOut(t *testing.T) {
	dir := t.TempDir()
	writeMobileScriptedReadiness(t, dir)
	writeMobileScriptedJourneyFile(t, dir)
	writeMobileDeviceMap(t, dir, "device-ref:android-pixel-7", "emulator-5554")
	installFakeBinaryOnPath(t, "maestro", "exit 0")
	output := filepath.Join(dir, "runs")

	cmd := newQACmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"run", "--project-dir", dir, "--output", output, "--lane", "mobile-scripted", "--format", "json"})

	require.NoError(t, cmd.Execute())
	data := decodeJSONMap(t, out.Bytes())["data"].(map[string]any)
	assert.Equal(t, "mobile-scripted", data["lane"])
	adapterResults, ok := data["adapter_results"].([]any)
	require.True(t, ok)
	require.NotEmpty(t, adapterResults)
	assert.Equal(t, "passed", adapterResults[0].(map[string]any)["status"])
	for _, raw := range stringSliceOfMaps(data["setup_gaps"]) {
		assert.NotEqual(t, "mobile-readiness", raw["adapter"])
	}
}

func writeMobileScriptedReadiness(t *testing.T, dir string) {
	t.Helper()
	path := filepath.Join(dir, ".autopus", "qa", "mobile", "readiness.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	body := `device_inventory:
  devices:
    - device_ref: device-ref:android-pixel-7
      platform: android
simulator_emulator:
  targets:
    - target_ref: emulator-ref:android-34
      platform: android
app_artifact:
  path: .autopus/qa/mobile/apps/app.apk
  digest: sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
credentials:
  refs:
    - credential-ref:android-dev
`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}

func writeMobileScriptedJourneyFile(t *testing.T, dir string) {
	t.Helper()
	flow := filepath.Join(dir, ".autopus", "qa", "mobile", "flows", "smoke.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(flow), 0o755))
	require.NoError(t, os.WriteFile(flow, []byte("appId: example\n---\n"), 0o644))
	journeyDir := filepath.Join(dir, ".autopus", "qa", "journeys")
	require.NoError(t, os.MkdirAll(journeyDir, 0o755))
	body := `id: mobile-scripted-smoke
title: Mobile scripted smoke
surface: mobile
lanes: [mobile-scripted]
adapter:
  id: maestro-scripted
command:
  argv: ["maestro", "test", ".autopus/qa/mobile/flows/smoke.yaml"]
  cwd: .
  timeout: 120s
checks:
  - id: mobile-scripted-smoke
    type: deterministic
    expected:
      exit_code: 0
mobile:
  flow_path: .autopus/qa/mobile/flows/smoke.yaml
  device_target: device-ref:android-pixel-7
  app_artifact_digest: sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa
`
	require.NoError(t, os.WriteFile(filepath.Join(journeyDir, "mobile-scripted-smoke.yaml"), []byte(body), 0o644))
}

func writeMobileDeviceMap(t *testing.T, dir, deviceRef, handle string) {
	t.Helper()
	path := filepath.Join(dir, ".autopus", "qa", "mobile", "devices.local.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	body := `{"` + deviceRef + `":"` + handle + `"}`
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
}

func installFakeBinaryOnPath(t *testing.T, name, script string) {
	t.Helper()
	bin := filepath.Join(t.TempDir(), "bin")
	require.NoError(t, os.MkdirAll(bin, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(bin, name), []byte("#!/bin/sh\n"+script+"\n"), 0o755))
	t.Setenv("PATH", bin+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func stringSliceOfMaps(value any) []map[string]any {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(raw))
	for _, entry := range raw {
		if m, ok := entry.(map[string]any); ok {
			out = append(out, m)
		}
	}
	return out
}
