package orchestra

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type persistedRecoveryTransition struct {
	SchemaVersion string         `json:"schema_version"`
	Timestamp     time.Time      `json:"timestamp"`
	Correlation   CorrelationIDs `json:"correlation"`
	Provider      string         `json:"provider"`
	Sequence      uint64         `json:"sequence"`
	Stage         string         `json:"stage"`
	Status        string         `json:"status"`
	FailureCode   string         `json:"failure_code,omitempty"`
}

type persistedRecoveryBundle struct {
	RecoveryTransitions []persistedRecoveryTransition `json:"recovery_transitions"`
	Events              []ReliabilityEvent            `json:"events"`
	Degraded            bool                          `json:"degraded"`
}

type round2PromptFailureTerminal struct {
	*round2ReleaseRecoveryTerminal
	mu            sync.Mutex
	longTextCalls int
}

func (m *round2PromptFailureTerminal) SendLongText(
	ctx context.Context,
	paneID terminal.PaneID,
	text string,
) error {
	m.mu.Lock()
	m.longTextCalls++
	call := m.longTextCalls
	m.mu.Unlock()
	if call == 2 {
		m.recordOrder("long_text:" + string(paneID))
		return errors.New("injected prompt submit failure for pane-1")
	}
	return m.round2ReleaseRecoveryTerminal.SendLongText(ctx, paneID, text)
}

func TestExecuteRound_RecoveryLifecyclePersistsOrderedTransitions(t *testing.T) {
	term := newRound2ReleaseRecoveryTerminal()
	cfg, session, provider := newRound2ReleaseRecoveryFixture(t, term)
	store := attachRecoveryReceiptStore(t, &cfg)
	armSuccessfulRound2Recovery(t, term.recoveryLaunchTerminal, session, provider)

	responses := executeRound(
		context.Background(), cfg,
		[]paneInfo{{provider: provider, paneID: "old-pane"}},
		session, 2,
		[]ProviderResponse{{Provider: "codex", Output: "round one"}},
	)

	require.Len(t, responses, 1)
	assert.Equal(t, "replacement response", responses[0].Output)
	transitions, raw := readRecoveryTransitions(t, store.artifactDir())
	assertRecoveryTransitions(t, transitions,
		[]string{"owner_closed", "replacement_ready", "prompt_submitted"},
		[]string{"pass", "pass", "pass"},
		[]string{"", "", ""},
	)
	assert.NotContains(t, raw, "old-pane")
	assert.NotContains(t, raw, "pane-1")
	assert.NotContains(t, raw, cfg.Prompt)
	assert.NotContains(t, raw, session.SessionID())

	bundlePath := store.writeFailureBundle("recovery complete", "inspect receipts", false)
	var bundle persistedRecoveryBundle
	readJSONFile(t, bundlePath, &bundle)
	assert.Equal(t, transitions, bundle.RecoveryTransitions)
	assert.Empty(t, bundle.Events)
	assert.Zero(t, store.summary(bundlePath).OpenEvents)
}

func TestExecuteRound_RecoveryCloseFailurePersistsOnlyFailure(t *testing.T) {
	term := newRound2ReleaseRecoveryTerminal()
	term.closeErr = errors.New("injected close failure for old-pane")
	cfg, session, provider := newRound2ReleaseRecoveryFixture(t, term)
	store := attachRecoveryReceiptStore(t, &cfg)

	responses := executeRound(
		context.Background(), cfg,
		[]paneInfo{{provider: provider, paneID: "old-pane"}},
		session, 2,
		[]ProviderResponse{{Provider: "codex", Output: "round one"}},
	)

	require.Len(t, responses, 1)
	assert.Equal(t, skippedHookCollectionError, responses[0].Error)
	transitions, raw := readRecoveryTransitions(t, store.artifactDir())
	assertRecoveryTransitions(t, transitions,
		[]string{"owner_closed"}, []string{"failed"}, []string{"owner_close_failed"},
	)
	assert.NotContains(t, raw, "old-pane")
}

func TestExecuteRound_RecoveryReplacementFailureOmitsPromptTransition(t *testing.T) {
	term := newRound2ReleaseRecoveryTerminal()
	term.splitPaneErr = errors.New("injected replacement failure for pane-1")
	cfg, session, provider := newRound2ReleaseRecoveryFixture(t, term)
	store := attachRecoveryReceiptStore(t, &cfg)

	responses := executeRound(
		context.Background(), cfg,
		[]paneInfo{{provider: provider, paneID: "old-pane"}},
		session, 2,
		[]ProviderResponse{{Provider: "codex", Output: "round one"}},
	)

	require.Len(t, responses, 1)
	assert.Equal(t, skippedHookCollectionError, responses[0].Error)
	transitions, raw := readRecoveryTransitions(t, store.artifactDir())
	assertRecoveryTransitions(t, transitions,
		[]string{"owner_closed", "replacement_ready"},
		[]string{"pass", "failed"},
		[]string{"", "replacement_ready_failed"},
	)
	assert.NotContains(t, raw, "pane-1")
	bundlePath := store.writeFailureBundle("replacement failed", "inspect receipts", true)
	var bundle persistedRecoveryBundle
	readJSONFile(t, bundlePath, &bundle)
	assert.True(t, bundle.Degraded)
	assert.Equal(t, transitions, bundle.RecoveryTransitions)
	assert.Empty(t, bundle.Events)
}

func TestExecuteRound_RecoveryPromptFailurePersistsFailureWithoutSuccess(t *testing.T) {
	base := newRound2ReleaseRecoveryTerminal()
	term := &round2PromptFailureTerminal{round2ReleaseRecoveryTerminal: base}
	cfg, session, provider := newRound2ReleaseRecoveryFixture(t, term)
	store := attachRecoveryReceiptStore(t, &cfg)
	term.onLaunch = func(terminal.PaneID) {
		require.NoError(t, session.writeArtifact(
			RoundSignalName(provider.Name, 2, "ready"), nil, 0o600,
		))
	}

	responses := executeRound(
		context.Background(), cfg,
		[]paneInfo{{provider: provider, paneID: "old-pane"}},
		session, 2,
		[]ProviderResponse{{Provider: "codex", Output: "round one"}},
	)

	require.Len(t, responses, 1)
	assert.Equal(t, skippedHookCollectionError, responses[0].Error)
	transitions, raw := readRecoveryTransitions(t, store.artifactDir())
	assertRecoveryTransitions(t, transitions,
		[]string{"owner_closed", "replacement_ready", "prompt_submitted"},
		[]string{"pass", "pass", "failed"},
		[]string{"", "", "prompt_submit_failed"},
	)
	assert.NotContains(t, raw, "injected prompt submit failure")
	assert.NotContains(t, raw, "pane-1")
}

func TestReliabilityStore_RecoveryPersistenceFailureWarnsOnceWithoutOpenEvent(t *testing.T) {
	parent := filepath.Join(t.TempDir(), "not-a-dir")
	require.NoError(t, os.WriteFile(parent, []byte("x"), 0o600))
	store := &reliabilityStore{
		runID: "recovery-persistence-failure",
		dir:   filepath.Join(parent, "receipts"),
	}

	var first, second string
	logged := captureLog(t, func() {
		first = store.recordRecoveryTransition(
			"claude", 2, 1, "owner_closed", "pass", "",
		)
		second = store.recordRecoveryTransition(
			"claude", 2, 1, "replacement_ready", "pass", "",
		)
	})

	assert.Empty(t, first)
	assert.Empty(t, second)
	assert.True(t, store.degraded)
	assert.Equal(t, 1, strings.Count(logged, "reliability"))
	assert.Len(t, store.recovery, 2)
	assert.Empty(t, store.events)
	assert.Zero(t, store.summary("").OpenEvents)
}

func attachRecoveryReceiptStore(t *testing.T, cfg *OrchestraConfig) *reliabilityStore {
	t.Helper()
	runID := "round2-recovery-receipt-" + NewSessionID()
	store := &reliabilityStore{runID: runID, dir: t.TempDir()}
	cfg.RunID = runID
	cfg.ReliabilityStore = store
	return store
}

func armSuccessfulRound2Recovery(
	t *testing.T,
	term *recoveryLaunchTerminal,
	session *HookSession,
	provider ProviderConfig,
) {
	t.Helper()
	longTextCount := 0
	term.onLaunch = func(terminal.PaneID) {
		longTextCount++
		if longTextCount == 1 {
			require.NoError(t, session.writeArtifact(
				RoundSignalName(provider.Name, 2, "ready"), nil, 0o600,
			))
			return
		}
		require.NoError(t, session.writeJSONArtifact(
			RoundSignalName(provider.Name, 2, "result.json"),
			HookResult{Output: "replacement response"},
		))
		require.NoError(t, session.writeArtifact(
			RoundSignalName(provider.Name, 2, "done"), nil, 0o600,
		))
	}
}

func readRecoveryTransitions(
	t *testing.T,
	dir string,
) ([]persistedRecoveryTransition, string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "recovery-*.json"))
	require.NoError(t, err)
	sort.Strings(paths)
	require.NotEmpty(t, paths, "recovery lifecycle artifacts must be persisted")
	transitions := make([]persistedRecoveryTransition, 0, len(paths))
	var raw strings.Builder
	for i, path := range paths {
		assert.Contains(t, filepath.Base(path), fmt.Sprintf("recovery-%06d-", i+1))
		var transition persistedRecoveryTransition
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		require.NoError(t, json.Unmarshal(data, &transition))
		transitions = append(transitions, transition)
		raw.Write(data)
	}
	return transitions, raw.String()
}

func assertRecoveryTransitions(
	t *testing.T,
	transitions []persistedRecoveryTransition,
	stages []string,
	statuses []string,
	failureCodes []string,
) {
	t.Helper()
	require.Len(t, transitions, len(stages))
	for i, transition := range transitions {
		assert.Equal(t, uint64(i+1), transition.Sequence)
		assert.False(t, transition.Timestamp.IsZero())
		assert.Equal(t, stages[i], transition.Stage)
		assert.Equal(t, statuses[i], transition.Status)
		assert.Equal(t, failureCodes[i], transition.FailureCode)
		assert.NotEmpty(t, transition.Correlation.RunID)
		assert.Equal(t, "claude", transition.Provider)
		assert.Equal(t, transition.Provider, transition.Correlation.ProviderID)
		assert.Equal(t, "round-2", transition.Correlation.RoundID)
		assert.Equal(t, "attempt-1", transition.Correlation.AttemptID)
	}
}

func readJSONFile(t *testing.T, path string, target any) {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.NoError(t, json.Unmarshal(data, target))
}
