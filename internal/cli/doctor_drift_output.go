package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/insajin/autopus-adk/pkg/config"
)

// Platform-neutral remediation hints. They are valid for claude-code, codex,
// antigravity-cli, and opencode users alike (REQ-009): none names a
// platform-specific command or path.
const (
	driftHintUpdate   = "run 'auto update' to regenerate"
	driftHintRemove   = "remove stale manifest(s) with 'rm'"
	driftHintRegen    = "run 'generate-templates' to regenerate"
	driftHintRebuild  = "rebuild with 'go build' to refresh the binary commit"
	driftReprPathsCap = maxDriftReprPaths
)

// collectDriftGateChecks mirrors the drift observers into the JSON report. Every
// drift check is advisory: warn-status checks are appended but r.status is never
// touched, so overall_ok stays true (REQ-007, the collectContextWeightChecks
// precedent). Checks are skipped entirely when their subject is absent (REQ-008).
func (r *doctorJSONReport) collectDriftGateChecks(dir string, cfg *config.HarnessConfig) {
	for _, res := range collectContentDrift(dir, cfg) {
		r.checks = append(r.checks, contentDriftCheck(res))
	}

	if orphans := detectOrphanManifests(dir, cfg); orphans.Present {
		r.checks = append(r.checks, orphanManifestCheck(orphans))
	}

	source := collectSourceDrift(dir)
	if source.RegenChecked {
		r.checks = append(r.checks, templateRegenCheck(source))
	}
	if source.BinaryChecked {
		r.checks = append(r.checks, binaryStaleCheck(source))
	}
}

func contentDriftCheck(res contentDriftResult) jsonCheck {
	id := "doctor.drift.content." + res.Platform
	if res.DriftCount == 0 {
		return jsonCheck{
			ID:       id,
			Severity: "info",
			Status:   "pass",
			Detail: fmt.Sprintf("content drift: none observed (%d deterministic file(s) checked)",
				res.Compared),
		}
	}
	return jsonCheck{
		ID:       id,
		Severity: "warning",
		Status:   "warn",
		Detail: fmt.Sprintf("content drift: %d file(s) differ from generated output (%s); %s",
			res.DriftCount, driftReprPaths(res.DriftPaths), driftHintUpdate),
	}
}

func orphanManifestCheck(res orphanManifestResult) jsonCheck {
	if len(res.Paths) == 0 {
		return jsonCheck{
			ID:       "doctor.drift.orphan_manifest",
			Severity: "info",
			Status:   "pass",
			Detail:   "orphan manifest: none observed",
		}
	}
	return jsonCheck{
		ID:       "doctor.drift.orphan_manifest",
		Severity: "warning",
		Status:   "warn",
		Detail: fmt.Sprintf("orphan manifest: %d not in configured platforms (%s)%s; %s",
			len(res.Paths), strings.Join(res.Paths, ", "), orphanAliasSuffix(res), driftHintRemove),
	}
}

// orphanAliasSuffix names the successor for known legacy aliases so operators
// understand why a superseded manifest lingers.
func orphanAliasSuffix(res orphanManifestResult) string {
	if len(res.Aliases) == 0 {
		return ""
	}
	var parts []string
	for token, successor := range res.Aliases {
		parts = append(parts, fmt.Sprintf("%s superseded by %s", token, successor))
	}
	return " [" + strings.Join(parts, "; ") + "]"
}

func templateRegenCheck(res sourceDriftReport) jsonCheck {
	if len(res.StaleTemplates) == 0 {
		return jsonCheck{
			ID:       "doctor.drift.template_regen",
			Severity: "info",
			Status:   "pass",
			Detail:   "template regeneration: none observed",
		}
	}
	return jsonCheck{
		ID:       "doctor.drift.template_regen",
		Severity: "warning",
		Status:   "warn",
		Detail: fmt.Sprintf("template regeneration drift: %d template(s) stale (%s); %s",
			len(res.StaleTemplates), driftReprPaths(res.StaleTemplates), driftHintRegen),
	}
}

func binaryStaleCheck(res sourceDriftReport) jsonCheck {
	if !res.BinaryStale {
		return jsonCheck{
			ID:       "doctor.drift.binary_stale",
			Severity: "info",
			Status:   "pass",
			Detail: fmt.Sprintf("binary commit %s matches HEAD %s",
				res.BuildCommit, res.HeadPrefix),
		}
	}
	return jsonCheck{
		ID:       "doctor.drift.binary_stale",
		Severity: "warning",
		Status:   "warn",
		Detail: fmt.Sprintf("binary stale: build commit %s is not a prefix of HEAD %s; %s",
			res.BuildCommit, res.HeadPrefix, driftHintRebuild),
	}
}

// driftReprPaths renders up to driftReprPathsCap representative paths, appending
// an overflow marker so the count stays honest without listing every file.
func driftReprPaths(paths []string) string {
	if len(paths) <= driftReprPathsCap {
		return strings.Join(paths, ", ")
	}
	return fmt.Sprintf("%s, ... and %d more",
		strings.Join(paths[:driftReprPathsCap], ", "), len(paths)-driftReprPathsCap)
}

// renderDriftText mirrors the JSON drift checks into the text doctor. It prints
// nothing when no drift subject is present, so end-user installs without
// manifests or a source repo see no Drift section. The section is advisory and
// its warnings never flip the doctor verdict.
func renderDriftText(out io.Writer, dir string, cfg *config.HarnessConfig) {
	content := collectContentDrift(dir, cfg)
	orphans := detectOrphanManifests(dir, cfg)
	source := collectSourceDrift(dir)

	if len(content) == 0 && !orphans.Present && !source.RegenChecked && !source.BinaryChecked {
		return
	}

	tui.SectionHeader(out, "Drift")

	for _, res := range content {
		if res.DriftCount == 0 {
			tui.OK(out, fmt.Sprintf("%s content: none observed (%d checked)", res.Platform, res.Compared))
			continue
		}
		tui.Warn(out, fmt.Sprintf("%s content: %d file(s) stale; %s", res.Platform, res.DriftCount, driftHintUpdate))
		for _, p := range limitDriftTextPaths(res.DriftPaths) {
			tui.Bullet(out, p)
		}
	}

	renderOrphanText(out, orphans)
	renderSourceDriftText(out, source)
}

func renderOrphanText(out io.Writer, orphans orphanManifestResult) {
	if !orphans.Present {
		return
	}
	if len(orphans.Paths) == 0 {
		tui.OK(out, "orphan manifest: none observed")
		return
	}
	tui.Warn(out, fmt.Sprintf("orphan manifest: %d not configured%s; %s",
		len(orphans.Paths), orphanAliasSuffix(orphans), driftHintRemove))
	for _, p := range limitDriftTextPaths(orphans.Paths) {
		tui.Bullet(out, p)
	}
}

func renderSourceDriftText(out io.Writer, source sourceDriftReport) {
	if source.RegenChecked {
		if len(source.StaleTemplates) == 0 {
			tui.OK(out, "template regeneration: none observed")
		} else {
			tui.Warn(out, fmt.Sprintf("template regeneration: %d stale; %s",
				len(source.StaleTemplates), driftHintRegen))
			for _, p := range limitDriftTextPaths(source.StaleTemplates) {
				tui.Bullet(out, p)
			}
		}
	}
	if source.BinaryChecked {
		if source.BinaryStale {
			tui.Warn(out, fmt.Sprintf("binary stale: commit %s not a prefix of HEAD %s; %s",
				source.BuildCommit, source.HeadPrefix, driftHintRebuild))
		} else {
			tui.OK(out, fmt.Sprintf("binary commit %s matches HEAD %s", source.BuildCommit, source.HeadPrefix))
		}
	}
}

func limitDriftTextPaths(paths []string) []string {
	if len(paths) <= driftReprPathsCap {
		return paths
	}
	return paths[:driftReprPathsCap]
}
