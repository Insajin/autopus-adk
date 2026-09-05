package spec_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/spec"
)

const defaultProbeRow = "| A1 | implementation_assumption | high | composition-root boot | fixture config | boot succeeds | temp home | not-run | probe not executed yet | - |"

// probeSection renders a plan.md "## Risk-First Integration Probe" section
// with the given table rows.
func probeSection(rows ...string) string {
	var sb strings.Builder
	sb.WriteString("## Risk-First Integration Probe\n\n")
	sb.WriteString("| assumption_id | class | risk | boundary | input | oracle | isolation | status | reason | evidence |\n")
	sb.WriteString("|---|---|---|---|---|---|---|---|---|---|\n")
	for _, row := range rows {
		sb.WriteString(row)
		sb.WriteString("\n")
	}
	return sb.String()
}

func planWithProbeRows(t *testing.T, rows ...string) (string, *spec.SpecDocument) {
	t.Helper()

	plan := strings.Replace(validPlanMD(),
		probeSection(defaultProbeRow), probeSection(rows...), 1)
	require.NotEqual(t, validPlanMD(), plan, "probe section replacement must apply")
	return writeAuthoringPreflightSpec(t, map[string]string{"plan.md": plan})
}

func assertNoProbeError(t *testing.T, errs []spec.ValidationError) {
	t.Helper()

	for _, e := range errs {
		if e.Level == "error" && strings.Contains(e.Message, "Risk-First Integration Probe") {
			t.Fatalf("unexpected probe finding: %s", e.Message)
		}
	}
}

func TestValidateSpecSet_AcceptsProbeTableWithPassFailAndNotRunRows(t *testing.T) {
	t.Parallel()

	specDir, doc := planWithProbeRows(t,
		"| A1 | implementation_assumption | critical | browser -> BFF -> API | seeded session | 200 with scoped token | ephemeral stack | PASS | round trip observed | artifact://probe-a1 |",
		"| A2 | implementation_assumption | high | logout during in-flight upload | queued upload | upload cancelled, no orphan row | temp database | FAIL | upload survived logout | artifact://probe-a2 |",
		"| A3 | requirement_invariant | medium | account switch | two tenants | tenant scoping preserved | temp database | not-run | blocked on tenant fixture | - |",
	)

	assertNoProbeError(t, spec.ValidateSpecSet(specDir, doc))
}

func TestValidateSpecSet_RejectsProbePassWithoutEvidence(t *testing.T) {
	t.Parallel()

	specDir, doc := planWithProbeRows(t,
		"| A1 | implementation_assumption | critical | browser -> BFF -> API | seeded session | 200 with scoped token | ephemeral stack | PASS | round trip observed | - |",
	)

	errs := spec.ValidateSpecSet(specDir, doc)
	assertValidationError(t, errs, "plan.md", "A1 행이 PASS인데 evidence가 없습니다")
}

func TestValidateSpecSet_RejectsProbeRowMissingReason(t *testing.T) {
	t.Parallel()

	specDir, doc := planWithProbeRows(t,
		"| A1 | implementation_assumption | high | composition-root boot | restricted permissions | boot succeeds | temp home | not-run | | - |",
	)

	errs := spec.ValidateSpecSet(specDir, doc)
	assertValidationError(t, errs, "plan.md", "A1 행에 reason이 없습니다")
}

func TestValidateSpecSet_RejectsUnknownProbeStatus(t *testing.T) {
	t.Parallel()

	specDir, doc := planWithProbeRows(t,
		"| A1 | implementation_assumption | high | composition-root boot | restricted permissions | boot succeeds | temp home | passed | boot verified | artifact://probe-a1 |",
	)

	errs := spec.ValidateSpecSet(specDir, doc)
	assertValidationError(t, errs, "plan.md", "status가")
}

func TestValidateSpecSet_RejectsProbeTableOverRowCap(t *testing.T) {
	t.Parallel()

	rows := make([]string, 0, 4)
	for _, id := range []string{"A1", "A2", "A3", "A4"} {
		rows = append(rows, "| "+id+" | implementation_assumption | high | boundary | input | oracle | isolation | not-run | probe not executed yet | - |")
	}
	specDir, doc := planWithProbeRows(t, rows...)

	errs := spec.ValidateSpecSet(specDir, doc)
	assertValidationError(t, errs, "plan.md", "1-3개의 행이 필요합니다")
}

func TestValidateSpecSet_RejectsEmptyProbeTable(t *testing.T) {
	t.Parallel()

	specDir, doc := planWithProbeRows(t)

	errs := spec.ValidateSpecSet(specDir, doc)
	assertValidationError(t, errs, "plan.md", "1-3개의 행이 필요합니다")
}

// A plan.md without the section must report the missing required section once,
// not a second row-structure finding on an absent table.
func TestValidateSpecSet_RejectsMissingProbeSection(t *testing.T) {
	t.Parallel()

	plan := strings.Replace(validPlanMD(), probeSection(defaultProbeRow), "", 1)
	specDir, doc := writeAuthoringPreflightSpec(t, map[string]string{"plan.md": plan})

	errs := spec.ValidateSpecSet(specDir, doc)
	assertValidationError(t, errs, "plan.md", "섹션 누락: ## Risk-First Integration Probe")

	var probeFindings int
	for _, e := range errs {
		if e.Level == "error" && strings.Contains(e.Message, "Risk-First Integration Probe") {
			probeFindings++
		}
	}
	require.Equal(t, 1, probeFindings, "missing section must not also report row findings: %#v", errs)
}

// The scaffolded placeholder row is the state every new SPEC starts in, so it
// must be structurally valid on its own.
func TestScaffoldedPlanProbeRowPassesPreflight(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, spec.Scaffold(dir, "PROBE-001", "Probe scaffold"))
	specDir := filepath.Join(dir, ".autopus", "specs", "SPEC-PROBE-001")

	plan, err := os.ReadFile(filepath.Join(specDir, "plan.md"))
	require.NoError(t, err)
	require.Contains(t, string(plan), "not-run")

	doc, err := spec.Load(specDir)
	require.NoError(t, err)
	assertNoProbeError(t, spec.ValidateSpecSet(specDir, doc))
}
