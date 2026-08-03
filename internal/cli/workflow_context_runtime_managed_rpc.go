package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"sync"
	"time"
)

const workflowContextManagedRuntimeMarker = ".autopus-managed-omp-runtime"

// WorkflowContextManagedRPCOptions identifies one Autopus-owned OMP RPC process.
type WorkflowContextManagedRPCOptions struct {
	Executable         string
	Workspace          string
	RuntimeBase        string
	RuntimeRoot        string
	SessionDir         string
	ConfigPath         string
	Model              string
	AllowedEndpoint    string
	Environment        []string
	HistoryAfterTokens map[string]int
	MaxTime            time.Duration
}

// WorkflowContextManagedRPCObservation is body-free live admission evidence.
type WorkflowContextManagedRPCObservation struct {
	PID                       int
	ProviderTurns             int
	PreACKs                   int
	PostACKs                  int
	NativeStarts              int
	NativeEnds                int
	SameProcess               bool
	SameSession               bool
	Sandboxed                 bool
	ProviderObserved          bool
	ProcessActiveAfterCleanup bool
}

// WorkflowContextManagedRPCDriver owns a single private OMP stdio session.
type WorkflowContextManagedRPCDriver struct {
	mu                sync.Mutex
	options           WorkflowContextManagedRPCOptions
	baseRoot          *os.Root
	directoryRoot     *os.Root
	directoryInfo     fs.FileInfo
	binding           WorkflowContextBridgeBinding
	bound             bool
	running           bool
	closed            bool
	process           *workflowContextManagedRPCProcess
	protocol          *workflowContextManagedRPCProtocol
	protocolPostID    string
	protocolSessionID string
	sourceIdentities  []workflowContextManagedSourceIdentity
	observation       WorkflowContextManagedRPCObservation
}

var _ WorkflowContextManagedProcessDriver = (*WorkflowContextManagedRPCDriver)(nil)

// NewWorkflowContextManagedRPCDriver validates ownership before any child starts.
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: public constructor for the Autopus-owned OMP RPC process boundary.
// @AX:REASON [AUTO]: live-canary and integration callers depend on runtime ownership being validated and leased before process startup.
func NewWorkflowContextManagedRPCDriver(
	options WorkflowContextManagedRPCOptions,
) (*WorkflowContextManagedRPCDriver, error) {
	normalized, err := validateWorkflowContextManagedRPCOptions(options)
	if err != nil {
		return nil, err
	}
	identities, err := captureWorkflowContextManagedSourceIdentities(normalized)
	if err != nil {
		return nil, err
	}
	baseRoot, directoryRoot, directoryInfo, err := openWorkflowContextManagedRuntime(normalized)
	if err != nil {
		return nil, err
	}
	if err := validateInitialWorkflowContextManagedRuntime(directoryRoot, normalized, false); err != nil {
		_ = directoryRoot.Close()
		_ = baseRoot.Close()
		return nil, err
	}
	if err := createWorkflowContextManagedRuntimeLease(directoryRoot); err != nil {
		_ = directoryRoot.Close()
		_ = baseRoot.Close()
		return nil, fmt.Errorf("mark managed OMP runtime: %w", err)
	}
	if err := validateInitialWorkflowContextManagedRuntime(directoryRoot, normalized, true); err != nil {
		_ = directoryRoot.Remove(workflowContextManagedRuntimeMarker)
		_ = directoryRoot.Close()
		_ = baseRoot.Close()
		return nil, err
	}
	return &WorkflowContextManagedRPCDriver{
		options: normalized, baseRoot: baseRoot, directoryRoot: directoryRoot, directoryInfo: directoryInfo,
		sourceIdentities: identities,
	}, nil
}

func (driver *WorkflowContextManagedRPCDriver) Bind(
	_ context.Context, binding WorkflowContextBridgeBinding,
) error {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.running || driver.closed || driver.bound {
		return errors.New("managed OMP bridge binding is not replaceable")
	}
	if err := validateWorkflowContextManagedBinding(binding); err != nil {
		return err
	}
	driver.binding, driver.bound = binding, true
	return nil
}

func (driver *WorkflowContextManagedRPCDriver) ArtifactCount(context.Context) (int, error) {
	driver.mu.Lock()
	root := driver.options.RuntimeRoot
	driver.mu.Unlock()
	if _, err := os.Lstat(root); errors.Is(err, os.ErrNotExist) {
		return 0, nil
	} else if err != nil {
		return 0, fmt.Errorf("inspect managed OMP runtime: %w", err)
	}
	return 1, nil
}

func (driver *WorkflowContextManagedRPCDriver) Cleanup(context.Context) error {
	driver.mu.Lock()
	if driver.closed {
		driver.mu.Unlock()
		return nil
	}
	process := driver.process
	baseRoot := driver.baseRoot
	directoryRoot := driver.directoryRoot
	directoryInfo := driver.directoryInfo
	driver.mu.Unlock()
	var result error
	if process != nil {
		result = errors.Join(result, process.Close())
	}
	if !removeWorkflowContextManagedRuntime(baseRoot, directoryRoot, directoryInfo) {
		result = errors.Join(result, errors.New("managed OMP runtime ownership changed before cleanup"))
	}
	result = errors.Join(result, directoryRoot.Close(), baseRoot.Close())
	driver.mu.Lock()
	driver.closed = true
	driver.running = false
	driver.baseRoot = nil
	driver.directoryRoot = nil
	driver.observation.ProcessActiveAfterCleanup = process != nil && process.Active()
	driver.mu.Unlock()
	return result
}

// Observation returns a detached, body-free snapshot of live evidence.
func (driver *WorkflowContextManagedRPCDriver) Observation() WorkflowContextManagedRPCObservation {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	return driver.observation
}
