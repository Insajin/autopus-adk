package run

import (
	"strings"
	"time"

	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

func buildManifest(opts Options, pack journey.Pack, result commandResult, check IndexCheck) qaevidence.Manifest {
	return qaevidence.Manifest{
		SchemaVersion: qaevidence.SchemaVersionV2,
		QAResultID:    pack.ID + "-" + strings.ReplaceAll(result.StartedAt.Format("20060102150405.000000000"), ".", ""),
		Surface:       surfaceForAdapter(pack.Adapter.ID),
		Lane:          opts.Lane,
		ScenarioRef:   pack.ID,
		Runner:        qaevidence.Runner{Name: pack.Adapter.ID, Command: result.Command},
		Status:        result.Status,
		StartedAt:     result.StartedAt.Format(time.RFC3339Nano),
		EndedAt:       result.EndedAt.Format(time.RFC3339Nano),
		DurationMS:    result.DurationMS,
		Artifacts: []qaevidence.ArtifactRef{
			{Kind: "stdout", Path: result.StdoutPath, Publishable: true, Redaction: "text_redacted_and_scanned"},
			{Kind: "stderr", Path: result.StderrPath, Publishable: true, Redaction: "text_redacted_and_scanned"},
		},
		OracleResults: qaevidence.OracleResults{Checks: []qaevidence.CheckResult{{
			ID:             check.ID,
			Type:           firstCheckType(pack),
			Status:         result.Status,
			Expected:       check.Expected,
			Actual:         check.Actual,
			ArtifactRefs:   []string{"stdout", "stderr"},
			FailureSummary: check.FailureSummary,
		}}},
		RedactionStatus:     qaevidence.RedactionStatus{Status: "passed"},
		SourceRefs:          sourceRefs(pack),
		RetentionClass:      "local-redacted",
		ReproductionCommand: result.Command,
	}
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
	if refs.SourceSpec == "" {
		refs.SourceSpec = "SPEC-QAMESH-002"
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
