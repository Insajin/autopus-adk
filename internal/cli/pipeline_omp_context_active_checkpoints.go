package cli

// REQ-CHECKPOINT-001 checkpoint admission for one manual compact transaction.
//
// Both retained T0 probe runs stopped at sequence 6 on an authenticated
// pre-compaction checkpoint that arrived a second time inside the same
// transaction, which the single-pre contract could only read as a replay. This
// file bounds that retry instead of tolerating it: the allowance is two, it is
// granted only to the measured executable that actually produced the repeat,
// and every ordering, replay and authority barrier stays closed. A repeated
// checkpoint is a protocol observation, never evidence that a compaction
// succeeded or that maintenance cost is known.

import (
	"errors"
	"strings"

	"github.com/insajin/autopus-adk/pkg/ompprobe"
)

const (
	pipelineOMPActiveSinglePreCheckpoint   = 1
	pipelineOMPActiveBoundedPreCheckpoints = 2
	// The measured Darwin/arm64 identity of the only executable whose repeated
	// pre-checkpoint was observed. A different version, an unmeasured digest or
	// any mismatch keeps the single-pre rule.
	pipelineOMPActiveCheckpointOMPVersion = "omp/18.1.13"
	pipelineOMPActiveCheckpointOMPDigest  = "sha256:a4c5c9cc5b8222184d0d7429b0bb6ac2a92bbe45dd11bf68e4b1360050791909"
)

// pipelineOMPActiveCheckpointChain is the verified B overlay chain the profile
// requires. Any other chain, the production [snapcompact] body included, keeps
// the single-pre rule.
var pipelineOMPActiveCheckpointChain = []string{ompprobe.MethodRemote, ompprobe.MethodSnapcompact}

// pipelineOMPActiveCheckpoints is the checkpoint state of one manual compact
// transaction. pre and post count authenticated received events, including the
// ones refused for ordering, because the retained record has to show what the
// runtime sent; preACKs and postACKs count only the confirmations actually
// written. Neither pair is a provider request count or a method identity claim.
//
// Request IDs arrive already validated nonempty by
// validatePipelineOMPActiveBridgeFrame, so this state only has to keep them
// distinct.
type pipelineOMPActiveCheckpoints struct {
	limit    int
	ids      [pipelineOMPActiveBoundedPreCheckpoints + 1]string
	idCount  int
	pre      int
	post     int
	preACKs  int
	postACKs int
}

// admitPre accepts an authenticated pre-compaction checkpoint and reports
// whether it is the bounded second one, whose transcript proof must be
// revalidated before it may be acknowledged. The event is counted before every
// decision so a refused checkpoint still appears in the counts, and it is
// refused after any post-checkpoint or compact response, past the profile
// limit, and on a request ID this transaction already used.
func (checkpoints *pipelineOMPActiveCheckpoints) admitPre(id string, responded bool) (bool, error) {
	checkpoints.pre++
	if checkpoints.post > 0 || responded || checkpoints.pre > checkpoints.limit {
		return false, errors.New("managed active OMP pre-compaction checkpoint is out of order")
	}
	if err := checkpoints.register(id); err != nil {
		return false, err
	}
	return checkpoints.pre > 1, nil
}

// admitPost accepts the single authenticated post-compaction checkpoint a
// successful compaction requires. It needs an acknowledged pre-checkpoint
// first and refuses a duplicate post or one arriving after the compact
// response.
func (checkpoints *pipelineOMPActiveCheckpoints) admitPost(id string, responded bool) error {
	checkpoints.post++
	if checkpoints.preACKs == 0 || checkpoints.post > 1 || responded {
		return errors.New("managed active OMP post-compaction rehydration is out of order")
	}
	return checkpoints.register(id)
}

// countDuringProof keeps an authenticated checkpoint that arrived while the
// repeated-pre proof query was outstanding. It is counted and never
// acknowledged; the caller fails the transaction.
func (checkpoints *pipelineOMPActiveCheckpoints) countDuringProof(event string) {
	if event == WorkflowContextEventPreCompaction {
		checkpoints.pre++
		return
	}
	checkpoints.post++
}

// acknowledged records a confirmation that reached OMP.
func (checkpoints *pipelineOMPActiveCheckpoints) acknowledged(event string) {
	if event == WorkflowContextEventPreCompaction {
		checkpoints.preACKs++
		return
	}
	checkpoints.postACKs++
}

// register keeps the checkpoint request IDs of one transaction distinct, so a
// replayed ID can never be acknowledged twice.
func (checkpoints *pipelineOMPActiveCheckpoints) register(id string) error {
	for _, seen := range checkpoints.ids[:checkpoints.idCount] {
		if seen == id {
			return errors.New("managed active OMP checkpoint request ID is replayed")
		}
	}
	if checkpoints.idCount == len(checkpoints.ids) {
		return errors.New("managed active OMP checkpoint request count is invalid")
	}
	checkpoints.ids[checkpoints.idCount] = id
	checkpoints.idCount++
	return nil
}

// configureProbeCheckpointProfile decides how many authenticated
// pre-compaction checkpoints one transaction may acknowledge. The bounded
// second pre is granted only to an explicitly enabled probe recording an
// optimized session on the measured omp/18.1.13 executable with the verified
// [remote, snapcompact] overlay. The identity is the runtime measurement the
// observe-session setup made, never a caller-supplied retry limit or model
// name, so production, A sessions, an unmeasured binary and any other chain
// keep the original single-pre rule.
func (protocol *pipelineOMPRPCProtocol) configureProbeCheckpointProfile(
	version string,
	digest string,
	optimized bool,
	methodOrder []string,
) {
	protocol.preCheckpointLimit = pipelineOMPActiveSinglePreCheckpoint
	if !protocol.probe.enabled() || !optimized ||
		version != pipelineOMPActiveCheckpointOMPVersion || digest != pipelineOMPActiveCheckpointOMPDigest ||
		len(methodOrder) != len(pipelineOMPActiveCheckpointChain) {
		return
	}
	for index, method := range pipelineOMPActiveCheckpointChain {
		if methodOrder[index] != method {
			return
		}
	}
	protocol.preCheckpointLimit = pipelineOMPActiveBoundedPreCheckpoints
}

// pipelineOMPActiveOverlayChain reads back the compaction chain the overlay
// body written for this session declares, so a checkpoint profile can never be
// granted for a chain the runtime did not configure.
func pipelineOMPActiveOverlayChain(probeMethodOrder bool) []string {
	declaration := workflowContextProductOverlayMethodOrder
	if probeMethodOrder {
		declaration = workflowContextProbeOverlayMethodOrder
	}
	methods := strings.Split(strings.Trim(declaration, "[]"), ",")
	chain := make([]string, 0, len(methods))
	for _, method := range methods {
		chain = append(chain, strings.TrimSpace(method))
	}
	return chain
}

// bindRuntimeIdentity records the measured executable identity once, before
// any segment can send a frame. A probe cannot re-bind it, so no later frame,
// option or model name moves a run onto the checkpoint profile.
func (probe *pipelineOMPActiveProbe) bindRuntimeIdentity(version, digest string) {
	if probe == nil || probe.ompVersion != "" || probe.ompExecutableSHA256 != "" {
		return
	}
	probe.ompVersion, probe.ompExecutableSHA256 = version, digest
}

// runtimeIdentity is the measured identity a checkpoint profile may be granted
// from. A production run has no probe and no identity.
func (probe *pipelineOMPActiveProbe) runtimeIdentity() (string, string) {
	if probe == nil {
		return "", ""
	}
	return probe.ompVersion, probe.ompExecutableSHA256
}

// observeCheckpoints retains the authenticated checkpoint counts of one
// attempt. They are received-event counts: a checkpoint refused for ordering
// is included, and neither number is a provider request count.
func (probe *pipelineOMPActiveProbe) observeCheckpoints(pre, post int) {
	if probe == nil || probe.attempt == nil {
		return
	}
	probe.attempt.preCheckpoints, probe.attempt.postCheckpoints = pre, post
}
