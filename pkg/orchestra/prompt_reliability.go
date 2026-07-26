package orchestra

// @AX:NOTE: [AUTO] each failed receipt persists one stable code: allowlist by transport, otherwise policy fallback; never raw errors
const (
	promptFailureFileIPCReady      = "file_ipc_ready_failed"
	promptFailureFileIPCInput      = "file_ipc_input_failed"
	promptFailureFileIPCAbort      = "file_ipc_abort_failed"
	promptFailureFileIPCReleaseAck = "file_ipc_release_ack_failed"
	promptFailureFileIPCFallback   = "file_ipc_failed"
	promptFailureReady             = "prompt_ready_failed"
	promptFailureSend              = "prompt_send_failed"
	promptFailureEnter             = "prompt_enter_failed"
	promptFailureSendKeys          = "prompt_sendkeys_failed"
	promptFailureTransport         = "prompt_transport_failed"
	promptFailureStatusFailed      = "failed"
	promptTransportFileIPC         = "file_ipc"
	promptTransportReady           = "prompt_ready"
	promptTransportSendLongText    = "send_long_text"
	promptTransportSubmitEnter     = "submit_enter"
	promptTransportSendKeys        = "sendkeys"
)

type promptFailurePolicy struct {
	fallback string
	allowed  []string
}

func sanitizePromptArtifact(prompt string) SanitizedArtifact {
	return SanitizedArtifact{
		ByteLength: len(prompt),
		Hash:       hashString(prompt),
	}
}

func normalizePromptFailureCode(status, transportMode, candidate string) string {
	if status != promptFailureStatusFailed {
		return ""
	}
	policy := promptFailurePolicyForTransport(transportMode)
	if policy.allows(candidate) {
		return candidate
	}
	return policy.fallback
}

func promptFailurePolicyForTransport(transportMode string) promptFailurePolicy {
	switch transportMode {
	case promptTransportFileIPC:
		return promptFailurePolicy{
			fallback: promptFailureFileIPCFallback,
			allowed: []string{
				promptFailureFileIPCReady,
				promptFailureFileIPCInput,
				promptFailureFileIPCAbort,
				promptFailureFileIPCReleaseAck,
				promptFailureFileIPCFallback,
			},
		}
	case promptTransportReady:
		return promptFailurePolicy{fallback: promptFailureReady, allowed: []string{promptFailureReady}}
	case promptTransportSendLongText:
		return promptFailurePolicy{fallback: promptFailureSend, allowed: []string{promptFailureSend}}
	case promptTransportSubmitEnter:
		return promptFailurePolicy{fallback: promptFailureEnter, allowed: []string{promptFailureEnter}}
	case promptTransportSendKeys:
		return promptFailurePolicy{fallback: promptFailureSendKeys, allowed: []string{promptFailureSendKeys}}
	default:
		return promptFailurePolicy{fallback: promptFailureTransport, allowed: []string{promptFailureTransport}}
	}
}

func (policy promptFailurePolicy) allows(candidate string) bool {
	for _, allowed := range policy.allowed {
		if candidate == allowed {
			return true
		}
	}
	return false
}
