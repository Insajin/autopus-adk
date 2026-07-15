package spec

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func TestBuildReviewPromptFromContextDeliveryChecked_IncludesEachVerifiedDocumentOnce(t *testing.T) {
	t.Parallel()

	root, specDir, specRef, markers := writeReviewContextDeliveryFixture(t)
	extraRef := "docs/supervisor-contract.md"
	extraMarker := "SUPERVISOR_EXTRA_TAIL"
	writeReviewContextFile(t, root, extraRef, "supervisor contract\n"+extraMarker)
	deliveryOpts := promptlayer.ContextDeliveryOptions{
		Root: root, Command: "review", SpecDir: specRef,
		RequiredReferences: []string{extraRef},
	}
	delivery, err := promptlayer.BuildContextDelivery(deliveryOpts)
	require.NoError(t, err)
	require.NoError(t, promptlayer.VerifyContextDeliveryForOptions(deliveryOpts, delivery))
	doc, err := Load(specDir)
	require.NoError(t, err)

	prompt, err := BuildReviewPromptFromContextDeliveryChecked(doc, "", ReviewPromptOptions{}, delivery)

	require.NoError(t, err)
	markers[extraRef] = extraMarker
	for ref, marker := range markers {
		assert.Equalf(t, 1, strings.Count(prompt, marker), "required body %s must appear once", ref)
		receipt := contextDeliveryDocumentByRef(t, delivery, ref)
		assert.Contains(t, prompt, "source_ref="+ref)
		assert.Contains(t, prompt, "source_sha256="+receipt.SourceHash)
		assert.Contains(t, prompt, "prompt_sha256="+receipt.PromptHash)
	}
	assert.NotContains(t, prompt, "[Review-context notice:")
}

func TestBuildReviewPromptFromContextDeliveryChecked_RejectsBodyHashMismatch(t *testing.T) {
	t.Parallel()

	root, specDir, specRef, _ := writeReviewContextDeliveryFixture(t)
	deliveryOpts := promptlayer.ContextDeliveryOptions{Root: root, Command: "review", SpecDir: specRef}
	delivery, err := promptlayer.BuildContextDelivery(deliveryOpts)
	require.NoError(t, err)
	require.NoError(t, promptlayer.VerifyContextDeliveryForOptions(deliveryOpts, delivery))
	delivery.Layers[0].Content += "tampered"
	doc, err := Load(specDir)
	require.NoError(t, err)

	prompt, err := BuildReviewPromptFromContextDeliveryChecked(doc, "", ReviewPromptOptions{}, delivery)

	require.Error(t, err)
	assert.ErrorContains(t, err, "prompt hash mismatch")
	assert.Empty(t, prompt)
}

func writeReviewContextDeliveryFixture(t *testing.T) (string, string, string, map[string]string) {
	t.Helper()
	root := t.TempDir()
	specID := "SPEC-REVIEW-CONTEXT-001"
	specRef := filepath.ToSlash(filepath.Join(".autopus", "specs", specID))
	specDir := filepath.Join(root, filepath.FromSlash(specRef))
	require.NoError(t, os.MkdirAll(specDir, 0o700))
	markers := map[string]string{
		"AGENTS.md":                     "AGENTS_CONTEXT_TAIL",
		".autopus/project/workspace.md": "WORKSPACE_CONTEXT_TAIL",
		"ARCHITECTURE.md":               "ARCHITECTURE_CONTEXT_TAIL",
		specRef + "/spec.md":            "SPEC_CONTEXT_TAIL",
		specRef + "/plan.md":            "PLAN_CONTEXT_TAIL",
		specRef + "/research.md":        "RESEARCH_CONTEXT_TAIL",
		specRef + "/acceptance.md":      "ACCEPTANCE_CONTEXT_TAIL",
	}
	for ref, marker := range markers {
		content := ref + "\n" + marker
		if strings.HasSuffix(ref, "/spec.md") {
			content = completeReviewSpecFixture(specID, "Verified Review Context", 5) + "\n" + marker
		}
		writeReviewContextFile(t, root, ref, content)
	}
	return root, specDir, specRef, markers
}

func writeReviewContextFile(t *testing.T, root, ref, content string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(ref))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func contextDeliveryDocumentByRef(
	t *testing.T,
	delivery promptlayer.ContextDeliveryResult,
	ref string,
) promptlayer.ContextDeliveryDocument {
	t.Helper()
	for _, document := range delivery.RequiredDocuments {
		if document.SourceRef == ref {
			return document
		}
	}
	t.Fatalf("context delivery document not found: %s", ref)
	return promptlayer.ContextDeliveryDocument{}
}
