// Package pipeline provides pipeline state management types and persistence.
package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// PhaseContext holds runtime context passed to each phase's prompt builder.
type PhaseContext struct {
	// PreviousResults maps PhaseID to the normalized output of that phase.
	PreviousResults map[PhaseID]string
}

// PhasePromptBuilder builds prompts for each pipeline phase by reading files
// from a spec directory and injecting previous phase results.
type PhasePromptBuilder struct {
	specDir string
}

// NewPhasePromptBuilder creates a PhasePromptBuilder that reads files from specDir.
func NewPhasePromptBuilder(specDir string) *PhasePromptBuilder {
	return &PhasePromptBuilder{specDir: specDir}
}

// @AX:NOTE: [AUTO] hardcoded section headers — "## SPEC", "## Plan" etc. are implicit prompt contract with the AI backend
// BuildPrompt constructs the prompt for the given phase using the spec directory
// contents and any prior phase results available in ctx.
func (b *PhasePromptBuilder) BuildPrompt(phaseID PhaseID, ctx PhaseContext) (string, error) {
	var sb strings.Builder

	// @AX:NOTE: [AUTO] magic constant — "spec.md" filename is a hardcoded filesystem contract
	// Always include spec.md when it exists.
	specContent, err := b.readFile("spec.md")
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("read spec.md: %w", err)
	}
	if specContent != "" {
		sb.WriteString("## SPEC\n")
		sb.WriteString(specContent)
		sb.WriteString("\n\n")
	}

	// Phase-specific additional files and context injection.
	switch phaseID {
	case PhasePlan:
		planContent, _ := b.readFile("plan.md")
		if planContent != "" {
			sb.WriteString("## Plan\n")
			sb.WriteString(planContent)
			sb.WriteString("\n\n")
		}

	case PhaseTestScaffold:
		b.appendFileSectionIfPresent(&sb, "acceptance.md", "Acceptance")
		b.injectPrior(&sb, ctx, PhasePlan, "Plan Output")

	case PhaseImplement:
		b.appendFileSectionIfPresent(&sb, "acceptance.md", "Acceptance")
		b.injectPrior(&sb, ctx, PhasePlan, "Plan Output")
		b.injectPrior(&sb, ctx, PhaseTestScaffold, "Test Scaffold Output")

	case PhaseValidate:
		b.appendFileSectionIfPresent(&sb, "acceptance.md", "Acceptance")
		b.injectPrior(&sb, ctx, PhaseImplement, "Implementation Output")

	case PhaseReview:
		b.appendFileSectionIfPresent(&sb, "acceptance.md", "Acceptance")
		b.injectPrior(&sb, ctx, PhaseValidate, "Validation Output")
	}

	return sb.String(), nil
}

// injectPrior appends the result of a prior phase to the prompt if it exists.
func (b *PhasePromptBuilder) injectPrior(sb *strings.Builder, ctx PhaseContext, id PhaseID, label string) {
	if ctx.PreviousResults == nil {
		return
	}
	if result, ok := ctx.PreviousResults[id]; ok && result != "" {
		sb.WriteString(fmt.Sprintf("## %s\n%s\n\n", label, result))
	}
}

func (b *PhasePromptBuilder) appendFileSectionIfPresent(sb *strings.Builder, name, label string) {
	content, err := b.readFile(name)
	if err != nil || content == "" {
		return
	}
	sb.WriteString(fmt.Sprintf("## %s\n%s\n\n", label, content))
}

// readFile reads a file relative to the spec directory and returns its contents.
func (b *PhasePromptBuilder) readFile(name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(b.specDir, name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
