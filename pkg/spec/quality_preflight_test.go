package spec_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/spec"
)

func TestValidateSpecSet_AcceptsAuthoringPreflightPackage(t *testing.T) {
	t.Parallel()

	specDir, doc := writeAuthoringPreflightSpec(t, nil)

	errs := spec.ValidateSpecSet(specDir, doc)
	for _, e := range errs {
		assert.NotEqual(t, "error", e.Level, "unexpected error: %s", e.Message)
	}
}

func TestValidateSpecSet_RejectsMissingReviewerBrief(t *testing.T) {
	t.Parallel()

	specDir, doc := writeAuthoringPreflightSpec(t, map[string]string{
		"research.md": strings.ReplaceAll(validResearchMD(), "## Reviewer Brief", "## Reviewer Notes"),
	})

	errs := spec.ValidateSpecSet(specDir, doc)
	assertValidationError(t, errs, "research.md", "Reviewer Brief")
}

func TestValidateSpecSet_RejectsUnmappedSemanticInvariant(t *testing.T) {
	t.Parallel()

	specDir, doc := writeAuthoringPreflightSpec(t, map[string]string{
		"spec.md": strings.ReplaceAll(validSpecMD(), "INV-001", "INV-999"),
	})

	errs := spec.ValidateSpecSet(specDir, doc)
	assertValidationError(t, errs, "spec.md", "INV-001")
}

func TestValidateSpecSet_RejectsPromotedEvolutionIdea(t *testing.T) {
	t.Parallel()

	specDir, doc := writeAuthoringPreflightSpec(t, map[string]string{
		"research.md": strings.ReplaceAll(validResearchMD(), "No optional ideas remain.", "Create SPEC-FOLLOWUP-001 later."),
	})

	errs := spec.ValidateSpecSet(specDir, doc)
	assertValidationError(t, errs, "research.md", "Evolution Ideas")
}

func TestValidateSpecSet_RejectsUnresolvedScaffoldPlaceholder(t *testing.T) {
	t.Parallel()

	specDir, doc := writeAuthoringPreflightSpec(t, map[string]string{
		"acceptance.md": strings.ReplaceAll(validAcceptanceMD(), "expected stdout contains \"SPEC 검증 통과\"", "[예상 결과]"),
	})

	errs := spec.ValidateSpecSet(specDir, doc)
	assertValidationError(t, errs, "acceptance.md", "placeholder")
}

func writeAuthoringPreflightSpec(t *testing.T, overrides map[string]string) (string, *spec.SpecDocument) {
	t.Helper()

	specDir := t.TempDir()
	files := map[string]string{
		"spec.md":       validSpecMD(),
		"plan.md":       validPlanMD(),
		"acceptance.md": validAcceptanceMD(),
		"research.md":   validResearchMD(),
	}
	for name, content := range overrides {
		files[name] = content
	}
	for name, content := range files {
		require.NoError(t, os.WriteFile(filepath.Join(specDir, name), []byte(content), 0o644))
	}

	doc, err := spec.Load(specDir)
	require.NoError(t, err)
	return specDir, doc
}

func assertValidationError(t *testing.T, errs []spec.ValidationError, field, contains string) {
	t.Helper()

	for _, e := range errs {
		if e.Level == "error" && e.Field == field && strings.Contains(e.Message, contains) {
			return
		}
	}
	t.Fatalf("expected validation error field=%s contains=%q, got %#v", field, contains, errs)
}

func validSpecMD() string {
	return `# SPEC-QUAL-001: Authoring Preflight

---
id: SPEC-QUAL-001
title: Authoring Preflight
version: 0.1.0
status: draft
---

## Purpose

Improve first-pass SPEC quality.

## Background

Review findings currently close too much authoring debt.

## Outcome Boundary

- Outcome Lock: draft SPECs fail before review when quality evidence is missing.
- Mandatory requirements: full SPEC package validation.
- Explicit non-goals: provider review replacement.
- Completion evidence: auto spec validate reports deterministic failures.

## Requirements

The system SHALL run authoring preflight before review.
WHEN a draft SPEC package is validated THEN the system shall verify traceability evidence.

## Traceability Matrix

| Requirement | Plan Task | Acceptance Scenario | Semantic Invariant |
|-------------|-----------|---------------------|--------------------|
| REQ-001 | T1 | S1 | INV-001 |
`
}

func validPlanMD() string {
	return `# SPEC-QUAL-001 Plan

## Implementation Strategy

Use deterministic markdown checks before provider review.

## Visual Planning Brief

flow: draft -> preflight -> review

## Feature Completion Scope

The Primary SPEC closes the Outcome Lock without sibling dependencies.

## Tasks

- [ ] T1: Add deterministic SPEC package validation.
`
}

func validAcceptanceMD() string {
	return `# SPEC-QUAL-001 Acceptance

## Test Scenarios

### S1: Authoring preflight blocks weak drafts
Priority: Must
Given a draft SPEC package with traceability evidence
When auto spec validate runs
Then expected stdout contains "SPEC 검증 통과"
And expected JSON value verdict equals "PASS"

## Oracle Acceptance Notes

Must scenarios include concrete expected output and explicit tolerance where needed.
`
}

func validResearchMD() string {
	return `# SPEC-QUAL-001 Research

## Outcome Lock

- User-visible outcome: weak drafts fail before provider review.
- Mandatory requirements: deterministic package checks.
- Explicit non-goals: replacing provider review.
- Completion evidence: validation output.

## Visual Planning Brief

flow: draft -> preflight -> review

## Semantic Invariant Inventory

| ID | source clause | invariant type | affected outputs | acceptance IDs |
|----|---------------|----------------|------------------|----------------|
| INV-001 | reviewer criteria move earlier | traceability | validation verdict | S1 |

## Feature Coverage Map

| Outcome slice | Covered by | Status |
|---------------|------------|--------|
| preflight | Primary SPEC | covered |

## Completion Debt

| Item | Blocks | Required resolution |
|------|--------|---------------------|
| None | - | - |

## Evolution Ideas

No optional ideas remain.

## Reference Discipline

| Reference | Type | Verification |
|-----------|------|--------------|
| pkg/spec/validator.go | existing | verified with rg/read |

## Reviewer Brief

- Intended scope: deterministic authoring preflight.
- Explicit non-goals: provider review replacement.
- Self-verified: Traceability Matrix, Semantic Invariant Inventory, oracle acceptance, existing/[NEW] reference discipline.
- Reviewer should focus on: correctness, convergence safety, regression risk, Completion Debt only.

## Self-Verify Summary

- Q-CORR-04 | status: PASS | attempt: 1 | files: research.md | reason: references separated
- Q-COMP-05 | status: PASS | attempt: 1 | files: spec.md, plan.md, acceptance.md, research.md | reason: invariants mapped
- Q-COMP-06 | status: PASS | attempt: 1 | files: spec.md, research.md | reason: scope constrained
- Q-COMP-07 | status: PASS | attempt: 1 | files: research.md | reason: optional ideas separated
`
}
