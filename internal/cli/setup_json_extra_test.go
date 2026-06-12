package cli

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/setup"
)

// TestBuildSetupStatusPayload_SortsFiles verifies file ordering and field mapping.
func TestBuildSetupStatusPayload_SortsFiles(t *testing.T) {
	t.Parallel()

	mod := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	status := &setup.Status{
		Exists:      true,
		GeneratedAt: mod,
		DriftScore:  0.25,
		FileStatuses: map[string]setup.FileStatus{
			"zeta.md":  {Exists: true, Fresh: false, ModTime: mod},
			"alpha.md": {Exists: true, Fresh: true, ModTime: mod},
		},
		ProjectContext: setup.ProjectContextStatus{
			Exists:       true,
			Dir:          "/p/.autopus/project",
			Files:        []string{"product.md"},
			MissingFiles: []string{"architecture.md"},
		},
	}

	payload := buildSetupStatusPayload("/p", "", status)
	assert.Equal(t, "/p", payload.ProjectDir)
	assert.Equal(t, "/p/.autopus/docs", payload.DocsDir)
	assert.True(t, payload.Exists)
	assert.Equal(t, 0.25, payload.DriftScore)
	require.Len(t, payload.Files, 2)
	// Files must be sorted alphabetically.
	assert.Equal(t, "alpha.md", payload.Files[0].Name)
	assert.Equal(t, "zeta.md", payload.Files[1].Name)
	assert.True(t, payload.Files[0].Fresh)
	assert.False(t, payload.Files[1].Fresh)
	assert.True(t, payload.ProjectContextExists)
	assert.Equal(t, []string{"product.md"}, payload.ProjectContextFiles)
	assert.Equal(t, []string{"architecture.md"}, payload.MissingProjectContextFiles)
}

// TestBuildSetupStatusPayload_ExplicitDocsDir honors an explicit output dir.
func TestBuildSetupStatusPayload_ExplicitDocsDir(t *testing.T) {
	t.Parallel()

	status := &setup.Status{FileStatuses: map[string]setup.FileStatus{}}
	payload := buildSetupStatusPayload("/p", "/custom/docs", status)
	assert.Equal(t, "/custom/docs", payload.DocsDir)
	assert.Empty(t, payload.Files)
}

// TestBuildSetupStatusWarnings_MissingBundleWithContext emits the bundle warning.
func TestBuildSetupStatusWarnings_MissingBundleWithContext(t *testing.T) {
	t.Parallel()

	status := &setup.Status{
		Exists: false,
		ProjectContext: setup.ProjectContextStatus{
			Exists:       true,
			MissingFiles: []string{"glossary.md"},
		},
	}
	warnings := buildSetupStatusWarnings(status)
	codes := warningCodes(warnings)
	assert.Contains(t, codes, "docs_bundle_not_found")
	assert.Contains(t, codes, "missing_project_context")
}

// TestBuildSetupStatusWarnings_NoDocsNoContext emits docs_not_found.
func TestBuildSetupStatusWarnings_NoDocsNoContext(t *testing.T) {
	t.Parallel()

	status := &setup.Status{Exists: false}
	warnings := buildSetupStatusWarnings(status)
	require.Len(t, warnings, 1)
	assert.Equal(t, "docs_not_found", warnings[0].Code)
}

// TestBuildSetupStatusWarnings_ExistsNoWarnings returns empty when everything is present.
func TestBuildSetupStatusWarnings_ExistsNoWarnings(t *testing.T) {
	t.Parallel()

	status := &setup.Status{Exists: true}
	warnings := buildSetupStatusWarnings(status)
	assert.Empty(t, warnings)
}

// TestBuildSetupValidationPayload_MapsWarnings copies validation warnings.
func TestBuildSetupValidationPayload_MapsWarnings(t *testing.T) {
	t.Parallel()

	report := &setup.ValidationReport{
		Valid:      false,
		DriftScore: 0.5,
		Warnings: []setup.ValidationWarning{
			{File: "a.md", Line: 12, Type: "stale_path", Message: "broken path"},
			{File: "b.md", Type: "line_limit", Message: "too long"},
		},
	}
	payload := buildSetupValidationPayload("/proj", "/proj/docs", report)
	assert.Equal(t, "/proj", payload.ProjectDir)
	assert.Equal(t, "/proj/docs", payload.DocsDir)
	assert.False(t, payload.Valid)
	assert.Equal(t, 0.5, payload.DriftScore)
	assert.Equal(t, 2, payload.WarningCount)
	require.Len(t, payload.Warnings, 2)
	assert.Equal(t, 12, payload.Warnings[0].Line)
	assert.Equal(t, "stale_path", payload.Warnings[0].Type)
	assert.Equal(t, "broken path", payload.Warnings[0].Message)
}

// TestBuildSetupValidationWarnings_Empty returns nil when valid.
func TestBuildSetupValidationWarnings_Empty(t *testing.T) {
	t.Parallel()

	assert.Nil(t, buildSetupValidationWarnings(&setup.ValidationReport{}))
}

// TestBuildSetupValidationWarnings_MapsTypeAsCode maps warning type into code.
func TestBuildSetupValidationWarnings_MapsTypeAsCode(t *testing.T) {
	t.Parallel()

	report := &setup.ValidationReport{
		Warnings: []setup.ValidationWarning{
			{Type: "stale_command", Message: "cmd gone"},
		},
	}
	warnings := buildSetupValidationWarnings(report)
	require.Len(t, warnings, 1)
	assert.Equal(t, "stale_command", warnings[0].Code)
	assert.Equal(t, "cmd gone", warnings[0].Message)
}

func warningCodes(warnings []jsonMessage) []string {
	out := make([]string, 0, len(warnings))
	for _, w := range warnings {
		out = append(out, w.Code)
	}
	return out
}
