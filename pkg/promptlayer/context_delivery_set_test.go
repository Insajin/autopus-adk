package promptlayer_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func TestVerifyContextDeliveryForOptions_RejectsOmittedTaskSpecificReference(t *testing.T) {
	t.Parallel()

	root := writeContextDeliveryProject(t)
	reference := "docs/task-contract.md"
	path := filepath.Join(root, filepath.FromSlash(reference))
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
	require.NoError(t, os.WriteFile(path, []byte("TASK_CONTRACT_TAIL"), 0o600))
	expected := promptlayer.ContextDeliveryOptions{
		Root: root, Command: "go", SpecDir: deliverySpecDir,
		RequiredReferences: []string{reference},
	}
	complete, err := promptlayer.BuildContextDelivery(expected)
	require.NoError(t, err)
	require.NoError(t, promptlayer.VerifyContextDeliveryForOptions(expected, complete))

	omitted, err := promptlayer.BuildContextDelivery(promptlayer.ContextDeliveryOptions{
		Root: root, Command: "go", SpecDir: deliverySpecDir,
	})
	require.NoError(t, err)
	require.Error(t, promptlayer.VerifyContextDeliveryForOptions(expected, omitted))
}

func TestBuildContextDelivery_ResolvesAvailableAndSelectedConditionalProfiles(t *testing.T) {
	t.Parallel()

	root := writeContextDeliveryProject(t)
	architecture := filepath.Join(root, "ARCHITECTURE.md")
	require.NoError(t, os.WriteFile(architecture, []byte("ARCHITECTURE_CONTEXT"), 0o600))
	signature := filepath.Join(root, ".autopus", "context", "signatures.md")
	require.NoError(t, os.MkdirAll(filepath.Dir(signature), 0o700))
	require.NoError(t, os.WriteFile(signature, []byte("SIGNATURE_CONTEXT"), 0o600))

	result, err := promptlayer.BuildContextDelivery(promptlayer.ContextDeliveryOptions{
		Root: root, Command: "go", SpecDir: deliverySpecDir,
		ConditionalProfiles: []promptlayer.ContextProfileName{promptlayer.ProfileSignature},
	})
	require.NoError(t, err)
	assert.Contains(t, result.Prompt, "ARCHITECTURE_CONTEXT")
	assert.Contains(t, result.Prompt, "SIGNATURE_CONTEXT")

	_, err = promptlayer.BuildContextDelivery(promptlayer.ContextDeliveryOptions{
		Root: root, Command: "go", SpecDir: deliverySpecDir,
		ConditionalProfiles: []promptlayer.ContextProfileName{promptlayer.ProfileTest},
	})
	require.Error(t, err)
}

func TestBuildContextDelivery_RequiredReferenceSetOrderIsCanonical(t *testing.T) {
	t.Parallel()

	root := writeContextDeliveryProject(t)
	for _, ref := range []string{"docs/a.md", "docs/b.md"} {
		path := filepath.Join(root, filepath.FromSlash(ref))
		require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o700))
		require.NoError(t, os.WriteFile(path, []byte(ref), 0o600))
	}
	build := func(refs []string) promptlayer.ContextDeliveryResult {
		result, err := promptlayer.BuildContextDelivery(promptlayer.ContextDeliveryOptions{
			Root: root, Command: "go", SpecDir: deliverySpecDir, RequiredReferences: refs,
		})
		require.NoError(t, err)
		return result
	}

	first := build([]string{"docs/b.md", "docs/a.md"})
	second := build([]string{"docs/a.md", "docs/b.md"})
	assert.Equal(t, first.SnapshotHash, second.SnapshotHash)
	assert.Equal(t, first.PromptManifestHash, second.PromptManifestHash)
	assert.Equal(t, first.RequiredDocuments, second.RequiredDocuments)
}
