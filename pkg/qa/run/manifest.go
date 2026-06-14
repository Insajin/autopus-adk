package run

import (
	"path/filepath"
	"strings"
	"time"

	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

const (
	guiArtifactPublicationCheckID   = "gui-artifact-redaction"
	guiArtifactPublicationCheckType = "gui_artifact_redaction"
)

func buildManifest(opts Options, pack journey.Pack, result commandResult, checks []IndexCheck) qaevidence.Manifest {
	artifacts := defaultArtifactsForPack(pack, result)
	if !blocksGUIArtifactPublication(checks) {
		artifacts = append(artifacts, declaredArtifacts(opts.ProjectDir, pack)...)
	}
	refs := sourceRefs(pack)
	// @AX:NOTE: [AUTO] magic constants — fallback SPEC and AC refs for the mobile-scripted lane when the pack omits source_refs
	if strings.EqualFold(opts.Lane, laneMobileScripted) && strings.TrimSpace(pack.SourceRefs.SourceSpec) == "" {
		refs.SourceSpec = "SPEC-QAMESH-008"
		if len(pack.SourceRefs.AcceptanceRefs) == 0 {
			refs.AcceptanceRefs = []string{"AC-QAMESH8-007"}
		}
	}
	return qaevidence.Manifest{
		SchemaVersion:       qaevidence.SchemaVersionV2,
		QAResultID:          pack.ID + "-" + strings.ReplaceAll(result.StartedAt.Format("20060102150405.000000000"), ".", ""),
		Surface:             surfaceForPack(pack),
		Lane:                opts.Lane,
		ScenarioRef:         pack.ID,
		Runner:              qaevidence.Runner{Name: pack.Adapter.ID, Command: result.Command},
		Status:              result.Status,
		StartedAt:           result.StartedAt.Format(time.RFC3339Nano),
		EndedAt:             result.EndedAt.Format(time.RFC3339Nano),
		DurationMS:          result.DurationMS,
		Artifacts:           artifacts,
		OracleResults:       qaevidence.OracleResults{Checks: manifestChecks(pack, checks, artifacts)},
		RedactionStatus:     qaevidence.RedactionStatus{Status: "passed"},
		SourceRefs:          refs,
		RetentionClass:      "local-redacted",
		ReproductionCommand: result.Command,
	}
}

func surfaceForPack(pack journey.Pack) string {
	switch strings.ToLower(strings.TrimSpace(pack.Surface)) {
	case "desktop":
		return "desktop"
	case "frontend", "web", "browser":
		return "frontend"
	case "backend", "package", "custom", "multi", "cli":
		return strings.ToLower(strings.TrimSpace(pack.Surface))
	default:
		return surfaceForAdapter(pack.Adapter.ID)
	}
}

func declaredArtifacts(projectDir string, pack journey.Pack) []qaevidence.ArtifactRef {
	refs := make([]qaevidence.ArtifactRef, 0, len(pack.Artifacts))
	for _, artifact := range pack.Artifacts {
		if strings.TrimSpace(artifact.Path) == "" {
			continue
		}
		kind := strings.TrimSpace(artifact.Kind)
		if kind == "" {
			kind = "artifact"
		}
		path := artifact.Path
		if !filepath.IsAbs(path) {
			path = filepath.Join(projectDir, path)
		}
		publishable := !strings.Contains(strings.ToLower(kind), "quarantine")
		redaction := "text_redacted_and_scanned"
		if !publishable {
			redaction = "local_only_quarantine_ref"
		}
		refs = append(refs, qaevidence.ArtifactRef{Kind: kind, Path: path, Publishable: publishable, Redaction: redaction})
	}
	return refs
}

func artifactKinds(artifacts []qaevidence.ArtifactRef) []string {
	kinds := make([]string, 0, len(artifacts))
	for _, artifact := range artifacts {
		kinds = append(kinds, artifact.Kind)
	}
	return kinds
}

func manifestChecks(pack journey.Pack, checks []IndexCheck, artifacts []qaevidence.ArtifactRef) []qaevidence.CheckResult {
	out := make([]qaevidence.CheckResult, 0, len(checks))
	artifactRefs := artifactKinds(artifacts)
	for _, check := range checks {
		out = append(out, qaevidence.CheckResult{
			ID:             check.ID,
			Type:           manifestCheckType(pack, check),
			Status:         check.Status,
			Expected:       check.Expected,
			Actual:         check.Actual,
			ArtifactRefs:   artifactRefs,
			FailureSummary: check.FailureSummary,
		})
	}
	return out
}

func manifestCheckType(pack journey.Pack, check IndexCheck) string {
	if check.ID == guiPolicyRuntimeCheckID {
		return guiPolicyRuntimeCheckType
	}
	if check.ID == guiArtifactPublicationCheckID {
		return guiArtifactPublicationCheckType
	}
	return firstCheckType(pack)
}

func firstCheckID(pack journey.Pack) string {
	if len(pack.Checks) > 0 && pack.Checks[0].ID != "" {
		return pack.Checks[0].ID
	}
	return pack.ID
}

func firstCheckType(pack journey.Pack) string {
	if len(pack.Checks) > 0 && pack.Checks[0].Type != "" {
		return pack.Checks[0].Type
	}
	if pack.Adapter.ID == "gui-explore" {
		return "gui_exploration"
	}
	return "unit_test"
}

func sourceRefs(pack journey.Pack) qaevidence.SourceRefs {
	refs := qaevidence.SourceRefs{
		SourceSpec:       pack.SourceRefs.SourceSpec,
		AcceptanceRefs:   pack.SourceRefs.AcceptanceRefs,
		OwnedPaths:       pack.SourceRefs.OwnedPaths,
		DoNotModifyPaths: pack.SourceRefs.DoNotModifyPaths,
		JourneyID:        pack.ID,
		StepID:           "step-1",
		Adapter:          pack.Adapter.ID,
		OracleThresholds: firstExpected(pack),
	}
	if refs.SourceSpec == "" && pack.Adapter.ID == "gui-explore" {
		refs.SourceSpec = "SPEC-QAMESH-003"
	}
	if refs.SourceSpec == "" && mobileAdapter(pack.Adapter.ID) {
		refs.SourceSpec = "SPEC-QAMESH-006"
	}
	if refs.SourceSpec == "" {
		refs.SourceSpec = "SPEC-QAMESH-002"
	}
	if len(refs.AcceptanceRefs) == 0 && pack.Adapter.ID == "gui-explore" {
		refs.AcceptanceRefs = []string{"AC-QAMESH3-004", "AC-QAMESH3-006"}
	}
	if refs.Mobile == nil && mobileAdapter(pack.Adapter.ID) {
		refs.Mobile = &qaevidence.MobileRefs{
			FlowID:            pack.ID,
			AppArtifactDigest: pack.Mobile.AppArtifactDigest,
			DeviceRef:         pack.Mobile.DeviceTarget,
		}
	}
	if len(refs.AcceptanceRefs) == 0 && mobileAdapter(pack.Adapter.ID) {
		refs.AcceptanceRefs = []string{"AC-QAMESH6-008"}
	}
	if len(refs.AcceptanceRefs) == 0 {
		refs.AcceptanceRefs = []string{"AC-QAMESH2-005"}
	}
	if len(refs.OwnedPaths) == 0 {
		refs.OwnedPaths = []string{"."}
	}
	if len(refs.DoNotModifyPaths) == 0 {
		refs.DoNotModifyPaths = []string{".codex/**", ".opencode/**", ".autopus/plugins/**"}
	}
	return refs
}

// @AX:ANCHOR: [AUTO] adapter identity gate — called from runner.go, manifest.go (x3), mobile_manifest.go; controls lane routing and evidence provenance
// @AX:REASON: Adding a mobile adapter ID here is the single required change to onboard a new mobile surface; omitting it silently skips mobile evidence attribution
func mobileAdapter(adapterID string) bool {
	return adapterID == "maestro-scripted" || adapterID == "appium-mobile-explore"
}
