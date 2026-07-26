package orchestra

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/terminal"
)

// FreshJudgeSessionEvidence records the observable logical session boundary
// without serializing participant or judge session identifiers. Isolated does
// not assert OS-level process, filesystem, or network sandboxing.
// @AX:ANCHOR: [AUTO] public JSON evidence contract for proving participant-to-judge session separation
// @AX:REASON: [AUTO] run receipts and external diagnostics depend on stable redacted fingerprints and verification semantics
type FreshJudgeSessionEvidence struct {
	Required                      bool   `json:"required"`
	Isolated                      bool   `json:"isolated"`
	Verified                      bool   `json:"verified"`
	Mechanism                     string `json:"mechanism"`
	ParticipantsTerminated        bool   `json:"participants_terminated"`
	ParticipantSessionFingerprint string `json:"participant_session_fingerprint"`
	JudgeSessionFingerprint       string `json:"judge_session_fingerprint"`
	Reason                        string `json:"reason"`
}

func newFreshJudgeSessionEvidence(cfg OrchestraConfig, participantHook *HookSession) *FreshJudgeSessionEvidence {
	mechanism := "fresh_backend_execution"
	participantSessionID := ""
	if cfg.HookMode {
		mechanism = "isolated_hook_session"
		participantSessionID = cfg.SessionID
		if participantHook != nil {
			participantSessionID = participantHook.SessionID()
		}
	}
	return &FreshJudgeSessionEvidence{
		Required:                      true,
		Mechanism:                     mechanism,
		ParticipantSessionFingerprint: fingerprintSessionID(participantSessionID),
	}
}

func newFreshSubprocessJudgeSessionEvidence() *FreshJudgeSessionEvidence {
	return &FreshJudgeSessionEvidence{
		Required:               true,
		Mechanism:              "fresh_backend_execution",
		ParticipantsTerminated: true,
		Reason:                 "fresh subprocess judge execution pending verification",
	}
}

func verifyFreshSubprocessJudgeSession(
	evidence *FreshJudgeSessionEvidence,
	response *ProviderResponse,
) {
	if evidence == nil {
		return
	}
	if response == nil {
		evidence.Reason = "fresh subprocess judge execution returned no response"
		return
	}
	if response.ExecutedBackend != "subprocess" {
		evidence.Reason = fmt.Sprintf(
			"fresh subprocess judge backend was not observed: %q",
			response.ExecutedBackend,
		)
		return
	}
	evidence.Isolated = true
	evidence.Verified = true
	evidence.Reason = "fresh subprocess backend execution verified"
}

func freshJudgeSessionFromResponses(responses []ProviderResponse) *FreshJudgeSessionEvidence {
	for i := len(responses) - 1; i >= 0; i-- {
		if responses[i].freshJudgeSession != nil {
			return responses[i].freshJudgeSession
		}
	}
	return nil
}

// @AX:NOTE: [AUTO] judge Args and PaneArgs fail closed on provider-specific resume, continue, session, thread, or conversation tokens
func freshJudgeConfigError(provider ProviderConfig) error {
	identity := providerCanonicalName(provider.Name)
	if binaryIdentity := providerCanonicalName(filepath.Base(strings.TrimSpace(provider.Binary))); identity == "" {
		identity = binaryIdentity
	} else if !knownProviderIdentity(identity) && knownProviderIdentity(binaryIdentity) {
		identity = binaryIdentity
	}
	for _, args := range [][]string{provider.Args, provider.PaneArgs} {
		for _, arg := range args {
			if flag, blocked := freshJudgeResumeToken(identity, arg); blocked {
				return fmt.Errorf(
					"judge provider %q cannot prove a fresh session while resume/continue option %q is configured",
					provider.Name,
					flag,
				)
			}
		}
	}
	return nil
}

func freshJudgeResumeToken(identity, arg string) (string, bool) {
	token := strings.ToLower(strings.TrimSpace(arg))
	if token == "" {
		return "", false
	}
	switch identity {
	case "claude":
		if token == "-c" || token == "-r" {
			return token, true
		}
	case "gemini", "opencode":
		if token == "-c" || token == "-s" {
			return token, true
		}
	}

	normalized := strings.TrimLeft(token, "-")
	if index := strings.IndexByte(normalized, '='); index >= 0 {
		normalized = normalized[:index]
	}
	switch normalized {
	case "resume", "continue",
		"session", "session-id",
		"thread", "thread-id",
		"chat", "chat-id",
		"conversation", "conversation-id",
		"from-pr", "fork-session":
		return "--" + normalized, true
	default:
		return "", false
	}
}

func knownProviderIdentity(identity string) bool {
	switch identity {
	case "claude", "codex", "gemini", "opencode":
		return true
	default:
		return false
	}
}

func freshJudgeSessionError(evidence *FreshJudgeSessionEvidence) error {
	switch {
	case evidence == nil:
		return fmt.Errorf("evidence is missing")
	case !evidence.Required:
		return fmt.Errorf("evidence is not marked required")
	case !evidence.ParticipantsTerminated:
		return fmt.Errorf("participant termination is not verified")
	case !evidence.Isolated:
		return fmt.Errorf("judge execution isolation is not verified")
	case !evidence.Verified:
		return fmt.Errorf("fresh judge execution is not verified")
	default:
		return nil
	}
}

// @AX:WARN: [AUTO] high-branch terminal teardown — pane-close failures block judge dispatch; pipe-stop failures remain diagnostic
// @AX:REASON: [AUTO] eight conditional branches coordinate monitoring, best-effort pipe stop, fail-closed pane ownership, and joined close failures
func terminateParticipantsBeforeJudge(
	cfg OrchestraConfig,
	panes []paneInfo,
	participantHook *HookSession,
) error {
	if cfg.SurfaceMgr != nil {
		cfg.SurfaceMgr.Stop()
	}
	if participantHook != nil {
		defer participantHook.Cleanup()
	}
	if len(panes) == 0 {
		return nil
	}
	if cfg.Terminal == nil {
		return fmt.Errorf("participant pane termination failed: terminal unavailable")
	}

	var failures []error
	for i := range panes {
		paneID := panes[i].paneID
		if paneID == "" {
			continue
		}
		if err := cfg.Terminal.PipePaneStop(context.Background(), paneID); err != nil {
			log.Printf("[judge] participant pipe stop failed for %q (non-fatal): %v", paneID, err)
		}
		if closePaneSurface(cfg.Terminal, paneID) {
			panes[i].paneID = terminal.PaneID("")
		} else {
			failures = append(failures, fmt.Errorf("close participant pane %q", paneID))
		}
	}
	if err := errors.Join(failures...); err != nil {
		return fmt.Errorf("participant pane termination failed: %w", err)
	}
	return nil
}

func activateFreshJudgeHookSession(evidence *FreshJudgeSessionEvidence) (*HookSession, error) {
	if evidence.ParticipantSessionFingerprint == "" {
		return nil, fmt.Errorf("participant hook session unavailable for fingerprint verification")
	}
	judgeSessionID := NewSessionID()
	judgeHookSession, err := NewHookSession(judgeSessionID)
	if err != nil {
		return nil, fmt.Errorf("%s", strings.ReplaceAll(err.Error(), judgeSessionID, "[redacted]"))
	}
	evidence.JudgeSessionFingerprint = fingerprintSessionID(judgeHookSession.SessionID())
	if evidence.ParticipantSessionFingerprint == evidence.JudgeSessionFingerprint {
		judgeHookSession.Cleanup()
		return nil, fmt.Errorf("judge hook session fingerprint matches participant session")
	}
	evidence.Isolated = true
	evidence.Verified = true
	evidence.Reason = "distinct hook session fingerprints verified"
	return judgeHookSession, nil
}

func fingerprintSessionID(sessionID string) string {
	if sessionID == "" {
		return ""
	}
	sum := sha256.Sum256([]byte(sessionID))
	return fmt.Sprintf("sha256:%x", sum)
}
