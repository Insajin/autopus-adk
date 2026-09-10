package cli

// The checkpoint fixture model family: which exchanges exist and the shared
// pieces every one of them builds from. The exchange itself lives beside this
// file so both stay inside the source-size limit.

import (
	"encoding/json"
	"os"
	"slices"
	"strings"
)

const pipelineOMPActiveCheckpointModelPrefix = "model-checkpoint-"

// pipelineOMPActiveCheckpointSessionID is the session every fixture model
// reports, so a same-session proof can be asserted rather than assumed.
const pipelineOMPActiveCheckpointSessionID = "active-session"

// Fixture variants. Only repeat completes the two-pre transaction and only the
// refuse family reaches a declined compaction; every other one reproduces a
// single adversarial interleaving.
const (
	pipelineOMPActiveCheckpointRepeat          = "repeat"
	pipelineOMPActiveCheckpointNotice          = "notice"
	pipelineOMPActiveCheckpointReplay          = "replay"
	pipelineOMPActiveCheckpointThird           = "third"
	pipelineOMPActiveCheckpointMutate          = "mutate"
	pipelineOMPActiveCheckpointForged          = "forged"
	pipelineOMPActiveCheckpointPostFirst       = "postfirst"
	pipelineOMPActiveCheckpointPostDup         = "postdup"
	pipelineOMPActiveCheckpointPreAfterPost    = "preafterpost"
	pipelineOMPActiveCheckpointProvider        = "provider"
	pipelineOMPActiveCheckpointExtra           = "extra"
	pipelineOMPActiveCheckpointEarly           = "early"
	pipelineOMPActiveCheckpointStray           = "stray"
	pipelineOMPActiveCheckpointExtError        = "exterror"
	pipelineOMPActiveCheckpointTurnStart       = "turnstart"
	pipelineOMPActiveCheckpointTurnBefore      = "turnbefore"
	pipelineOMPActiveCheckpointTurnEnd         = "turnend"
	pipelineOMPActiveCheckpointAgentEnd        = "agentend"
	pipelineOMPActiveCheckpointPromptResult    = "promptresult"
	pipelineOMPActiveCheckpointPostDuringProof = "postduringproof"
	pipelineOMPActiveCheckpointWrongCommand    = "wrongcommand"
	pipelineOMPActiveCheckpointWrongID         = "wrongid"
	pipelineOMPActiveCheckpointRefuse          = "refuse"
	pipelineOMPActiveCheckpointRefuseMutate    = "refusemutate"
	pipelineOMPActiveCheckpointRefuseBusy      = "refusebusy"
)

// pipelineOMPActiveCheckpointVariants is the closed set of exchanges. A model
// id outside it leaves the fixture inactive, so a mistyped variant can never
// read as the legitimate exchange.
var pipelineOMPActiveCheckpointVariants = []string{
	pipelineOMPActiveCheckpointRepeat, pipelineOMPActiveCheckpointNotice,
	pipelineOMPActiveCheckpointReplay, pipelineOMPActiveCheckpointThird,
	pipelineOMPActiveCheckpointMutate, pipelineOMPActiveCheckpointForged,
	pipelineOMPActiveCheckpointPostFirst, pipelineOMPActiveCheckpointPostDup,
	pipelineOMPActiveCheckpointPreAfterPost, pipelineOMPActiveCheckpointProvider,
	pipelineOMPActiveCheckpointExtra, pipelineOMPActiveCheckpointEarly,
	pipelineOMPActiveCheckpointStray, pipelineOMPActiveCheckpointExtError,
	pipelineOMPActiveCheckpointTurnStart, pipelineOMPActiveCheckpointTurnBefore,
	pipelineOMPActiveCheckpointTurnEnd, pipelineOMPActiveCheckpointAgentEnd,
	pipelineOMPActiveCheckpointPromptResult, pipelineOMPActiveCheckpointPostDuringProof,
	pipelineOMPActiveCheckpointWrongCommand, pipelineOMPActiveCheckpointWrongID,
	pipelineOMPActiveCheckpointRefuse, pipelineOMPActiveCheckpointRefuseMutate,
	pipelineOMPActiveCheckpointRefuseBusy,
}

const (
	pipelineOMPActiveCheckpointIdle = iota
	pipelineOMPActiveCheckpointFirstPre
	pipelineOMPActiveCheckpointSecondPre
	pipelineOMPActiveCheckpointAwaitPost
	// pipelineOMPActiveCheckpointRefused is the window between a declined
	// compact response and the no-op proof the harness runs on it.
	pipelineOMPActiveCheckpointRefused
)

type pipelineOMPActiveCheckpointFixture struct {
	variant   string
	stage     int
	sequence  int
	compactID string
}

func newPipelineOMPActiveCheckpointFixture(modelID string) *pipelineOMPActiveCheckpointFixture {
	fixture := &pipelineOMPActiveCheckpointFixture{}
	fixture.retarget(modelID)
	return fixture
}

// retarget follows set_model.
func (fixture *pipelineOMPActiveCheckpointFixture) retarget(modelID string) {
	variant := strings.TrimPrefix(modelID, pipelineOMPActiveCheckpointModelPrefix)
	fixture.variant = ""
	if slices.Contains(pipelineOMPActiveCheckpointVariants, variant) {
		fixture.variant = variant
	}
}

// declines reports the models whose compaction ends in a refusal instead of a
// completion. omp/18.1.x evaluates the reduction after firing the pre hook, so
// a decline can arrive with the pre-checkpoints already acknowledged.
func (fixture *pipelineOMPActiveCheckpointFixture) declines() bool {
	switch fixture.variant {
	case pipelineOMPActiveCheckpointRefuse, pipelineOMPActiveCheckpointRefuseMutate,
		pipelineOMPActiveCheckpointRefuseBusy:
		return true
	}
	return false
}

func (fixture *pipelineOMPActiveCheckpointFixture) handles(commandType string) bool {
	if fixture.variant == "" {
		return false
	}
	switch commandType {
	case "compact", "extension_ui_response", "get_messages_page":
		return true
	case "get_state":
		// Only the idle read of one declined compaction is answered here.
		return fixture.variant == pipelineOMPActiveCheckpointRefuseBusy &&
			fixture.stage == pipelineOMPActiveCheckpointRefused
	}
	return false
}

// pipelineOMPActiveFixtureBinding is the bridge authority the managed
// environment handed to the fake OMP.
func pipelineOMPActiveFixtureBinding() WorkflowContextBridgeBinding {
	return WorkflowContextBridgeBinding{
		SchemaVersion: workflowContextBridgeSchemaVersion,
		BindingHash:   os.Getenv("AUTOPUS_OMP_CONTEXT_BINDING_HASH"),
		OptionsHash:   os.Getenv("AUTOPUS_OMP_CONTEXT_OPTIONS_HASH"),
		SessionHash:   os.Getenv("AUTOPUS_OMP_CONTEXT_SESSION_HASH"),
		NonceHash:     os.Getenv("AUTOPUS_OMP_CONTEXT_NONCE_HASH"),
	}
}

// pipelineOMPActiveCheckpointSummary is the message a completed snapcompact
// leaves behind. It is appended only after the transaction settles, so every
// proof query inside the barrier still sees the transcript the transaction
// started from.
func pipelineOMPActiveCheckpointSummary() json.RawMessage {
	return json.RawMessage(
		`{"role":"compactionSummary","method":"snapcompact","tokensBefore":61904,"tokensAfter":30000}`)
}

// pipelineOMPActiveCheckpointMoved is a structurally valid transcript that is
// not the one the transaction opened on, so only the proof hash can catch it.
func pipelineOMPActiveCheckpointMoved(transcript []json.RawMessage) []json.RawMessage {
	moved := make([]json.RawMessage, 0, len(transcript)+1)
	moved = append(moved, transcript...)
	return append(moved, json.RawMessage(`{"role":"user","content":"safe transcript change"}`))
}
