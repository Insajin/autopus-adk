package desktopobserve

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestStateRefOneUse_ConsumedAndSupersededRefsAreStale(t *testing.T) {
	t.Parallel()

	ledger := NewStateLedger()
	first := stateBinding("state-1", "provider-local", "digest-1")
	require.NoError(t, ledger.Register(first))
	require.NoError(t, ledger.Consume(first))
	assert.Equal(t, ReasonStaleState, ReasonCodeOf(ledger.Consume(first)))

	second := stateBinding("state-2", "provider-local", "digest-2")
	third := stateBinding("state-3", "provider-local", "digest-3")
	require.NoError(t, ledger.Register(second))
	require.NoError(t, ledger.Register(third))
	assert.Equal(t, ReasonStaleState, ReasonCodeOf(ledger.Consume(second)))
	require.NoError(t, ledger.Consume(third))
}

func TestStateRefOneUse_ConcurrentConsumeHasExactlyOneWinner(t *testing.T) {
	t.Parallel()

	ledger := NewStateLedger()
	binding := stateBinding("state-1", "provider-local", "digest-1")
	require.NoError(t, ledger.Register(binding))
	var passed atomic.Int32
	var stale atomic.Int32
	var group sync.WaitGroup
	for range 32 {
		group.Add(1)
		go func() {
			defer group.Done()
			if err := ledger.Consume(binding); err == nil {
				passed.Add(1)
			} else if ReasonCodeOf(err) == ReasonStaleState {
				stale.Add(1)
			}
		}()
	}
	group.Wait()
	assert.Equal(t, int32(1), passed.Load())
	assert.Equal(t, int32(31), stale.Load())
}

func TestStateRefOneUse_ScopeAndProviderMismatchAreStale(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		mutate func(*StateBinding)
	}{
		{name: "provider", mutate: func(binding *StateBinding) { binding.ProviderRef = "provider-orca" }},
		{name: "application", mutate: func(binding *StateBinding) { binding.AppRef = "other-app" }},
		{name: "window", mutate: func(binding *StateBinding) { binding.WindowRef = "other-window" }},
		{name: "digest", mutate: func(binding *StateBinding) { binding.Digest = "other-digest" }},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			ledger := NewStateLedger()
			registered := stateBinding("state-1", "provider-local", "digest-1")
			require.NoError(t, ledger.Register(registered))
			candidate := registered
			test.mutate(&candidate)
			assert.Equal(t, ReasonStaleState, ReasonCodeOf(ledger.Consume(candidate)))
		})
	}
}

func TestStateRefRegister_DuplicateLiveAndConsumedReuseAreRejected(t *testing.T) {
	t.Parallel()

	t.Run("duplicate live ref", func(t *testing.T) {
		t.Parallel()
		ledger := NewStateLedger()
		binding := stateBinding("state-1", "provider-local", "digest-1")
		require.NoError(t, ledger.Register(binding))
		assert.Equal(t, ReasonStaleState, ReasonCodeOf(ledger.Register(binding)))
		require.NoError(t, ledger.Consume(binding), "rejected duplicate must not corrupt the live ref")
	})

	t.Run("consumed ref reuse", func(t *testing.T) {
		t.Parallel()
		ledger := NewStateLedger()
		binding := stateBinding("state-1", "provider-local", "digest-1")
		require.NoError(t, ledger.Register(binding))
		require.NoError(t, ledger.Consume(binding))
		assert.Equal(t, ReasonStaleState, ReasonCodeOf(ledger.Register(binding)))
	})

	t.Run("same ref different scope", func(t *testing.T) {
		t.Parallel()
		ledger := NewStateLedger()
		binding := stateBinding("state-1", "provider-local", "digest-1")
		require.NoError(t, ledger.Register(binding))
		mismatch := binding
		mismatch.WindowRef = "other-window"
		assert.Equal(t, ReasonStaleState, ReasonCodeOf(ledger.Register(mismatch)))
		require.NoError(t, ledger.Consume(binding), "scope mismatch must not replace the live binding")
	})
}

func stateBinding(stateRef, providerRef, digest string) StateBinding {
	return StateBinding{
		StateRef:    stateRef,
		ProviderRef: providerRef,
		AppRef:      "autopus-desktop",
		WindowRef:   "main-window",
		Digest:      digest,
	}
}
