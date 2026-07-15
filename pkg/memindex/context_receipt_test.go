package memindex

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestContext_ReceiptReservesMandatoryFieldsBeforeOptionalRecall(t *testing.T) {
	t.Parallel()

	projectDir := makeMemIndexFixture(t)
	_, err := Rebuild(Options{ProjectDir: projectDir, IndexPath: "receipt.sqlite"})
	require.NoError(t, err)

	result, err := Context(ContextOptions{
		ProjectDir: projectDir, IndexPath: "receipt.sqlite", Query: "approval drift deterministic QA", TopK: 20,
		BudgetTokens: 800, BuildReceipt: true,
		OutcomeLock:        "S7 through S10 pass without weakening Ultra quality",
		Constraints:        []string{"no new dependency", "preserve generated-source ownership"},
		OwnedPaths:         []string{"pkg/memindex/**", "pkg/promptlayer/**", "pkg/pipeline/**"},
		ForbiddenPaths:     []string{"templates/claude/**", "templates/gemini/**"},
		AcceptanceCriteria: []string{"S8", "S9", "S10"},
		RequiredReferences: []string{"spec.md#REQ-UTE-CONTEXT-01", "acceptance.md#S9"},
		DecisionDelta:      "Use ContextResult as the receipt instead of adding a parallel DTO.",
		SnapshotHash:       "snapshot-sha256", PromptManifestHash: "manifest-sha256",
	})
	require.NoError(t, err)

	assert.Equal(t, 800, result.BudgetTokens)
	assert.Positive(t, result.RecallBudgetTokens)
	assert.Less(t, result.RecallBudgetTokens, result.BudgetTokens)
	assert.LessOrEqual(t, result.EstimatedTokens, result.BudgetTokens)
	assert.Contains(t, result.Prompt, "outcome_lock:")
	assert.Contains(t, result.Prompt, "owned_paths:")
	assert.Contains(t, result.Prompt, "snapshot-sha256")
	assert.Contains(t, result.Prompt, "manifest-sha256")
	assert.Contains(t, result.Prompt, "omitted_results:")
	assert.Contains(t, result.Prompt, "source_ref:")
	assert.Contains(t, result.Prompt, "source_hash:")
	assert.GreaterOrEqual(t, result.OmittedCount, 0)
	for _, recalled := range result.Results {
		assert.NotEmpty(t, recalled.SourceRef)
		assert.NotEmpty(t, recalled.SourceHash)
	}
}

func TestContext_ReceiptRejectsOutOfRangeAndOversizedMandatoryContent(t *testing.T) {
	t.Parallel()

	_, err := Context(ContextOptions{BuildReceipt: true, BudgetTokens: 799})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "between 800 and 2000")

	_, err = Context(ContextOptions{
		BuildReceipt: true, BudgetTokens: 800,
		OutcomeLock: strings.Repeat("mandatory outcome ", 3000),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "mandatory")
}
