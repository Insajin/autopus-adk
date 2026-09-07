package gates

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assessFor(t *testing.T, declared ChangeKind, newContract bool, paths ...string) ChangeRisk {
	t.Helper()
	return AssessChange(declared, Classify(paths, nil), newContract)
}

// A small UI change is the canonical low-risk case: the compact contract is
// the default and the four-document SPEC set does not apply.
func TestAssessChange_SmallUIIsCompactByDefault(t *testing.T) {
	risk := assessFor(t, KindSmallUI, false, "frontend/src/components/Button.tsx")

	assert.Equal(t, RiskLow, risk.Tier)
	assert.Equal(t, DecisionCompact, risk.Decision)
	assert.Equal(t, KindSmallUI, risk.EffectiveClass)
	assert.Empty(t, risk.Reasons, "a compact decision carries no escalation reason")
}

func TestAssessChange_LowRiskClassesTakeTheCompactPath(t *testing.T) {
	cases := map[ChangeKind][]string{
		KindTestOnly: {"pkg/foo/foo_test.go"},
		KindDocsOnly: {"docs/guide.md", "README.md"},
		KindSmallUI:  {"frontend/src/components/Button.tsx"},
		KindBugfix:   {"pkg/foo/foo.go"},
	}
	for declared, paths := range cases {
		t.Run(string(declared), func(t *testing.T) {
			risk := assessFor(t, declared, false, paths...)
			assert.Equal(t, RiskLow, risk.Tier)
			assert.Equal(t, DecisionCompact, risk.Decision)
		})
	}
}

func TestAssessChange_HighRiskClassesNeverCompact(t *testing.T) {
	cases := map[ChangeKind][]string{
		KindFeature:        {"pkg/foo/foo.go"},
		KindMultiDomain:    {"pkg/a/x.go", "pkg/b/y.go"},
		KindSecurityOrData: {"backend/db/migrations/001.sql"},
	}
	for declared, paths := range cases {
		t.Run(string(declared), func(t *testing.T) {
			risk := assessFor(t, declared, false, paths...)
			assert.Equal(t, RiskHigh, risk.Tier)
			assert.Equal(t, DecisionEscalate, risk.Decision)
			assert.Contains(t, risk.Reasons, EscalationDeclaredHighRisk)
		})
	}
}

// Declaring test_only while touching production source is the escalation the
// issue names: the command must refuse instead of proceeding compactly.
func TestAssessChange_DeclaredTestOnlyTouchingProductionSourceEscalates(t *testing.T) {
	risk := assessFor(t, KindTestOnly, false, "pkg/foo/foo_test.go", "pkg/foo/foo.go")

	assert.Equal(t, DecisionEscalate, risk.Decision)
	assert.Equal(t, RiskHigh, risk.Tier)
	assert.Equal(t, KindTestOnly, risk.DeclaredClass)
	assert.Equal(t, KindFeature, risk.EffectiveClass, "the effective class outranks the declaration")
	assert.Equal(t, []string{EscalationTestOnlyCode}, risk.Reasons)
}

func TestAssessChange_DeclaredTestOnlyStaysCompactForTestMaterial(t *testing.T) {
	risk := assessFor(t, KindTestOnly, false,
		"pkg/foo/foo_test.go", "pkg/foo/testdata/golden.json", "web/src/Button.spec.tsx", "docs/testing.md")

	assert.Equal(t, DecisionCompact, risk.Decision)
	assert.Empty(t, risk.Reasons)
}

func TestAssessChange_DeclaredDocsOnlyTouchingCodeEscalates(t *testing.T) {
	risk := assessFor(t, KindDocsOnly, false, "README.md", "pkg/foo/foo.go")

	assert.Equal(t, DecisionEscalate, risk.Decision)
	assert.Equal(t, []string{EscalationDocsOnlyCode}, risk.Reasons)
}

func TestAssessChange_DeclaredSmallUITouchingNonUISourceEscalates(t *testing.T) {
	risk := assessFor(t, KindSmallUI, false, "frontend/src/components/Button.tsx", "frontend/src/store.ts")

	assert.Equal(t, DecisionEscalate, risk.Decision)
	assert.Equal(t, []string{EscalationSmallUINonUI}, risk.Reasons)
}

// A security or data path outranks any lower declaration; escalation must not
// depend on how many files the change touches.
func TestAssessChange_SecurityPathEscalatesRegardlessOfFileCount(t *testing.T) {
	risk := assessFor(t, KindBugfix, false, "backend/db/migrations/001.sql")

	assert.Equal(t, RiskHigh, risk.Tier)
	assert.Equal(t, DecisionEscalate, risk.Decision)
	assert.Equal(t, KindSecurityOrData, risk.EffectiveClass)
	assert.Contains(t, risk.Reasons, EscalationSecuritySurface)

	authRisk := assessFor(t, KindTestOnly, false, "internal/auth/token_test.go")
	assert.Equal(t, DecisionEscalate, authRisk.Decision, "an auth path escalates even for a single test file")
	assert.Contains(t, authRisk.Reasons, EscalationSecuritySurface)
}

func TestAssessChange_MultiDomainSurfaceEscalatesLowDeclaration(t *testing.T) {
	risk := assessFor(t, KindBugfix, false, "pkg/a/x.go", "pkg/b/y.go")

	assert.Equal(t, RiskHigh, risk.Tier)
	assert.Equal(t, DecisionEscalate, risk.Decision)
	assert.Equal(t, KindMultiDomain, risk.EffectiveClass)
	assert.Contains(t, risk.Reasons, EscalationMultiDomain)
}

func TestAssessChange_NewContractEscalatesLowDeclaration(t *testing.T) {
	declaredNew := assessFor(t, KindBugfix, true, "pkg/foo/foo.go")
	assert.Equal(t, DecisionEscalate, declaredNew.Decision)
	assert.Contains(t, declaredNew.Reasons, EscalationNewContract)

	surface := assessFor(t, KindBugfix, false, "api/v1/users.go")
	assert.Equal(t, DecisionEscalate, surface.Decision, "a public API root is a contract surface")
	assert.Contains(t, surface.Reasons, EscalationContractSurface)

	idl := assessFor(t, KindSmallUI, false, "web/schema.graphql")
	assert.Equal(t, DecisionEscalate, idl.Decision, "an IDL file is a contract surface")
	assert.Contains(t, idl.Reasons, EscalationContractSurface)
}

// A large single-domain change stays low risk: file count alone must not
// escalate, only the declared class and the path signals do.
func TestAssessChange_FileCountAloneDoesNotEscalate(t *testing.T) {
	paths := []string{
		"pkg/foo/a.go", "pkg/foo/b.go", "pkg/foo/c.go", "pkg/foo/d.go",
		"pkg/foo/e.go", "pkg/foo/f.go", "pkg/foo/g.go", "pkg/foo/h.go",
		"pkg/foo/i.go", "pkg/foo/j.go", "pkg/foo/k.go", "pkg/foo/l.go",
	}
	risk := assessFor(t, KindBugfix, false, paths...)

	assert.Equal(t, RiskLow, risk.Tier)
	assert.Equal(t, DecisionCompact, risk.Decision)
}

func TestDeriveChangeKind_ReadsTheSurfaceWhenNothingIsDeclared(t *testing.T) {
	cases := []struct {
		paths []string
		want  ChangeKind
	}{
		{[]string{"docs/guide.md"}, KindDocsOnly},
		{[]string{"frontend/src/components/Button.tsx"}, KindSmallUI},
		{[]string{"backend/db/migrations/001.sql"}, KindSecurityOrData},
		{[]string{"pkg/a/x.go", "pkg/b/y.go"}, KindMultiDomain},
		{[]string{"pkg/foo/foo_test.go"}, KindTestOnly},
		{[]string{"pkg/foo/foo.go"}, KindFeature},
	}
	for _, tc := range cases {
		classification := Classify(tc.paths, nil)
		assert.Equal(t, tc.want, DeriveChangeKind(classification), "%v", tc.paths)

		risk := AssessChange("", classification, false)
		assert.Equal(t, tc.want, risk.DeclaredClass, "an undeclared class comes from the surface")
		assert.Equal(t, tc.want, risk.EffectiveClass, "a derived class cannot contradict itself")
		if HighRiskKind(tc.want) {
			assert.Equal(t, []string{EscalationDeclaredHighRisk}, risk.Reasons, "%v", tc.paths)
			continue
		}
		assert.Empty(t, risk.Reasons, "%v", tc.paths)
	}
}

func TestParseChangeKind(t *testing.T) {
	kind, err := ParseChangeKind("  Small_UI ")
	require.NoError(t, err)
	assert.Equal(t, KindSmallUI, kind)

	_, err = ParseChangeKind("refactor")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown change class \"refactor\"")
	assert.Contains(t, err.Error(), string(KindBugfix), "the error lists the accepted classes")
}

func TestIsTestPath(t *testing.T) {
	for _, p := range []string{
		"pkg/foo/foo_test.go", "web/src/Button.test.tsx", "web/src/Button.spec.ts",
		"pkg/foo/testdata/golden.json", "tests/e2e/login.py", "app/__tests__/render.jsx",
		"python/test_login.py",
	} {
		assert.True(t, IsTestPath(p), "%s is test material", p)
	}
	for _, p := range []string{
		"pkg/foo/foo.go", "pkg/spec/gates/change.go", "frontend/src/components/Button.tsx",
		"internal/latest/version.go",
	} {
		assert.False(t, IsTestPath(p), "%s is not test material", p)
	}
}

func TestIsContractPath(t *testing.T) {
	for _, p := range []string{
		"api/v1/users.go", "proto/user.proto", "web/schema.graphql", "docs/openapi.yaml",
		"internal/api/handler.go", "swagger/index.json",
	} {
		assert.True(t, IsContractPath(p), "%s is a contract surface", p)
	}
	for _, p := range []string{
		"pkg/foo/foo.go", "internal/cli/spec_change.go", "frontend/src/components/Button.tsx",
	} {
		assert.False(t, IsContractPath(p), "%s is not a contract surface", p)
	}
}
