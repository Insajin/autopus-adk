package memindex

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

const (
	minReceiptBudgetTokens = 800
	maxReceiptBudgetTokens = 2000
)

func buildContextReceipt(opts ContextOptions, budget, topK int) (ContextResult, error) {
	if budget < minReceiptBudgetTokens || budget > maxReceiptBudgetTokens {
		return ContextResult{}, fmt.Errorf("context receipt budget must be between %d and %d tokens", minReceiptBudgetTokens, maxReceiptBudgetTokens)
	}

	result := mandatoryContextResult(opts, budget)
	header := renderReceiptHeader(result)
	headerWithGap := header + "\n\n"
	mandatoryTokens := promptlayer.EstimateTokens(headerWithGap) + 1
	if mandatoryTokens >= budget {
		return ContextResult{}, fmt.Errorf("mandatory context receipt fields exceed %d-token budget", budget)
	}
	result.RecallBudgetTokens = budget - mandatoryTokens

	response, err := Search(SearchOptions{
		ProjectDir: opts.ProjectDir,
		IndexPath:  opts.IndexPath,
		Query:      opts.Query,
		TopK:       topK,
	})
	if err != nil {
		return ContextResult{}, err
	}
	selected, omitted, recallPrompt := renderContext(opts.Query, response.Results, result.RecallBudgetTokens)
	result.Results = selected
	result.OmittedCount = omitted
	headerWithGap = renderReceiptHeader(result) + "\n\n"
	result.Prompt = headerWithGap + recallPrompt
	result.EstimatedTokens = promptlayer.EstimateTokens(result.Prompt)
	if result.EstimatedTokens > budget {
		return ContextResult{}, fmt.Errorf("context receipt estimate %d exceeds %d-token budget", result.EstimatedTokens, budget)
	}
	return result, nil
}

func mandatoryContextResult(opts ContextOptions, budget int) ContextResult {
	return ContextResult{
		Query: opts.Query, BudgetTokens: budget,
		OutcomeLock:        receiptSafeText(opts.OutcomeLock),
		Constraints:        receiptSafeStrings(opts.Constraints),
		OwnedPaths:         receiptSafeStrings(opts.OwnedPaths),
		ForbiddenPaths:     receiptSafeStrings(opts.ForbiddenPaths),
		AcceptanceCriteria: receiptSafeStrings(opts.AcceptanceCriteria),
		RequiredReferences: receiptSafeStrings(opts.RequiredReferences),
		DecisionDelta:      receiptSafeText(opts.DecisionDelta),
		SnapshotHash:       receiptSafeText(opts.SnapshotHash),
		PromptManifestHash: receiptSafeText(opts.PromptManifestHash),
	}
}

func renderReceiptHeader(result ContextResult) string {
	var b strings.Builder
	b.WriteString("## Context Receipt\n")
	fmt.Fprintf(&b, "budget_tokens: %d\n", result.BudgetTokens)
	fmt.Fprintf(&b, "omitted_results: %d\n", result.OmittedCount)
	fmt.Fprintf(&b, "outcome_lock: %s\n", result.OutcomeLock)
	writeReceiptList(&b, "constraints", result.Constraints)
	writeReceiptList(&b, "owned_paths", result.OwnedPaths)
	writeReceiptList(&b, "forbidden_paths", result.ForbiddenPaths)
	writeReceiptList(&b, "acceptance_criteria", result.AcceptanceCriteria)
	writeReceiptList(&b, "required_references", result.RequiredReferences)
	fmt.Fprintf(&b, "decision_delta: %s\n", result.DecisionDelta)
	fmt.Fprintf(&b, "snapshot_hash: %s\n", result.SnapshotHash)
	fmt.Fprintf(&b, "prompt_manifest_hash: %s", result.PromptManifestHash)
	return b.String()
}

func writeReceiptList(b *strings.Builder, label string, values []string) {
	fmt.Fprintf(b, "%s:\n", label)
	for _, value := range values {
		fmt.Fprintf(b, "- %s\n", value)
	}
}

func receiptSafeStrings(values []string) []string {
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = receiptSafeText(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}
