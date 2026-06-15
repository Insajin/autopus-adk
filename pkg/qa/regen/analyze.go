package regen

import (
	"github.com/insajin/autopus-adk/pkg/e2e"
	qaevidence "github.com/insajin/autopus-adk/pkg/qa/evidence"
	qaproject "github.com/insajin/autopus-adk/pkg/qa/project"
)

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-011: surface tokens are ordered fixed constants — changing values or order breaks PresentSurfaces determinism contract (AC-001) and downstream pack synthesis.
// Surface tokens, appended in this fixed order for determinism (AC-001).
const (
	SurfaceWeb     = "web"
	SurfaceDesktop = "desktop"
	SurfaceMobile  = "mobile"
)

// CLIFlow is a redacted summary of a single extracted Cobra leaf command.
// Command and Description originate from project source and are treated as
// untrusted; both are passed through RedactText before populating this struct.
type CLIFlow struct {
	ID          string `json:"id"`
	Command     string `json:"command"`
	Description string `json:"description"`
}

// Analysis is the output of AnalyzeProject: present surfaces (fixed order) plus
// redacted CLI flows extracted from the project's Cobra command tree.
type Analysis struct {
	Surfaces []string  `json:"surfaces"`
	CLIFlows []CLIFlow `json:"cli_flows"`
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-QAMESH-011: PresentSurfaces is the cross-package surface detection entry point consumed by AnalyzeProject, BuildResult, and orchestrateWith.
// @AX:REASON: fan_in=5 (analyze.go, regen.go, orchestrate.go, analyze_test.go, diff_test.go); surface list drives all downstream pack synthesis, dispatch, and verdict.
// PresentSurfaces returns the present surfaces in fixed order web, desktop,
// mobile. A surface is absent when its signal detector returns false.
func PresentSurfaces(projectDir string) []string {
	surfaces := make([]string, 0, 3)
	if qaproject.HasBrowserSignals(projectDir) {
		surfaces = append(surfaces, SurfaceWeb)
	}
	if qaproject.HasDesktopGUISignals(projectDir) {
		surfaces = append(surfaces, SurfaceDesktop)
	}
	if qaproject.HasAndroidSignals(projectDir) || qaproject.HasIOSSignals(projectDir) {
		surfaces = append(surfaces, SurfaceMobile)
	}
	return surfaces
}

// AnalyzeProject determines present surfaces and extracts CLI flows. Every
// extracted Command/Description is redacted before it enters the returned
// struct so untrusted project text cannot leak secrets or local paths into any
// downstream pack or diff. A flow extraction error is non-fatal: surfaces are
// still returned and CLIFlows is left empty.
func AnalyzeProject(projectDir string) (Analysis, error) {
	analysis := Analysis{Surfaces: PresentSurfaces(projectDir)}
	scenarios, err := e2e.ExtractCobra(projectDir)
	if err != nil {
		return analysis, nil
	}
	for _, s := range scenarios {
		analysis.CLIFlows = append(analysis.CLIFlows, CLIFlow{
			ID:          qaevidence.RedactText(s.ID),
			Command:     qaevidence.RedactText(s.Command),
			Description: qaevidence.RedactText(s.Description),
		})
	}
	return analysis, nil
}
