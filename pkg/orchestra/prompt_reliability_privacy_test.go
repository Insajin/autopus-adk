package orchestra

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/terminal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	privacyPromptSentinel = "PROMPT-SENTINEL-DO-NOT-PERSIST"
	privacyPromptSecret   = "sk-prompt-secret-do-not-persist"
)

type privacyPromptReceipt struct {
	TransportMode string            `json:"transport_mode"`
	Status        string            `json:"status"`
	Mismatch      string            `json:"mismatch,omitempty"`
	FailureCode   string            `json:"failure_code,omitempty"`
	Prompt        SanitizedArtifact `json:"prompt"`
}

type privacyPromptBundle struct {
	PromptReceipts []privacyPromptReceipt `json:"prompt_receipts"`
}

func TestRecoveryPromptReceiptsAndBundleExcludeRawDiagnostics(t *testing.T) {
	for _, scenario := range []struct {
		name      string
		wantCodes []string
	}{
		{name: "success", wantCodes: []string{"file_ipc_abort_failed", ""}},
		{name: "close_failure", wantCodes: []string{"file_ipc_abort_failed"}},
		{name: "replacement_failure", wantCodes: []string{"file_ipc_abort_failed"}},
		{name: "prompt_failure", wantCodes: []string{"file_ipc_abort_failed", "prompt_send_failed"}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			store, session := runPromptPrivacyRecovery(t, scenario.name)
			bundlePath := store.writeFailureBundle("privacy evidence", "inspect receipts", true)

			receipts, raw := readPromptPrivacyEvidence(t, store.artifactDir(), bundlePath)
			assertPromptPrivacy(t, receipts, scenario.wantCodes)
			assertPromptRawExcludes(t, raw,
				privacyPromptSentinel,
				privacyPromptSecret,
				"old-pane",
				"pane-1",
				session.SessionID(),
				"RAW_CLOSE_ERROR",
				"RAW_REPLACEMENT_ERROR",
				"injected prompt submit failure",
			)
		})
	}
}

func TestPromptReceiptPassOmitsPreviewAndMismatch(t *testing.T) {
	session, err := NewHookSession("privacy-pass-" + NewSessionID())
	require.NoError(t, err)
	t.Cleanup(session.Cleanup)
	session.SetHookProviders(map[string]bool{"claude": true})
	writeRoundIPCFixture(t, session, "claude", 2, "normal IPC response")

	provider := ProviderConfig{Name: "claude", Binary: "echo", InteractiveInput: "stdin"}
	term := newCmuxMock()
	term.readScreenOutput = "❯\n"
	store := &reliabilityStore{runID: "privacy-pass-" + NewSessionID(), dir: t.TempDir()}
	cfg := OrchestraConfig{
		Providers: []ProviderConfig{provider}, Strategy: StrategyDebate,
		Prompt:         privacyPromptSentinel + " " + privacyPromptSecret,
		TimeoutSeconds: 5, Terminal: term, Interactive: true, HookMode: true,
		InitialDelay: time.Millisecond, WorkingDir: t.TempDir(),
		RunID: store.runID, ReliabilityStore: store,
	}

	responses := executeRound(
		context.Background(), cfg,
		[]paneInfo{{provider: provider, paneID: "pane-1"}},
		session, 2,
		[]ProviderResponse{{Provider: "codex", Output: "round one"}},
	)

	require.Len(t, responses, 1)
	bundlePath := store.writeFailureBundle("normal IPC", "none", false)
	receipts, raw := readPromptPrivacyEvidence(t, store.artifactDir(), bundlePath)
	assertPromptPrivacy(t, receipts, []string{""})
	assertPromptRawExcludes(t, raw, privacyPromptSentinel, privacyPromptSecret, session.SessionID())
}

func TestSanitizePromptArtifactUsesUTF8ByteLength(t *testing.T) {
	const prompt = "한🐙"
	artifact := sanitizePromptArtifact(prompt)

	assert.Equal(t, len(prompt), artifact.ByteLength)
	assert.NotEmpty(t, artifact.Hash)
	assert.Empty(t, artifact.Preview)
}

func TestPromptReceiptCommonBoundaryAllowsOnlyStableFailureCodes(t *testing.T) {
	for _, test := range []struct {
		mode      string
		candidate string
		wantCode  string
	}{
		{mode: "file_ipc", candidate: "file_ipc_ready_failed", wantCode: "file_ipc_ready_failed"},
		{mode: "file_ipc", candidate: "RAW arbitrary failure", wantCode: "file_ipc_failed"},
		{mode: "prompt_ready", candidate: "RAW pane-1", wantCode: "prompt_ready_failed"},
		{mode: "send_long_text", candidate: "RAW pane-1", wantCode: "prompt_send_failed"},
		{mode: "submit_enter", candidate: "RAW pane-1", wantCode: "prompt_enter_failed"},
		{mode: "sendkeys", candidate: "RAW pane-1", wantCode: "prompt_sendkeys_failed"},
		{mode: "unknown", candidate: "RAW pane-1", wantCode: "prompt_transport_failed"},
		{mode: "sendkeys", candidate: "file_ipc_abort_failed", wantCode: "prompt_sendkeys_failed"},
		{mode: "file_ipc", candidate: "prompt_send_failed", wantCode: "file_ipc_failed"},
		{mode: "unknown", candidate: "file_ipc_ready_failed", wantCode: "prompt_transport_failed"},
	} {
		t.Run(test.mode+"_"+test.wantCode, func(t *testing.T) {
			receipt := promptReceipt(
				"run-privacy", "claude", test.mode,
				privacyPromptSentinel, 2, "failed", test.candidate,
			)
			data, err := json.Marshal(receipt)
			require.NoError(t, err)

			var persisted privacyPromptReceipt
			require.NoError(t, json.Unmarshal(data, &persisted))
			assert.Equal(t, test.wantCode, persisted.FailureCode)
			assert.Empty(t, persisted.Mismatch)
			assert.Empty(t, persisted.Prompt.Preview)
			assert.Equal(t, len(privacyPromptSentinel), persisted.Prompt.ByteLength)
			assert.NotEmpty(t, persisted.Prompt.Hash)
			assertPromptRawExcludes(t, string(data), privacyPromptSentinel)
			if test.candidate != test.wantCode {
				assertPromptRawExcludes(t, string(data), test.candidate)
			}
		})
	}
}

func TestPromptFailurePoliciesAllowEveryFallbackAndKnownCode(t *testing.T) {
	tests := []struct {
		mode  string
		codes []string
	}{
		{mode: promptTransportFileIPC, codes: []string{
			promptFailureFileIPCReady,
			promptFailureFileIPCInput,
			promptFailureFileIPCAbort,
			promptFailureFileIPCReleaseAck,
			promptFailureFileIPCFallback,
		}},
		{mode: promptTransportReady, codes: []string{promptFailureReady}},
		{mode: promptTransportSendLongText, codes: []string{promptFailureSend}},
		{mode: promptTransportSubmitEnter, codes: []string{promptFailureEnter}},
		{mode: promptTransportSendKeys, codes: []string{promptFailureSendKeys}},
		{mode: "unknown", codes: []string{promptFailureTransport}},
	}
	for _, test := range tests {
		t.Run(test.mode, func(t *testing.T) {
			policy := promptFailurePolicyForTransport(test.mode)
			assert.True(t, policy.allows(policy.fallback))
			for _, code := range test.codes {
				assert.True(t, policy.allows(code))
				assert.Equal(t, code, normalizePromptFailureCode(promptFailureStatusFailed, test.mode, code))
			}
		})
	}
}

func runPromptPrivacyRecovery(t *testing.T, scenario string) (*reliabilityStore, *HookSession) {
	t.Helper()
	base := newRound2ReleaseRecoveryTerminal()
	var term terminal.Terminal = base
	if scenario == "prompt_failure" {
		term = &round2PromptFailureTerminal{round2ReleaseRecoveryTerminal: base}
	}
	cfg, session, provider := newRound2ReleaseRecoveryFixture(t, term)
	cfg.Prompt = privacyPromptSentinel + " " + privacyPromptSecret
	store := attachRecoveryReceiptStore(t, &cfg)

	switch scenario {
	case "success":
		armSuccessfulRound2Recovery(t, base.recoveryLaunchTerminal, session, provider)
	case "close_failure":
		base.closeErr = errors.New("RAW_CLOSE_ERROR old-pane")
	case "replacement_failure":
		base.splitPaneErr = errors.New("RAW_REPLACEMENT_ERROR pane-1")
	case "prompt_failure":
		base.onLaunch = func(terminal.PaneID) {
			require.NoError(t, session.writeArtifact(
				RoundSignalName(provider.Name, 2, "ready"), nil, 0o600,
			))
		}
	default:
		t.Fatalf("unknown scenario %q", scenario)
	}

	_ = executeRound(
		context.Background(), cfg,
		[]paneInfo{{provider: provider, paneID: "old-pane"}},
		session, 2,
		[]ProviderResponse{{Provider: "codex", Output: "round one"}},
	)
	return store, session
}

func readPromptPrivacyEvidence(
	t *testing.T,
	dir string,
	bundlePath string,
) ([]privacyPromptReceipt, string) {
	t.Helper()
	paths, err := filepath.Glob(filepath.Join(dir, "prompt-*.json"))
	require.NoError(t, err)
	sort.Strings(paths)
	require.NotEmpty(t, paths)
	var raw []byte
	for _, path := range paths {
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		raw = append(raw, data...)
	}
	bundleData, err := os.ReadFile(bundlePath)
	require.NoError(t, err)
	raw = append(raw, bundleData...)
	var bundle privacyPromptBundle
	require.NoError(t, json.Unmarshal(bundleData, &bundle))
	return bundle.PromptReceipts, string(raw)
}

func assertPromptPrivacy(t *testing.T, receipts []privacyPromptReceipt, wantCodes []string) {
	t.Helper()
	require.Len(t, receipts, len(wantCodes))
	for i, receipt := range receipts {
		assert.Equal(t, wantCodes[i], receipt.FailureCode)
		assert.Empty(t, receipt.Mismatch)
		assert.Empty(t, receipt.Prompt.Preview)
		assert.Positive(t, receipt.Prompt.ByteLength)
		assert.NotEmpty(t, receipt.Prompt.Hash)
	}
}

func assertPromptRawExcludes(t *testing.T, raw string, forbidden ...string) {
	t.Helper()
	for _, value := range forbidden {
		assert.NotContains(t, raw, value)
	}
}
