package orchestra

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInteractivePaneBackend_PrecommitFailure_ConsumesFallbackPolicy(t *testing.T) {
	tests := []struct {
		name         string
		mode         ReliabilityFallbackMode
		wantErr      bool
		wantBackend  string
		wantTerminal string
		wantReason   string
	}{
		{
			name: "subprocess", mode: FallbackModeSubprocess,
			wantBackend: "subprocess", wantTerminal: TerminalCompleted,
			wantReason: "pane_provisioning_fallback",
		},
		{
			name: "skip", mode: FallbackModeSkip,
			wantBackend: noneBackendMarker, wantTerminal: TerminalSkipped,
			wantReason: "pane_provisioning_skipped",
		},
		{
			name: "abort", mode: FallbackModeAbort, wantErr: true,
			wantBackend: noneBackendMarker, wantTerminal: TerminalBlocked,
			wantReason: "pane_provisioning_aborted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := newCmuxMock()
			term.splitPaneErr = errors.New("split unavailable")
			provider := echoProvider("claude")
			backend := NewInteractivePaneBackend(OrchestraConfig{
				Terminal: term, FallbackMode: tt.mode,
			})

			resp, err := backend.Execute(context.Background(), ProviderRequest{
				Provider: provider.Name, Config: provider, Prompt: "contract", Timeout: 5 * time.Second,
			})

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.NotNil(t, resp)
			assert.Equal(t, tt.wantBackend, resp.ExecutedBackend)
			assert.Equal(t, tt.wantTerminal, resp.TerminalState)
			assert.Contains(t, resp.DegradedReasons, tt.wantReason)
		})
	}
}

func TestRunSubprocessPipeline_PrecommitPanePolicy_StopsWithoutProviderDispatch(t *testing.T) {
	tests := []struct {
		name         string
		mode         ReliabilityFallbackMode
		wantErr      bool
		wantTerminal string
		wantReason   string
	}{
		{
			name: "skip", mode: FallbackModeSkip,
			wantTerminal: TerminalSkipped, wantReason: "pane_provisioning_skipped",
		},
		{
			name: "abort", mode: FallbackModeAbort, wantErr: true,
			wantTerminal: TerminalBlocked, wantReason: "pane_provisioning_aborted",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			term := newCmuxMock()
			term.splitPaneErr = errors.New("split unavailable")
			provider := ProviderConfig{Name: "claude", ModelFamily: "anthropic"}
			backendCfg := OrchestraConfig{Terminal: term, FallbackMode: tt.mode}
			result, err := RunSubprocessPipeline(context.Background(), SubprocessPipelineConfig{
				Backend:   NewInteractivePaneBackend(backendCfg),
				Providers: []ProviderConfig{provider},
				PromptData: PromptData{
					ProjectName: "autopus", ProjectSummary: "contract", TechStack: "Go",
					MustReadFiles: []string{"go.mod"}, Topic: "fallback contract",
				},
				Judge: ProviderConfig{Name: "judge", ModelFamily: "independent"},
			})

			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.NotNil(t, result)
			assert.Equal(t, tt.wantTerminal, result.TerminalState)
			assert.Contains(t, result.DegradedReasons, tt.wantReason)
			assert.Zero(t, result.DispatchCount)
			require.NotNil(t, result.RunReceipt)
			assert.Empty(t, result.RunReceipt.ProviderReceipts)
		})
	}
}
