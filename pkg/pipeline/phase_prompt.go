// Package pipeline provides pipeline state management types and persistence.
package pipeline

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
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
	prompt, _, err := b.BuildPromptWithManifest(phaseID, ctx)
	return prompt, err
}

// BuildPromptWithManifest constructs a phase prompt and diagnostic layer manifest.
// @AX:NOTE [AUTO] @AX:SPEC: SPEC-AGENT-PROMPT-001: phase:* layer IDs mirror prompt sections and prior-phase injections.
func (b *PhasePromptBuilder) BuildPromptWithManifest(phaseID PhaseID, ctx PhaseContext) (string, PromptManifest, error) {
	var layers []promptlayer.Layer

	// @AX:NOTE: [AUTO] magic constant — "spec.md" filename is a hardcoded filesystem contract
	// Always include spec.md when it exists.
	specContent, err := b.readFile("spec.md")
	if err != nil && !os.IsNotExist(err) {
		return "", PromptManifest{}, fmt.Errorf("read spec.md: %w", err)
	}
	if specContent != "" {
		sanitized := sanitizePromptContent(specContent)
		layers = append(layers, phaseFileLayer("phase:spec", "spec.md", "SPEC", sanitized, promptlayer.GroupProjectContext))
	}

	// Phase-specific additional files and context injection.
	switch phaseID {
	case PhasePlan:
		planContent, _ := b.readFile("plan.md")
		if planContent != "" {
			sanitized := sanitizePromptContent(planContent)
			layers = append(layers, phaseFileLayer("phase:plan", "plan.md", "Plan", sanitized, promptlayer.GroupMethodologyTools))
		}

	case PhaseTestScaffold:
		b.appendFileSectionLayerIfPresent(&layers, "acceptance.md", "Acceptance")
		b.injectPriorLayer(&layers, ctx, PhasePlan, "Plan Output")

	case PhaseImplement:
		b.appendFileSectionLayerIfPresent(&layers, "acceptance.md", "Acceptance")
		b.injectPriorLayer(&layers, ctx, PhasePlan, "Plan Output")
		b.injectPriorLayer(&layers, ctx, PhaseTestScaffold, "Test Scaffold Output")

	case PhaseValidate:
		b.appendFileSectionLayerIfPresent(&layers, "acceptance.md", "Acceptance")
		b.injectPriorLayer(&layers, ctx, PhaseImplement, "Implementation Output")

	case PhaseReview:
		b.appendFileSectionLayerIfPresent(&layers, "acceptance.md", "Acceptance")
		b.injectPriorLayer(&layers, ctx, PhaseValidate, "Validation Output")
	}

	rendered, err := promptlayer.Render(layers)
	if err != nil {
		return "", PromptManifest{}, err
	}
	return rendered.Prompt, rendered.Manifest, nil
}

func (b *PhasePromptBuilder) appendFileSectionLayerIfPresent(layers *[]promptlayer.Layer, name, label string) {
	content, err := b.readFile(name)
	if err != nil || content == "" {
		return
	}
	sanitized := sanitizePromptContent(content)
	*layers = append(*layers, phaseFileLayer("phase:"+strings.TrimSuffix(name, ".md"), name, label, sanitized, promptlayer.GroupProjectContext))
}

func (b *PhasePromptBuilder) injectPriorLayer(layers *[]promptlayer.Layer, ctx PhaseContext, id PhaseID, label string) {
	if ctx.PreviousResults == nil {
		return
	}
	if result, ok := ctx.PreviousResults[id]; ok && result != "" {
		sanitized := sanitizePromptContent(result)
		*layers = append(*layers, promptlayer.Layer{
			ID:                 "phase:previous:" + string(id),
			Kind:               promptlayer.KindEphemeral,
			Group:              promptlayer.GroupTaskContext,
			SourceRef:          "previous:" + string(id),
			Content:            formatSection(label, sanitized.Content),
			RedactionStatus:    sanitized.RedactionStatus,
			InvalidationReason: sanitized.InvalidationReason,
		})
	}
}

func phaseFileLayer(id, sourceRef, label string, sanitized promptlayer.SanitizedContent, group promptlayer.Group) promptlayer.Layer {
	return promptlayer.Layer{
		ID:                 id,
		Kind:               promptlayer.KindStable,
		Group:              group,
		SourceRef:          sourceRef,
		Content:            formatSection(label, sanitized.Content),
		CacheEligible:      sanitized.RedactionStatus == promptlayer.RedactionPassed,
		RedactionStatus:    sanitized.RedactionStatus,
		InvalidationReason: sanitized.InvalidationReason,
	}
}

func formatSection(label, content string) string {
	return fmt.Sprintf("## %s\n%s", label, content)
}

func sanitizePromptContent(raw string) promptlayer.SanitizedContent {
	maxBytes := len(raw)*2 + 1024
	if maxBytes < 1 {
		maxBytes = 1
	}
	return promptlayer.SanitizeContent(raw, promptlayer.ContextOptions{MaxBytes: maxBytes})
}

// readFile reads a file relative to the spec directory and returns its contents.
func (b *PhasePromptBuilder) readFile(name string) (string, error) {
	data, err := os.ReadFile(filepath.Join(b.specDir, name))
	if err != nil {
		return "", err
	}
	return string(data), nil
}
