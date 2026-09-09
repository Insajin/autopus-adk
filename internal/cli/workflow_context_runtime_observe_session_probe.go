package cli

// REQ-PROBE-001 probe wiring for observe-session. A probe run executes the
// identical 20-pair workload and writes measurement metadata to its own record
// file. It produces no promotion report, no evidence store, no attestation and
// no verdict, and it ends on a sentinel error so no caller can mistake it for a
// signed cohort.

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/ompprobe"
)

// errWorkflowContextObserveSessionProbeCompleted ends a complete probe cohort
// after the same input drain and process cleanup a successful cohort performs,
// and before any evidence is written.
var errWorkflowContextObserveSessionProbeCompleted = errors.New("observe-session probe completed")

const workflowContextObserveSessionProbeMaxPath = 4096

// validWorkflowContextObserveSessionProbeDir accepts an absent probe (the
// production path) or an absolute, already-clean directory path. Existence,
// mode 0700 and symlink refusal are enforced when the recorder opens it.
func validWorkflowContextObserveSessionProbeDir(dir string) bool {
	if dir == "" {
		return true
	}
	return filepath.IsAbs(dir) && filepath.Clean(dir) == dir &&
		len(dir) <= workflowContextObserveSessionProbeMaxPath &&
		!strings.ContainsRune(dir, 0) && strings.TrimSpace(dir) == dir
}

// startWorkflowContextObserveSessionProbe opens <dir>/probe.jsonl. The release
// lane passes a directory that does not exist yet under the canary-isolated
// TMPDIR, so the recorder creates it 0700 as the canary UID; root never
// pre-creates it.
func startWorkflowContextObserveSessionProbe(
	options workflowContextObserveSessionOptions,
) (*pipelineOMPActiveProbe, error) {
	if options.ProbeDir == "" {
		return nil, nil
	}
	recorder, err := ompprobe.NewRecorder(options.ProbeDir)
	if err != nil {
		return nil, fmt.Errorf("observe-session probe records are unavailable: %w", err)
	}
	return &pipelineOMPActiveProbe{recorder: recorder}, nil
}

// finish closes a complete probe. A record that did not land makes the run a
// partial probe, not a completed one, so the lost write is reported instead of
// probe_completed.
func (probe *pipelineOMPActiveProbe) finish() error {
	if !probe.enabled() {
		return nil
	}
	if err := errors.Join(probe.failure, probe.close()); err != nil {
		return fmt.Errorf("observe-session probe records failed: %w", err)
	}
	return errWorkflowContextObserveSessionProbeCompleted
}

// abort retains what an interrupted probe observed. The call in flight keeps
// its partial record with a body-free reason token; a failure outside any call
// is filed on its own with no variant and no sequence. Neither is a completed
// probe and neither carries a measurement.
func (probe *pipelineOMPActiveProbe) abort(runErr error) error {
	if !probe.enabled() {
		return nil
	}
	reason := workflowContextObserveSessionErrorCode(runErr)
	if probe.callOpen {
		probe.finishCall(0, reason)
	} else {
		probe.recordAbort(reason)
	}
	return probe.close()
}

func (probe *pipelineOMPActiveProbe) close() error {
	if !probe.enabled() {
		return nil
	}
	recorder := probe.recorder
	probe.recorder = nil
	return recorder.Close()
}

// observeCallElapsed is the probe's view of one finished cohort call. The run
// loop owns the clock because it also owns the monotonic ordering the evidence
// path needs.
func (probe *pipelineOMPActiveProbe) observeCallElapsed(startedAt, endedAt time.Time) {
	probe.finishCall(endedAt.Sub(startedAt), "")
}

// workflowContextObserveSessionCompactionFloor is the number of completed
// compactions a signed cohort must show. A probe measures whether compaction
// happens at all — a run that observes zero completions is a valid measurement
// and must still be recorded — so the floor does not apply to it. Every other
// cardinality and safety check is unchanged.
func workflowContextObserveSessionCompactionFloor(probe *pipelineOMPActiveProbe) int {
	if probe.enabled() {
		return 0
	}
	return 2
}
