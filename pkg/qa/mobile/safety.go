package mobile

import (
	"path/filepath"
	"regexp"
	"strings"

	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
)

const redactedDevice = "[REDACTED_DEVICE]"

var (
	opaqueCredentialRefRe = regexp.MustCompile(`^(credential-ref|secret-ref):[A-Za-z0-9._-]+$`)
)

func sanitizeTargets(targets []DeviceTarget, readiness *Readiness) []DeviceTarget {
	out := make([]DeviceTarget, 0, len(targets))
	for _, target := range targets {
		target.DeviceRef = sanitizeDeviceRef(target.DeviceRef, readiness)
		target.TargetRef = sanitizeDeviceRef(target.TargetRef, readiness)
		out = append(out, target)
	}
	return out
}

func sanitizePlanningPath(path string, readiness *Readiness) string {
	path = strings.TrimSpace(path)
	if path == "" {
		return ""
	}
	findings := qaevidence.FindUnsafeText(path, "mobile_readiness.app_artifact.path")
	if filepath.IsAbs(path) || !safeMobileRelPath(path) {
		findings = append(findings, Finding{
			Type:   "local_user_path",
			Source: "mobile_readiness.app_artifact.path",
			Sample: "[REDACTED_LOCAL_PATH]",
		})
	}
	if len(findings) > 0 {
		block(readiness, findings...)
		return "[REDACTED_LOCAL_PATH]"
	}
	return filepath.ToSlash(filepath.Clean(path))
}

func sanitizeCredentialRef(ref string, readiness *Readiness) string {
	ref = strings.TrimSpace(ref)
	if opaqueCredentialRefRe.MatchString(ref) {
		return ref
	}
	findings := qaevidence.FindUnsafeText(ref, "mobile_readiness.credentials.refs")
	findings = append(findings, Finding{
		Type:   "credential_ref",
		Source: "mobile_readiness.credentials.refs",
		Sample: qaevidence.RedactedSecret,
	})
	block(readiness, findings...)
	return qaevidence.RedactedSecret
}

func sanitizeDeviceRef(ref string, readiness *Readiness) string {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return ""
	}
	if !qaevidence.IsOpaqueMobileRef(ref) {
		block(readiness, Finding{
			Type:   "device_identifier",
			Source: "mobile_readiness.device_ref",
			Sample: redactedDevice,
		})
		return redactedDevice
	}
	return ref
}

func safeMobileRelPath(path string) bool {
	if filepath.IsAbs(path) {
		return false
	}
	clean := filepath.ToSlash(filepath.Clean(path))
	return clean == ".autopus/qa/mobile" || strings.HasPrefix(clean, ".autopus/qa/mobile/")
}

func block(readiness *Readiness, findings ...Finding) {
	readiness.RedactionStatus.Status = "blocked"
	readiness.RedactionStatus.Findings = append(readiness.RedactionStatus.Findings, findings...)
}
