package run

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

const redactedDeviceHandle = "[REDACTED_DEVICE_HANDLE]"

// redactMobileHandle scrubs the concrete runtime device handle from every
// published surface: the stdout/stderr log files (emitted as sanitized_log
// artifacts) and the result/check text that flows into the manifest. The handle
// is resolved from an opaque device_ref and must never reach published evidence,
// but the flow tooling (maestro/adb/simctl) echoes the MAESTRO_DEVICE/-s target
// into stdout. Pattern matching alone is bypassable, so we redact by the known
// handle value. No-op when the handle is empty or too short to redact safely.
func redactMobileHandle(result *commandResult, check *IndexCheck, handle string) {
	handle = strings.TrimSpace(handle)
	if len(handle) < 2 {
		return
	}
	// Scrub the actual log files the runner reported (runner-agnostic), not
	// hardcoded names, so a custom MobileDeviceRunner cannot bypass redaction.
	for _, path := range []string{result.StdoutPath, result.StderrPath} {
		if path == "" {
			continue
		}
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		if scrubbed := strings.ReplaceAll(string(raw), handle, redactedDeviceHandle); scrubbed != string(raw) {
			_ = os.WriteFile(path, []byte(scrubbed), 0o644)
		}
	}
	result.StdoutText = strings.ReplaceAll(result.StdoutText, handle, redactedDeviceHandle)
	result.FailureSummary = strings.ReplaceAll(result.FailureSummary, handle, redactedDeviceHandle)
	result.Command = strings.ReplaceAll(result.Command, handle, redactedDeviceHandle)
	if check != nil {
		check.Actual = strings.ReplaceAll(check.Actual, handle, redactedDeviceHandle)
		check.FailureSummary = strings.ReplaceAll(check.FailureSummary, handle, redactedDeviceHandle)
	}
}

// mobileDeviceMetadata is the publishable device summary. It carries only opaque
// refs; the concrete runtime handle is intentionally excluded.
// @AX:NOTE: [AUTO] handle redaction boundary — mobileDeviceMetadata must never include the concrete Handle field from mobileDeviceContext
type mobileDeviceMetadata struct {
	Platform  string `json:"platform"`
	DeviceRef string `json:"device_ref"`
	TargetRef string `json:"target_ref"`
}

type mobileQuarantineRef struct {
	Ref       string `json:"ref"`
	LocalOnly bool   `json:"local_only"`
}

// writeMobileArtifacts writes the device metadata and the screenshot/video
// quarantine refs into the artifact dir, then returns a COPY of the pack with
// those artifacts appended so buildManifest -> declaredArtifacts classifies
// them. The input pack is never mutated.
func writeMobileArtifacts(artifactDir string, pack journey.Pack, devCtx mobileDeviceContext) journey.Pack {
	metaPath := filepath.Join(artifactDir, "device-metadata.json")
	writeMobileJSON(metaPath, mobileDeviceMetadata{
		Platform:  devCtx.Platform,
		DeviceRef: devCtx.DeviceRef,
		TargetRef: devCtx.TargetRef,
	})
	screenshotPath := filepath.Join(artifactDir, "screenshot-quarantine-ref.json")
	writeMobileJSON(screenshotPath, mobileQuarantineRef{
		Ref:       "screenshot-quarantine-ref:" + pack.ID,
		LocalOnly: true,
	})
	videoPath := filepath.Join(artifactDir, "video-quarantine-ref.json")
	writeMobileJSON(videoPath, mobileQuarantineRef{
		Ref:       "video-quarantine-ref:" + pack.ID,
		LocalOnly: true,
	})

	packCopy := pack
	artifacts := make([]journey.Artifact, 0, len(pack.Artifacts)+3)
	artifacts = append(artifacts, pack.Artifacts...)
	artifacts = append(artifacts,
		journey.Artifact{Kind: "device_metadata", Path: metaPath},
		journey.Artifact{Kind: "screenshot_quarantine_ref", Path: screenshotPath},
		journey.Artifact{Kind: "video_quarantine_ref", Path: videoPath},
	)
	packCopy.Artifacts = artifacts
	return packCopy
}

func writeMobileJSON(path string, payload any) {
	body, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, body, 0o644)
}
