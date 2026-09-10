package cli

import (
	"encoding/json"
	"sort"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/ompprobe"
)

// pipelineOMPActiveProbeAttempt accumulates one manual compaction attempt,
// including the ones OMP declines. A declined or failed attempt is a
// measurement, not a gap: REQ-PROBE-001 keeps its outcome, its refusal class
// and whatever the transcript showed afterwards.
type pipelineOMPActiveProbeAttempt struct {
	startedAt    time.Time
	outcome      string
	method       string
	refusal      string
	tokensBefore *int64
	tokensAfter  *int64
	usagePresent bool
	usageKeys    pipelineOMPActiveProbeTally
	usageInput   *int64
	usageOutput  *int64
	images       int
	uiMethods    pipelineOMPActiveProbeTally
	roles        pipelineOMPActiveProbeTally
	contentTypes pipelineOMPActiveProbeTally
	collecting   bool
	// Authenticated checkpoint events this attempt received, including the ones
	// it refused for ordering. Never a provider request count.
	preCheckpoints  int
	postCheckpoints int
}

func (probe *pipelineOMPActiveProbe) beginCompaction() {
	if probe == nil {
		return
	}
	probe.attempt = &pipelineOMPActiveProbeAttempt{
		startedAt: time.Now(),
		outcome:   ompprobe.OutcomeFailed, method: ompprobe.MethodNone, refusal: ompprobe.RefusalNone,
	}
}

// acceptsCompactionAction widens the accepted native compaction action to the
// probe chain. Production still admits snapcompact only; the probe has to be
// able to watch a remote completion to answer H1', and it records the method
// rather than approving anything about it.
func (probe *pipelineOMPActiveProbe) acceptsCompactionAction(action string) bool {
	if action == ompprobe.MethodSnapcompact {
		return true
	}
	return probe != nil && action == ompprobe.MethodRemote
}

func (probe *pipelineOMPActiveProbe) validNativeEnd(frame pipelineOMPRPCFrame) bool {
	if !probe.acceptsCompactionAction(frame.Action) {
		return false
	}
	return pipelineOMPActiveNativeEndShape(frame)
}

// toleratesUIRequest lets a notice through the compaction barrier. omp/18.1.13
// reports a notice as extension_ui_request{method: notify}; it asks for no
// decision, so the probe records the method name and answers nothing. Every
// other unsupported UI activity still fails closed, and no confirmation is
// ever sent for a frame the bridge did not authenticate.
func (probe *pipelineOMPActiveProbe) toleratesUIRequest(frame pipelineOMPRPCFrame) bool {
	return probe != nil && frame.Type == "extension_ui_request" && frame.Method == "notify"
}

func (probe *pipelineOMPActiveProbe) observeCompactionFrame(frame pipelineOMPRPCFrame) {
	if probe == nil || probe.attempt == nil {
		return
	}
	switch frame.Type {
	case "auto_compaction_start", "auto_compaction_end":
		probe.attempt.observeMethod(frame.Action)
	case "extension_ui_request":
		probe.attempt.uiMethods.observe(frame.Method, pipelineOMPActiveProbeMaxIdentifiers)
	}
}

// observeCompactRefusal keeps the class of a declined compaction. The refusal
// text itself is never stored; only the class the harness already recognizes.
func (probe *pipelineOMPActiveProbe) observeCompactRefusal(message string) {
	if probe == nil || probe.attempt == nil {
		return
	}
	probe.attempt.outcome = ompprobe.OutcomeRefused
	probe.attempt.refusal = pipelineOMPActiveProbeRefusalClass(message)
}

func (probe *pipelineOMPActiveProbe) observeCompactResult(data json.RawMessage) {
	if probe == nil || probe.attempt == nil {
		return
	}
	probe.attempt.outcome = ompprobe.OutcomeCompleted
	probe.attempt.observeRemoteUsage(data)
}

func (probe *pipelineOMPActiveProbe) collectTranscript(collecting bool) {
	if probe == nil || probe.attempt == nil {
		return
	}
	probe.attempt.collecting = collecting
}

func (probe *pipelineOMPActiveProbe) observeCompactionImages(images []string) {
	if probe == nil || probe.attempt == nil {
		return
	}
	probe.attempt.images = len(images)
}

// observeTranscriptMessage walks with the production transcript validator and
// tallies the role and content-type tokens of the post-compaction page. T1
// needs those histograms to decide an allowlist; a probe run only observes
// them, so an unknown token never fails the call here.
func (probe *pipelineOMPActiveProbe) observeTranscriptMessage(value map[string]any) {
	if probe == nil || probe.attempt == nil || !probe.attempt.collecting {
		return
	}
	attempt := probe.attempt
	if role, ok := value["role"].(string); ok {
		attempt.roles.observe(role, pipelineOMPActiveProbeMaxHistogramKeys)
		// The page is ordered oldest to newest, so the summary of the
		// compaction that just ran is the last one to write these fields.
		if role == "compactionSummary" {
			attempt.observeMethod(pipelineOMPActiveProbeString(value["method"]))
			attempt.tokensBefore = pipelineOMPActiveProbeNumber(value["tokensBefore"])
			attempt.tokensAfter = pipelineOMPActiveProbeNumber(value["tokensAfter"])
		}
	}
	if contentType, ok := value["type"].(string); ok {
		attempt.contentTypes.observe(contentType, pipelineOMPActiveProbeMaxHistogramKeys)
	}
}

// finishCompaction emits the attempt record. A failure overrides the outcome
// so a compaction that crossed a barrier or lost its proof is never filed as
// completed, and the reason stays a body-free token.
func (probe *pipelineOMPActiveProbe) finishCompaction(abortReason string) {
	if probe == nil || probe.attempt == nil {
		return
	}
	attempt := probe.attempt
	probe.attempt = nil
	if abortReason != "" {
		attempt.outcome = ompprobe.OutcomeFailed
	}
	if !probe.enabled() {
		return
	}
	rejected := attempt.usageKeys.rejected + attempt.uiMethods.rejected +
		attempt.roles.rejected + attempt.contentTypes.rejected
	probe.record(ompprobe.Record{
		Kind: ompprobe.KindCompaction, Sequence: probe.sequence, Variant: probe.variant,
		SessionSequence: probe.sessionSequence, SessionSegment: probe.sessionSegment,
		Outcome: attempt.outcome, Method: attempt.method, Refusal: attempt.refusal,
		ElapsedMS:   pipelineOMPActiveProbeElapsedMS(time.Since(attempt.startedAt)),
		AbortReason: abortReason, TokensBefore: attempt.tokensBefore, TokensAfter: attempt.tokensAfter,
		MaintenanceUsagePresent: attempt.usagePresent,
		MaintenanceUsageKeys:    attempt.usageKeys.names(),
		MaintenanceInputTokens:  attempt.usageInput,
		MaintenanceOutputTokens: attempt.usageOutput,
		// The RPC surface reports the final attempt only. Nothing here proves
		// how many provider requests a completion cost, so coverage stays
		// unknown and T1 derives observability from the recorded usage
		// presence, method and image count instead.
		AttemptCoverage:  ompprobe.AttemptCoverageUnknown,
		CompactionImages: attempt.images, UIRequestMethods: attempt.uiMethods.names(),
		PreCheckpoints: attempt.preCheckpoints, PostCheckpoints: attempt.postCheckpoints,
		Roles: attempt.roles.histogram(), ContentTypes: attempt.contentTypes.histogram(),
		RejectedIdentifiers: rejected,
	})
}

func (attempt *pipelineOMPActiveProbeAttempt) observeMethod(action string) {
	if action == ompprobe.MethodRemote || action == ompprobe.MethodSnapcompact {
		attempt.method = action
	}
}

// observeRemoteUsage reads the only maintenance measurement source the compact
// response has. Presence and key names are recorded even when the values are
// missing, because "the field was not there" is exactly what T1 must know.
func (attempt *pipelineOMPActiveProbeAttempt) observeRemoteUsage(data json.RawMessage) {
	var body struct {
		PreserveData struct {
			OpenAIRemoteCompaction struct {
				Usage map[string]json.Number `json:"usage"`
			} `json:"openaiRemoteCompaction"`
		} `json:"preserveData"`
	}
	if json.Unmarshal(data, &body) != nil {
		return
	}
	usage := body.PreserveData.OpenAIRemoteCompaction.Usage
	if len(usage) == 0 {
		return
	}
	attempt.usagePresent = true
	keys := make([]string, 0, len(usage))
	for key := range usage {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		attempt.usageKeys.observe(key, pipelineOMPActiveProbeMaxIdentifiers)
	}
	attempt.usageInput = pipelineOMPActiveProbeNumber(usage["inputTokens"])
	attempt.usageOutput = pipelineOMPActiveProbeNumber(usage["outputTokens"])
}

func pipelineOMPActiveProbeRefusalClass(message string) string {
	switch message {
	case "Nothing to compact (session too small)":
		return ompprobe.RefusalTooSmall
	case "Nothing to compact (no messages yet)":
		return ompprobe.RefusalNoMessages
	case "snapcompact would not reduce context locally.":
		return ompprobe.RefusalWouldNotReduce
	}
	return ompprobe.RefusalNone
}

// pipelineOMPActiveProbeAbortReason maps a compaction failure to a body-free
// token. An unrecognized failure becomes compaction_failed rather than leaking
// the message it came from.
func pipelineOMPActiveProbeAbortReason(err error) string {
	if err == nil {
		return ""
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "crossed the compaction barrier"):
		return "barrier_crossed"
	case strings.Contains(message, "compaction start is invalid"):
		return "start_invalid"
	case strings.Contains(message, "compaction response is invalid"):
		return "response_invalid"
	case strings.Contains(message, "compaction completion is invalid"):
		return "completion_invalid"
	case strings.Contains(message, "no-op compaction proof is invalid"):
		return "noop_proof_invalid"
	case strings.Contains(message, "post-compaction state is invalid"):
		return "post_state_invalid"
	case strings.Contains(message, "pre-compaction checkpoint is out of order"):
		return "pre_ack_out_of_order"
	case strings.Contains(message, "post-compaction rehydration is out of order"):
		return "post_ack_out_of_order"
	case strings.Contains(message, "checkpoint request ID is replayed"):
		return "checkpoint_id_replayed"
	case strings.Contains(message, "repeated pre-compaction proof changed"):
		return "pre_proof_changed"
	case strings.Contains(message, "checkpoint arrived during the compaction proof"):
		return "checkpoint_during_proof"
	case strings.Contains(message, "compaction proof query was not answered"):
		return "proof_query_unanswered"
	case strings.Contains(message, "is out of order"):
		return "ack_out_of_order"
	case strings.Contains(message, "bridge authority mismatch"),
		strings.Contains(message, "unsupported UI activity"):
		return "bridge_rejected"
	case strings.Contains(message, "transcript"):
		return "transcript_invalid"
	case strings.Contains(message, "extension failed"):
		return "extension_failed"
	}
	return "compaction_failed"
}

func pipelineOMPActiveProbeString(value any) string {
	text, _ := value.(string)
	return text
}

// pipelineOMPActiveProbeNumber reads a transcript or usage number. The
// transcript walk decodes with UseNumber, so a native estimate arrives as
// json.Number; an absent or non-integral value stays nil instead of zero,
// because zero maintenance is a claim the probe must not make.
func pipelineOMPActiveProbeNumber(value any) *int64 {
	number, ok := value.(json.Number)
	if !ok {
		return nil
	}
	parsed, err := number.Int64()
	if err != nil || parsed < 0 {
		return nil
	}
	return &parsed
}
