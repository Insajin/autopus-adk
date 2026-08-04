package promptlayer

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOMPContextTransientStore_CheckpointRehydrate_RestoresAndRemovesBodies(t *testing.T) {
	t.Parallel()

	_, opts, delivery := buildOMPContextFixture(t)
	input := validOMPContextBindingInput(opts, delivery)
	store := NewOMPContextTransientStore()
	binding, err := store.Checkpoint(input)
	require.NoError(t, err)
	assert.Equal(t, 1, store.Pending())
	stored := store.entries[binding.BindingHash]
	require.NotNil(t, stored)
	retainedBytes := make([][]byte, 0, len(stored.bodies.values))
	for _, body := range stored.bodies.values {
		retainedBytes = append(retainedBytes, body)
	}
	// The caller's receipt must not alias the store's integrity baseline.
	binding.FullDocumentRefs[0].SourceHash = canonicalHash([]byte("caller mutation"))

	called := 0
	var retainedView OMPContextTransientView
	receipt, err := store.Rehydrate(binding.BindingHash, opts, func(view OMPContextTransientView) error {
		called++
		retainedView = view
		assert.Equal(t, input.Ephemeral.OriginalTask, view.OriginalTask())
		assert.Equal(t, input.Ephemeral.DecisionDelta, view.DecisionDelta())
		assert.Equal(t, input.Ephemeral.FrozenFindingIDs, view.FrozenFindingIDs())
		assert.Equal(t, input.Ephemeral.OwnershipPaths, view.OwnershipPaths())
		assert.Equal(t, input.Ephemeral.ForbiddenPaths, view.ForbiddenPaths())
		assert.Equal(t, OMPWorkerResultSchema(), view.WorkerResultSchema())
		_, marshalErr := json.Marshal(view)
		assert.ErrorIs(t, marshalErr, ErrOMPContextBodySerialization)
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 1, called)
	assert.True(t, receipt.ExactMatch)
	assert.True(t, receipt.Admission)
	assert.Equal(t, "verified", receipt.Reason)
	assert.Equal(t, 0, store.Pending())
	assert.NotEqual(t, input.Ephemeral.OriginalTask, retainedView.OriginalTask(), "view must expire after callback")
	for _, body := range retainedBytes {
		for _, value := range body {
			assert.Zero(t, value, "transient bytes must be zeroized")
		}
	}

	raw, err := json.Marshal(receipt)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), input.Ephemeral.OriginalTask)
	assert.NotContains(t, string(raw), opts.Root)
}

func TestOMPContextTransientStore_RehydrateMismatch_FailsClosedAndRemovesState(t *testing.T) {
	t.Parallel()

	root, opts, delivery := buildOMPContextFixture(t)
	store := NewOMPContextTransientStore()
	binding, err := store.Checkpoint(validOMPContextBindingInput(opts, delivery))
	require.NoError(t, err)

	path := filepath.Join(root, filepath.FromSlash(ompContextSpecDir), "acceptance.md")
	require.NoError(t, os.WriteFile(path, []byte("changed acceptance"), 0o600))
	called := 0
	receipt, err := store.Rehydrate(binding.BindingHash, opts, func(OMPContextTransientView) error {
		called++
		return nil
	})
	require.Error(t, err)
	assert.False(t, receipt.ExactMatch)
	assert.False(t, receipt.Admission)
	assert.Equal(t, "required-context-mismatch", receipt.Reason)
	assert.Equal(t, 0, called, "consumer/provider admission must not occur")
	assert.Equal(t, 0, store.Pending())
}

func TestOMPContextTransientStore_Abort_ZeroizesAndRemovesState(t *testing.T) {
	t.Parallel()

	_, opts, delivery := buildOMPContextFixture(t)
	store := NewOMPContextTransientStore()
	binding, err := store.Checkpoint(validOMPContextBindingInput(opts, delivery))
	require.NoError(t, err)

	receipt, err := store.Abort(binding.BindingHash, "operator-abort")
	require.NoError(t, err)
	assert.False(t, receipt.ExactMatch)
	assert.False(t, receipt.Admission)
	assert.Equal(t, "operator-abort", receipt.Reason)
	assert.Equal(t, 0, store.Pending())

	_, err = store.Abort(binding.BindingHash, "operator-abort")
	require.ErrorIs(t, err, ErrOMPContextBindingUnavailable)
}

func TestOMPContextTransientStore_ConcurrentRehydrate_AdmitsExactlyOnce(t *testing.T) {
	t.Parallel()

	_, opts, delivery := buildOMPContextFixture(t)
	store := NewOMPContextTransientStore()
	binding, err := store.Checkpoint(validOMPContextBindingInput(opts, delivery))
	require.NoError(t, err)

	var wg sync.WaitGroup
	results := make(chan error, 2)
	for range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, runErr := store.Rehydrate(binding.BindingHash, opts, func(OMPContextTransientView) error { return nil })
			results <- runErr
		}()
	}
	wg.Wait()
	close(results)

	successes, unavailable := 0, 0
	for runErr := range results {
		switch {
		case runErr == nil:
			successes++
		case errors.Is(runErr, ErrOMPContextBindingUnavailable):
			unavailable++
		default:
			t.Fatalf("unexpected error: %v", runErr)
		}
	}
	assert.Equal(t, 1, successes)
	assert.Equal(t, 1, unavailable)
	assert.Equal(t, 0, store.Pending())
}

func TestOMPContextTransientStore_EphemeralTamper_FailsBeforeConsumer(t *testing.T) {
	t.Parallel()

	_, opts, delivery := buildOMPContextFixture(t)
	store := NewOMPContextTransientStore()
	binding, err := store.Checkpoint(validOMPContextBindingInput(opts, delivery))
	require.NoError(t, err)
	store.entries[binding.BindingHash].bodies.values["original_task"][0] ^= 0xff

	called := false
	receipt, err := store.Rehydrate(binding.BindingHash, opts, func(OMPContextTransientView) error {
		called = true
		return nil
	})
	require.Error(t, err)
	assert.False(t, called)
	assert.False(t, receipt.ExactMatch)
	assert.False(t, receipt.Admission)
	assert.Equal(t, "ephemeral-context-mismatch", receipt.Reason)
	assert.Equal(t, 0, store.Pending())
}

func TestOMPContextTransientStore_ConsumerFailure_DeniesAdmissionAfterExactMatch(t *testing.T) {
	t.Parallel()

	_, opts, delivery := buildOMPContextFixture(t)
	store := NewOMPContextTransientStore()
	binding, err := store.Checkpoint(validOMPContextBindingInput(opts, delivery))
	require.NoError(t, err)

	receipt, err := store.Rehydrate(binding.BindingHash, opts, func(OMPContextTransientView) error {
		return errors.New("consumer refused")
	})
	require.Error(t, err)
	assert.True(t, receipt.ExactMatch)
	assert.False(t, receipt.Admission)
	assert.Equal(t, "rehydration-consumer-rejected", receipt.Reason)
	assert.Equal(t, 0, store.Pending())
}

func TestOMPContextTransientStore_DuplicateAndNilStore_AreRejected(t *testing.T) {
	t.Parallel()

	_, opts, delivery := buildOMPContextFixture(t)
	input := validOMPContextBindingInput(opts, delivery)
	store := NewOMPContextTransientStore()
	binding, err := store.Checkpoint(input)
	require.NoError(t, err)
	_, err = store.Checkpoint(input)
	require.Error(t, err)
	_, err = store.Rehydrate(binding.BindingHash, opts, nil)
	require.Error(t, err)

	var nilStore *OMPContextTransientStore
	_, err = nilStore.Checkpoint(input)
	require.Error(t, err)
	_, err = nilStore.Rehydrate(binding.BindingHash, opts, nil)
	require.ErrorIs(t, err, ErrOMPContextBindingUnavailable)
	assert.Zero(t, nilStore.Pending())
	assert.Empty(t, OMPContextTransientView{}.OriginalTask())
	(*ompContextBodies)(nil).zeroize()
}
