package run

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

func TestFailureNormalizer_ActualRunnerTriggersMapToExactSafeReasons(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		condition desktopobserve.FailureCondition
		reason    desktopobserve.ReasonCode
		attempts  int
		leak      string
		configure func(*fakeDesktopClient, *DesktopObservationRunRequest)
	}{
		{name: "provider start", condition: desktopobserve.FailureProviderStart, reason: desktopobserve.ReasonProviderUnavailable, leak: "private provider /Users/alice/helper", configure: func(client *fakeDesktopClient, _ *DesktopObservationRunRequest) {
			client.handshakeErr = errors.New("private provider /Users/alice/helper")
		}},
		{name: "capability missing", condition: desktopobserve.FailureOperationMissing, reason: desktopobserve.ReasonCapabilityUnsupported, configure: func(client *fakeDesktopClient, _ *DesktopObservationRunRequest) {
			client.capabilities = []desktopobserve.Operation{desktopobserve.OperationCapabilities, desktopobserve.OperationListApps, desktopobserve.OperationListWindows, desktopobserve.OperationPermissions}
		}},
		{name: "accessibility denied", condition: desktopobserve.FailureAccessibilityDenied, reason: desktopobserve.ReasonAccessibilityPermissionMissing, configure: func(client *fakeDesktopClient, _ *DesktopObservationRunRequest) {
			client.accessibilityGranted = false
		}},
		{name: "app alias unmatched", condition: desktopobserve.FailureAppAliasUnmatched, reason: desktopobserve.ReasonTargetAppNotFound, leak: "raw-private-app-title", configure: func(client *fakeDesktopClient, _ *DesktopObservationRunRequest) {
			client.apps = []desktopobserve.AppSummary{{AppRef: "raw-private-app-title"}}
		}},
		{name: "window alias unmatched", condition: desktopobserve.FailureWindowAliasUnmatched, reason: desktopobserve.ReasonTargetWindowNotFound, leak: "raw-private-window-title", configure: func(client *fakeDesktopClient, _ *DesktopObservationRunRequest) {
			client.windows = []desktopobserve.WindowSummary{{WindowRef: "raw-private-window-title"}}
		}},
		{name: "consumed state", condition: desktopobserve.FailureStateRefRejected, reason: desktopobserve.ReasonStaleState, attempts: 2, configure: func(_ *fakeDesktopClient, _ *DesktopObservationRunRequest) {}},
		{name: "minimum landmarks", condition: desktopobserve.FailureLandmarksInsufficient, reason: desktopobserve.ReasonSemanticProjectionUnavailable, configure: func(client *fakeDesktopClient, _ *DesktopObservationRunRequest) {
			projection := runnerSemanticFixture("provider-local", "state-canvas")
			projection.Root = desktopobserve.SemanticNode{Role: desktopobserve.RoleCanvas}
			client.projection = &projection
		}},
		{name: "redactor error", condition: desktopobserve.FailureRedaction, reason: desktopobserve.ReasonRedactionFailed, leak: "private redactor secret /tmp/raw", configure: func(_ *fakeDesktopClient, request *DesktopObservationRunRequest) {
			request.Redactor = func(string) (string, error) { return "", errors.New("private redactor secret /tmp/raw") }
		}},
		{name: "raw only quarantine", condition: desktopobserve.FailureRawOnlyQuarantine, reason: desktopobserve.ReasonEvidenceQuarantined, leak: "raw tree handle=0x42 /private/var/raw", configure: func(client *fakeDesktopClient, _ *DesktopObservationRunRequest) {
			client.getStateErr = fmt.Errorf("raw tree handle=0x42 /private/var/raw: %w", desktopobserve.ErrRawOnlyEvidence)
		}},
		{name: "protocol version", condition: desktopobserve.FailureProtocolVersion, reason: desktopobserve.ReasonProviderProtocolMismatch, configure: func(client *fakeDesktopClient, _ *DesktopObservationRunRequest) {
			client.protocolVersion = desktopobserve.ProtocolVersion + 1
		}},
	}

	emitted := make([]desktopobserve.ReasonCode, 0, len(tests))
	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			local := newFakeDesktopClient(desktopobserve.RuntimeProviderLocal)
			alternate := newFakeDesktopClient(desktopobserve.RuntimeProviderOrca)
			request := desktopRunRequest(desktopobserve.RuntimeProviderLocal)
			test.configure(local, &request)
			runner := newDesktopObservationRunner(local, alternate)
			attempts := test.attempts
			if attempts == 0 {
				attempts = 1
			}
			var outcome desktopobserve.OracleOutcome
			for attempt := 0; attempt < attempts; attempt++ {
				var err error
				outcome, err = runner.Run(context.Background(), request)
				require.NoError(t, err)
				if attempt+1 < attempts {
					assert.Equal(t, desktopobserve.VerdictPassed, outcome.Verdict)
				}
			}

			assert.Equal(t, test.condition, outcome.FailureCondition)
			require.NotNil(t, outcome.ReasonCode)
			assert.Equal(t, test.reason, *outcome.ReasonCode)
			emitted = append(emitted, *outcome.ReasonCode)
			assert.NotEqual(t, desktopobserve.VerdictPassed, outcome.Verdict)
			assert.Nil(t, outcome.SemanticProjection)
			assert.Zero(t, passedDesktopCheckCount(outcome.DeterministicChecks))
			assert.Empty(t, alternate.calls)
			require.NoError(t, outcome.RuntimeReceipt.Validate())
			body, err := json.Marshal(outcome)
			require.NoError(t, err)
			assert.False(t, unsafeActualFailureOutput.Match(body), string(body))
			if test.leak != "" {
				assert.NotContains(t, string(body), test.leak)
			}
			assert.NotContains(t, string(body), "failure_condition")
		})
	}
	assert.Equal(t, desktopobserve.ReasonCodes(), emitted)
}

var unsafeActualFailureOutput = regexp.MustCompile(`(?i)(raw[_ -]?tree|0x[0-9a-f]+|/Users/|/tmp/|/private/var/|handle[=:]|provider error|redactor secret)`)
