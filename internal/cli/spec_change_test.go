package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/spec/gates"
)

func runSpecChange(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := newSpecChangeCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func decodeChangeContract(t *testing.T, out string) specChangeContract {
	t.Helper()
	var contract specChangeContract
	require.NoError(t, json.Unmarshal([]byte(out), &contract), "--json output must round-trip through the Go struct")
	return contract
}

func changeApplicability(t *testing.T, contract specChangeContract, id gates.GateID) gates.GateDecision {
	t.Helper()
	for _, decision := range contract.Gates {
		if decision.Gate == id {
			return decision
		}
	}
	t.Fatalf("gate %s missing from the contract", id)
	return gates.GateDecision{}
}

// A small UI change is the low-risk default: the compact contract is written,
// the four-document SPEC gate is not applicable, and UX capture stays required.
func TestSpecChangeCmd_SmallUITakesTheCompactPath(t *testing.T) {
	_, specDir := specGatesProject(t)

	out, err := runSpecChange(t, "SPEC-GATES-001",
		"--class", "small_ui", "--ac", "AC-001",
		"--surface", "frontend/src/components/Button.tsx",
		"--verify", "npx playwright test button.spec.ts", "--json")
	require.NoError(t, err)

	contract := decodeChangeContract(t, out)
	assert.Equal(t, specChangeSchema, contract.Schema)
	assert.Equal(t, "SPEC-GATES-001", contract.SpecID)
	assert.Equal(t, []string{"AC-001"}, contract.AcceptanceIDs)
	assert.Equal(t, gates.RiskLow, contract.Risk.Tier)
	assert.Equal(t, gates.DecisionCompact, contract.Risk.Decision)
	assert.Empty(t, contract.Risk.Reasons)

	authoring := changeApplicability(t, contract, gates.GateSpecAuthoring)
	assert.Equal(t, gates.NotApplicable, authoring.Applicability)
	assert.Contains(t, authoring.Reason, "compact change contract replaces the four-document SPEC set")
	for _, id := range []gates.GateID{gates.GateAccessibility, gates.GateUXVerification} {
		assert.Equal(t, gates.Required, changeApplicability(t, contract, id).Applicability, "%s", id)
	}
	assert.Equal(t, gates.NotApplicable, changeApplicability(t, contract, gates.GateRiskFirstProbe).Applicability)

	body, readErr := os.ReadFile(filepath.Join(specDir, "change.md"))
	require.NoError(t, readErr, "the compact contract is written into the SPEC directory")
	assert.Contains(t, string(body), "spec_id: SPEC-GATES-001")
	assert.Contains(t, string(body), "decision: compact_contract")
	assert.Contains(t, string(body), "- Acceptance criteria: `AC-001`")
	assert.Contains(t, string(body), "| `ux_verification` | `required` |")
	assert.Contains(t, string(body), "npx playwright test button.spec.ts")
	assert.Equal(t, ".autopus/specs/SPEC-GATES-001/change.md", contract.ContractPath,
		"a SPEC addressed by id yields a project-root-relative contract path")

	_, err = runSpecChange(t, "SPEC-GATES-001",
		"--class", "small_ui", "--ac", "AC-001",
		"--surface", "frontend/src/components/Button.tsx", "--verify", "x")
	require.Error(t, err, "an existing contract is not silently overwritten")
	assert.Contains(t, err.Error(), "--force")
}

// A security or data surface needs the full SPEC set and the risk-first probe;
// the compact contract is refused with the reason named.
func TestSpecChangeCmd_SecurityPathRequiresFullSpecAndProbe(t *testing.T) {
	_, specDir := specGatesProject(t)

	out, err := runSpecChange(t, specDir,
		"--class", "bugfix_existing_contract", "--ac", "AC-001",
		"--surface", "backend/db/migrations/001.sql",
		"--verify", "go test ./backend/db/...", "--json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), string(gates.DecisionEscalate))
	assert.Contains(t, err.Error(), gates.EscalationSecuritySurface)
	assert.Contains(t, err.Error(), "risk-first integration probe")

	contract := decodeChangeContract(t, out)
	assert.Equal(t, gates.RiskHigh, contract.Risk.Tier)
	assert.Equal(t, gates.KindSecurityOrData, contract.Risk.EffectiveClass)
	assert.Equal(t, gates.Required, changeApplicability(t, contract, gates.GateSpecAuthoring).Applicability)
	assert.Equal(t, gates.Required, changeApplicability(t, contract, gates.GateRiskFirstProbe).Applicability)
	assert.Contains(t, changeApplicability(t, contract, gates.GateSpecAuthoring).Reason, gates.EscalationSecuritySurface)

	_, statErr := os.Stat(filepath.Join(specDir, "change.md"))
	assert.True(t, os.IsNotExist(statErr), "an escalated change never writes a compact contract")
}

func TestSpecChangeCmd_MultiDomainSurfaceEscalates(t *testing.T) {
	_, specDir := specGatesProject(t)

	out, err := runSpecChange(t, specDir,
		"--class", "bugfix_existing_contract", "--ac", "AC-001",
		"--surface", "pkg/a/x.go,pkg/b/y.go", "--verify", "go test ./...")
	require.Error(t, err)
	assert.Contains(t, err.Error(), string(gates.DecisionEscalate))
	assert.Contains(t, err.Error(), gates.EscalationMultiDomain)
	assert.Contains(t, out, "escalate_to_full_spec")
	assert.Contains(t, out, "spec_authoring: required")
}

// The issue's escalation case: a change declared test_only whose surface
// includes production source must report escalate_to_full_spec.
func TestSpecChangeCmd_DeclaredTestOnlyTouchingProductionSourceEscalates(t *testing.T) {
	_, specDir := specGatesProject(t)

	out, err := runSpecChange(t, specDir,
		"--class", "test_only", "--ac", "AC-001",
		"--surface", "pkg/foo/foo_test.go,pkg/foo/foo.go",
		"--verify", "go test ./pkg/foo/...", "--json")
	require.Error(t, err)
	assert.Contains(t, err.Error(), string(gates.DecisionEscalate))
	assert.Contains(t, err.Error(), gates.EscalationTestOnlyCode)

	contract := decodeChangeContract(t, out)
	assert.Equal(t, gates.KindTestOnly, contract.Risk.DeclaredClass)
	assert.Equal(t, gates.DecisionEscalate, contract.Risk.Decision)
	assert.Equal(t, []string{gates.EscalationTestOnlyCode}, contract.Risk.Reasons)

	pure, err := runSpecChange(t, specDir,
		"--class", "test_only", "--ac", "AC-002",
		"--surface", "pkg/foo/foo_test.go", "--verify", "go test ./pkg/foo/...", "--json")
	require.NoError(t, err, "test material alone stays on the compact path")
	assert.Equal(t, gates.DecisionCompact, decodeChangeContract(t, pure).Risk.Decision)
}

func TestSpecChangeCmd_NewContractEscalates(t *testing.T) {
	_, specDir := specGatesProject(t)

	_, err := runSpecChange(t, specDir,
		"--class", "bugfix_existing_contract", "--ac", "AC-001",
		"--surface", "pkg/foo/foo.go", "--verify", "go test ./pkg/foo/...", "--new-contract")
	require.Error(t, err)
	assert.Contains(t, err.Error(), gates.EscalationNewContract)
}

func TestSpecChangeCmd_RefusesMissingSpecOrAcceptanceReference(t *testing.T) {
	_, specDir := specGatesProject(t)
	common := []string{"--class", "small_ui", "--surface", "frontend/src/components/Button.tsx", "--verify", "x"}

	_, err := runSpecChange(t, append([]string{"SPEC-NOPE", "--ac", "AC-001"}, common...)...)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "SPEC-NOPE not found")

	_, err = runSpecChange(t, append([]string{specDir, "--ac", "AC-999"}, common...)...)
	require.Error(t, err)
	assert.Contains(t, err.Error(), `acceptance criterion "AC-999" not found`)
	assert.Contains(t, err.Error(), "AC-001", "the error lists the acceptance ids that do exist")

	require.NoError(t, os.Remove(filepath.Join(specDir, "acceptance.md")))
	_, err = runSpecChange(t, append([]string{specDir, "--ac", "AC-001"}, common...)...)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "must reference existing acceptance criteria")
}

func TestSpecChangeCmd_RequiresContractFields(t *testing.T) {
	_, specDir := specGatesProject(t)
	cases := []struct {
		name string
		args []string
		want string
	}{
		{"missing class", []string{specDir, "--ac", "AC-001", "--surface", "pkg/a/x.go", "--verify", "x"}, "--class is required"},
		{"unknown class", []string{specDir, "--class", "refactor", "--ac", "AC-001", "--surface", "pkg/a/x.go", "--verify", "x"}, "unknown change class"},
		{"missing ac", []string{specDir, "--class", "small_ui", "--surface", "pkg/a/x.go", "--verify", "x"}, "--ac is required"},
		{"missing surface", []string{specDir, "--class", "small_ui", "--ac", "AC-001", "--verify", "x"}, "--surface is required"},
		{"missing verify", []string{specDir, "--class", "small_ui", "--ac", "AC-001", "--surface", "pkg/a/x.go"}, "--verify is required"},
		{"bad change id", []string{specDir, "--class", "small_ui", "--ac", "AC-001", "--surface", "pkg/a/x.go", "--verify", "x", "--change-id", "../escape"}, "invalid --change-id"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := runSpecChange(t, tc.args...)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.want)
		})
	}
}

func TestSpecChangeCmd_ChangeIDWritesSiblingDirectory(t *testing.T) {
	root, _ := specGatesProject(t)

	out, err := runSpecChange(t, "SPEC-GATES-001",
		"--class", "test_only", "--ac", "AC-001,AC-002",
		"--surface", "pkg/a/x_test.go", "--verify", "go test ./pkg/a/...",
		"--change-id", "REGRESSION-01", "--json")
	require.NoError(t, err)

	contract := decodeChangeContract(t, out)
	assert.Equal(t, []string{"AC-001", "AC-002"}, contract.AcceptanceIDs)
	assert.Equal(t, ".autopus/specs/CHG-REGRESSION-01/change.md", contract.ContractPath)

	body, readErr := os.ReadFile(filepath.Join(root, ".autopus", "specs", "CHG-REGRESSION-01", "change.md"))
	require.NoError(t, readErr)
	assert.Contains(t, string(body), "change_id: CHG-REGRESSION-01")
	assert.Contains(t, string(body), "spec_id: SPEC-GATES-001")
	assert.Contains(t, string(body), ".autopus/specs/SPEC-GATES-001/spec.md",
		"the sibling contract points back at the referenced SPEC")
}

// auto spec gates honours the declared class, so the applicability receipt is
// the machine-readable decision the pipeline reads.
func TestSpecGatesCmd_DeclaredChangeClassDrivesAuthoringGate(t *testing.T) {
	_, specDir := specGatesProject(t)

	low, err := runSpecGates(t, specDir, "--changed", "frontend/src/components/Button.tsx",
		"--change-class", "small_ui")
	require.NoError(t, err)
	assert.Contains(t, low, "change risk: low compact_contract (declared small_ui, effective small_ui)")
	assert.Contains(t, low, "spec_authoring: not_applicable")

	escalated, err := runSpecGates(t, specDir, "--changed", "pkg/foo/foo_test.go,pkg/foo/foo.go",
		"--change-class", "test_only", "--json")
	require.NoError(t, err)
	var receipt gates.ApplicabilityReceipt
	require.NoError(t, json.Unmarshal([]byte(escalated), &receipt))
	assert.Equal(t, gates.DecisionEscalate, receipt.ChangeRisk.Decision)
	assert.Equal(t, []string{gates.EscalationTestOnlyCode}, receipt.ChangeRisk.Reasons)
	authoring, ok := receipt.Decision(gates.GateSpecAuthoring)
	require.True(t, ok)
	assert.Equal(t, gates.Required, authoring.Applicability)

	_, err = runSpecGates(t, specDir, "--changed", "pkg/a/x.go", "--change-class", "refactor")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unknown change class")
}
