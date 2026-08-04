package pipeline

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

type requiredPhaseDocument struct {
	name     string
	label    string
	optional bool
	project  bool
}

func (b *PhasePromptBuilder) requiredPhaseDocumentLayers() ([]promptlayer.Layer, error) {
	b.requiredSnapshotOnce.Do(func() {
		b.requiredSnapshotLayers, b.requiredSnapshotLoadErr = b.loadRequiredPhaseDocumentLayers()
	})
	if b.requiredSnapshotLoadErr != nil {
		return nil, b.requiredSnapshotLoadErr
	}
	return append([]promptlayer.Layer(nil), b.requiredSnapshotLayers...), nil
}

func (b *PhasePromptBuilder) loadRequiredPhaseDocumentLayers() ([]promptlayer.Layer, error) {
	return b.loadRequiredPhaseDocumentLayersAfterFirstPass(b.requiredSnapshotFirstPassHook)
}

// @AX:WARN [AUTO]: Frozen SPEC snapshot assembly has cyclomatic complexity 16 and nine explicit branches.
// @AX:REASON [AUTO]: Both read passes, optional research, SPEC identity, and the resolved snapshot hash must stay coherent.
func (b *PhasePromptBuilder) loadRequiredPhaseDocumentLayersAfterFirstPass(afterFirstPass func()) ([]promptlayer.Layer, error) {
	documents := []requiredPhaseDocument{
		{name: "spec.md", label: "SPEC", project: true},
		{name: "plan.md", label: "Plan", project: true},
		{name: "acceptance.md", label: "Acceptance", project: true},
		{name: "research.md", optional: true},
	}
	contents := make(map[string]string, len(documents))
	for _, document := range documents {
		content, err := b.readPhaseSnapshotDocument(document)
		if err != nil {
			return nil, err
		}
		if content == "" && document.optional {
			continue
		}
		if document.name == "spec.md" {
			if err := promptlayer.VerifyContextSpecIdentity(b.specDir, []byte(content)); err != nil {
				return nil, err
			}
		}
		contents[document.name] = content
	}
	if afterFirstPass != nil {
		afterFirstPass()
	}
	for _, document := range documents {
		content, err := b.readPhaseSnapshotDocument(document)
		if err != nil {
			return nil, fmt.Errorf("verify required phase snapshot: %w", err)
		}
		if content != contents[document.name] {
			return nil, fmt.Errorf("required phase document %s changed while snapshot was built", document.name)
		}
	}
	raw := make(map[string][]byte, len(contents))
	for name, content := range contents {
		raw[name] = []byte(content)
	}
	if b.expectedSnapshotHash != "" && specSnapshotHashFromDocuments(raw) != b.expectedSnapshotHash {
		return nil, fmt.Errorf("required phase documents do not match the resolved SPEC snapshot")
	}

	layers := make([]promptlayer.Layer, 0, len(documents))
	for _, document := range documents {
		if !document.project {
			continue
		}
		sanitized := sanitizeRequiredPhaseDocument(contents[document.name])
		id := "phase:" + strings.TrimSuffix(document.name, ".md")
		layers = append(layers, phaseFileLayer(id, document.name, document.label, sanitized))
	}
	return layers, nil
}

func (b *PhasePromptBuilder) readPhaseSnapshotDocument(document requiredPhaseDocument) (string, error) {
	content, err := b.readRequiredPhaseDocument(document.name)
	if document.optional && errors.Is(err, os.ErrNotExist) {
		return "", nil
	}
	return content, err
}

// @AX:WARN [AUTO]: Required-document admission has ten fail-closed path, identity, file-mode, and content branches.
// @AX:REASON [AUTO]: A symlink, path escape, non-regular file, read failure, or empty required document invalidates the frozen snapshot.
func (b *PhasePromptBuilder) readRequiredPhaseDocument(name string) (string, error) {
	if filepath.IsAbs(name) || filepath.Clean(name) != name || filepath.Base(name) != name {
		return "", fmt.Errorf("required phase document %s has invalid path", name)
	}

	root, err := filepath.Abs(b.specDir)
	if err != nil {
		return "", fmt.Errorf("resolve SPEC directory: %w", err)
	}
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", fmt.Errorf("resolve SPEC directory: %w", err)
	}
	candidate := filepath.Join(root, name)
	info, err := os.Lstat(candidate)
	if err != nil {
		return "", fmt.Errorf("required phase document %s: %w", name, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("required phase document %s must not be a symlink", name)
	}
	if !info.Mode().IsRegular() {
		return "", fmt.Errorf("required phase document %s must be a regular file", name)
	}
	resolvedCandidate, err := filepath.EvalSymlinks(candidate)
	if err != nil {
		return "", fmt.Errorf("resolve required phase document %s: %w", name, err)
	}
	if !isPathBeneath(resolvedRoot, resolvedCandidate) {
		return "", fmt.Errorf("required phase document %s resolves outside SPEC directory", name)
	}
	data, err := os.ReadFile(candidate)
	if err != nil {
		return "", fmt.Errorf("read required phase document %s: %w", name, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return "", fmt.Errorf("required phase document %s is empty", name)
	}
	return string(data), nil
}

func isPathBeneath(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil || rel == "." || filepath.IsAbs(rel) {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func sanitizeRequiredPhaseDocument(raw string) promptlayer.SanitizedContent {
	return promptlayer.SanitizeContent(raw, promptlayer.ContextOptions{
		Required: true, PreserveInjectionEvidence: true,
	})
}
