package run

import (
	"os"
	"path/filepath"
	"testing"

	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildManifestMobileScriptedEmitsV2MobileArtifacts(t *testing.T) {
	dir := t.TempDir()
	result := mobileCommandResult(t, dir)
	deviceMeta := writeMobileArtifactJSON(t, dir, "device-metadata.json", `{"platform":"android","device_ref":"device-ref:android-pixel-7","target_ref":"target-ref:android-34"}`)
	screenshot := writeMobileArtifactJSON(t, dir, "screenshot-quarantine-ref.json", `{"ref":"screenshot-quarantine-ref:mobile-scripted-smoke","local_only":true}`)
	video := writeMobileArtifactJSON(t, dir, "video-quarantine-ref.json", `{"ref":"video-quarantine-ref:mobile-scripted-smoke","local_only":true}`)
	packCopy := mobileScriptedJourneyPack(".autopus/qa/mobile/flows/smoke.yaml")
	packCopy.Artifacts = []journey.Artifact{
		{Kind: "device_metadata", Path: deviceMeta},
		{Kind: "screenshot_quarantine_ref", Path: screenshot},
		{Kind: "video_quarantine_ref", Path: video},
	}
	check := IndexCheck{ID: "mobile-scripted-smoke", JourneyID: packCopy.ID, Adapter: packCopy.Adapter.ID, Status: "passed", Expected: "exit_code=0", Actual: "exit_code=0"}

	manifest := buildManifest(Options{ProjectDir: dir, Lane: "mobile-scripted"}, packCopy, result, []IndexCheck{check})
	manifestPath, err := qaevidence.WriteFinalManifest(manifest, filepath.Join(dir, "final"))
	require.NoError(t, err)

	loaded, err := qaevidence.LoadManifest(manifestPath)
	require.NoError(t, err)
	assert.Equal(t, qaevidence.SchemaVersionV2, loaded.SchemaVersion)
	assert.Equal(t, "mobile", loaded.Surface)
	allowed := map[string]bool{"sanitized_log": true, "device_metadata": true, "app_artifact_digest": true, "screenshot_quarantine_ref": true, "video_quarantine_ref": true}
	require.NotEmpty(t, loaded.Artifacts)
	for _, artifact := range loaded.Artifacts {
		assert.Truef(t, allowed[artifact.Kind], "unexpected mobile artifact kind %q", artifact.Kind)
	}
}

func TestWriteFinalManifestRejectsRawMobileMedia(t *testing.T) {
	dir := t.TempDir()
	result := mobileCommandResult(t, dir)
	pack := mobileScriptedJourneyPack(".autopus/qa/mobile/flows/smoke.yaml")
	check := IndexCheck{ID: "mobile-scripted-smoke", JourneyID: pack.ID, Adapter: pack.Adapter.ID, Status: "passed", Expected: "exit_code=0", Actual: "exit_code=0"}
	manifest := buildManifest(Options{ProjectDir: dir, Lane: "mobile-scripted"}, pack, result, []IndexCheck{check})
	pngPath := filepath.Join(dir, "screenshot.png")
	manifest.Artifacts = []qaevidence.ArtifactRef{{Kind: "sanitized_log", Path: pngPath, Publishable: true, Redaction: "text_redacted_and_scanned"}}

	outputDir := filepath.Join(dir, "final")
	_, err := qaevidence.WriteFinalManifest(manifest, outputDir)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "unsafe_mobile_artifact")
	assert.NoFileExists(t, filepath.Join(outputDir, "manifest.json"))
}

func writeMobileArtifactJSON(t *testing.T, dir, name, body string) string {
	t.Helper()
	path := filepath.Join(dir, "artifacts", name)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(body), 0o644))
	return path
}
