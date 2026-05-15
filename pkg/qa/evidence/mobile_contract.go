package evidence

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	mobileDeviceIDRe = regexp.MustCompile(`(?i)\b(?:udid|serial|imei|device[_ -]?id)?[:=]?\s*[A-F0-9]{15,40}\b`)
	mobileDigestRe   = regexp.MustCompile(`^sha256:[a-fA-F0-9]{64}$`)
	mobileRefRe      = regexp.MustCompile(`^(device-ref|target-ref|simulator-ref|emulator-ref|cloud-device-ref):[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)
	mobileURLRe      = regexp.MustCompile(`(?i)\bhttps?://`)
	signedURLQueryRe = regexp.MustCompile(`(?i)[?&](sig|signature|x-amz-signature|x-goog-signature|x-ms-signature|x-ms-sig)=`)
)

func IsOpaqueMobileRef(ref string) bool {
	ref = strings.TrimSpace(ref)
	if !mobileRefRe.MatchString(ref) {
		return false
	}
	parts := strings.SplitN(ref, ":", 2)
	if len(parts) != 2 {
		return false
	}
	suffix := strings.ToLower(parts[1])
	for _, marker := range []string{"iphone", "ipad", "imei", "serial", "udid"} {
		if strings.Contains(suffix, marker) {
			return false
		}
	}
	return !mobileDeviceIDRe.MatchString(parts[1])
}

func validateMobileContract(manifest Manifest) error {
	if manifest.SourceRefs.Adapter != "maestro-scripted" &&
		manifest.SourceRefs.Adapter != "appium-mobile-explore" {
		return fmt.Errorf("mobile evidence source_refs.adapter must be maestro-scripted or appium-mobile-explore")
	}
	mobile := manifest.SourceRefs.Mobile
	if mobile == nil || strings.TrimSpace(mobile.FlowID) == "" ||
		strings.TrimSpace(mobile.AppArtifactDigest) == "" ||
		strings.TrimSpace(mobile.DeviceRef) == "" {
		return fmt.Errorf("mobile evidence requires flow_id, app_artifact_digest, and device_ref")
	}
	if !mobileDigestRe.MatchString(strings.TrimSpace(mobile.AppArtifactDigest)) {
		return fmt.Errorf("mobile evidence source_refs.mobile.app_artifact_digest must be sha256")
	}
	if !IsOpaqueMobileRef(mobile.DeviceRef) || mobileDeviceIDRe.MatchString(mobile.DeviceRef) {
		return fmt.Errorf("unsafe_mobile_artifact: source_refs.mobile.device_ref must be an opaque mobile ref")
	}
	for _, artifact := range manifest.Artifacts {
		if !allowedMobileArtifactKind(artifact.Kind) {
			return fmt.Errorf("mobile evidence unsupported artifact kind %s", artifact.Kind)
		}
		if err := validateMobileArtifactRef(artifact); err != nil {
			return err
		}
	}
	return nil
}

// @AX:NOTE [AUTO] [downgraded from ANCHOR - fan_in < 3] @AX:SPEC: SPEC-QAMESH-006: mobile evidence publication is gated on sanitized refs and redaction-safe artifact contents.
// @AX:REASON: WriteFinalManifest calls this boundary before publishable evidence leaves the run output, so raw media, local paths, and device identifiers must remain blocked.
func validateMobilePublication(manifest Manifest) error {
	if manifest.Surface != "mobile" {
		return nil
	}
	for _, artifact := range manifest.Artifacts {
		if rawMobileMedia(artifact) {
			return fmt.Errorf("unsafe_mobile_artifact: raw mobile media artifact %s", RedactText(artifact.Path))
		}
		if len(FindUnsafeText(artifact.Path, artifact.Kind)) > 0 ||
			mobileDeviceIDRe.MatchString(artifact.Path) ||
			containsUnsafeMobileURL(artifact.Path) {
			return fmt.Errorf("unsafe_mobile_artifact: unsafe mobile artifact path %s", RedactText(artifact.Path))
		}
		body, err := os.ReadFile(artifact.Path)
		if err != nil {
			continue
		}
		redacted := RedactText(string(body))
		if err := AssertSafeText(redacted, artifact.Path); err != nil {
			return fmt.Errorf("unsafe_mobile_artifact: %w", err)
		}
		if mobileDeviceIDRe.MatchString(string(body)) {
			return fmt.Errorf("unsafe_mobile_artifact: raw mobile device identifier in %s", RedactText(artifact.Path))
		}
		if mobileQuarantineArtifact(artifact) && containsUnsafeMobileURL(string(body)) {
			return fmt.Errorf("unsafe_mobile_artifact: mobile quarantine refs must be opaque local refs")
		}
	}
	return nil
}

func validateMobileArtifactRef(artifact ArtifactRef) error {
	if !mobileQuarantineArtifact(artifact) {
		return nil
	}
	if artifact.Publishable || artifact.Redaction != "local_only_quarantine_ref" {
		return fmt.Errorf("unsafe_mobile_artifact: mobile quarantine refs must be non-publishable local-only refs")
	}
	return nil
}

func allowedMobileArtifactKind(kind string) bool {
	switch strings.TrimSpace(kind) {
	case "sanitized_log", "device_metadata", "app_artifact_digest", "screenshot_quarantine_ref", "video_quarantine_ref":
		return true
	default:
		return false
	}
}

func rawMobileMedia(artifact ArtifactRef) bool {
	kind := strings.ToLower(strings.TrimSpace(artifact.Kind))
	if strings.Contains(kind, "quarantine_ref") || strings.Contains(kind, "digest") || strings.Contains(kind, "metadata") {
		return false
	}
	switch strings.ToLower(filepath.Ext(artifact.Path)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".mp4", ".mov", ".webm":
		return true
	default:
		return false
	}
}

func mobileQuarantineArtifact(artifact ArtifactRef) bool {
	return artifact.Kind == "screenshot_quarantine_ref" || artifact.Kind == "video_quarantine_ref"
}

func containsUnsafeMobileURL(value string) bool {
	return mobileURLRe.MatchString(value) || signedURLQueryRe.MatchString(value)
}
