package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type workflowContextManagedRPCCredentialAuthority struct {
	environmentKey string
}

type workflowContextManagedDispatchLease struct {
	protocol       *workflowContextManagedRPCProtocol
	process        *workflowContextManagedRPCProcess
	postID         string
	admissionID    string
	barrierID      string
	compactID      string
	manual         bool
	initialSession string
	binding        WorkflowContextBridgeBinding
}

// triggerNextWorkflowContextManagedCompaction requests a deterministic manual
// boundary only for an explicit multi-cycle canary. Normal sessions remain single-cycle.
func (driver *WorkflowContextManagedRPCDriver) triggerNextWorkflowContextManagedCompaction(
	ctx context.Context, lease workflowContextManagedDispatchLease, message string, beforeMessages int,
) (bool, error) {
	driver.mu.Lock()
	wantsNext := driver.options.CompactionCycles > 1 && driver.dispatchSequence < driver.options.CompactionCycles
	sequence := driver.dispatchSequence + 1
	driver.mu.Unlock()
	if !wantsNext {
		return false, nil
	}
	triggerID := fmt.Sprintf("managed-cycle-trigger-%d", sequence)
	if err := lease.protocol.send(map[string]any{"id": triggerID, "type": "prompt", "message": message}); err != nil {
		return false, err
	}
	nativeStarted, err := lease.protocol.awaitProviderBoundaryState(ctx, triggerID, true)
	if err != nil {
		return false, err
	}
	if nativeStarted {
		return true, nil
	}
	state, err := lease.protocol.state(ctx, fmt.Sprintf("managed-cycle-trigger-state-%d", sequence))
	if err != nil || !safeWorkflowContextManagedRPCState(state) ||
		state.SessionID != lease.initialSession || state.MessageCount <= beforeMessages {
		return false, errors.New("managed OMP cycle trigger state is not session-bound")
	}
	compactID := fmt.Sprintf("managed-compact-%d", sequence)
	if err := lease.protocol.send(map[string]any{"id": compactID, "type": "compact"}); err != nil {
		return false, err
	}
	driver.mu.Lock()
	driver.protocolManual = true
	driver.mu.Unlock()
	return true, nil
}

func (driver *WorkflowContextManagedRPCDriver) setPendingWorkflowContextManagedDispatch(id, session string) {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.dispatchState != "" && driver.dispatchState != workflowContextManagedDispatchCompleted {
		return
	}
	driver.protocolPostID, driver.protocolSessionID = id, session
	driver.protocolPostReplay = false
	if _, replayed := driver.usedProtocolPostIDs[id]; replayed {
		driver.protocolPostReplay = true
	}
	driver.dispatchState = workflowContextManagedDispatchPending
}

func (driver *WorkflowContextManagedRPCDriver) beginWorkflowContextManagedDispatch() (
	workflowContextManagedDispatchLease, error,
) {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.protocolPostReplay {
		driver.dispatchState = workflowContextManagedDispatchFailed
		return workflowContextManagedDispatchLease{}, errors.New("managed OMP post hook was replayed")
	}
	if !driver.running || driver.protocol == nil || driver.process == nil ||
		driver.protocolPostID == "" || driver.dispatchState != workflowContextManagedDispatchPending {
		return workflowContextManagedDispatchLease{}, errors.New("managed OMP dispatch is outside the live post hook")
	}
	if driver.usedProtocolPostIDs == nil {
		driver.usedProtocolPostIDs = make(map[string]struct{})
	}
	driver.usedProtocolPostIDs[driver.protocolPostID] = struct{}{}
	driver.dispatchSequence++
	manual := driver.protocolManual
	driver.protocolManual = false
	lease := workflowContextManagedDispatchLease{
		protocol: driver.protocol, process: driver.process, postID: driver.protocolPostID,
		admissionID:    fmt.Sprintf("managed-admission-%d", driver.dispatchSequence),
		barrierID:      fmt.Sprintf("managed-compaction-barrier-%d", driver.dispatchSequence),
		compactID:      fmt.Sprintf("managed-compact-%d", driver.dispatchSequence),
		manual:         manual,
		initialSession: driver.protocolSessionID, binding: driver.binding,
	}
	driver.protocolPostID = ""
	driver.dispatchState = workflowContextManagedDispatching
	return lease, nil
}

func (driver *WorkflowContextManagedRPCDriver) finishWorkflowContextManagedDispatch(succeeded bool) {
	driver.mu.Lock()
	defer driver.mu.Unlock()
	if driver.dispatchState != workflowContextManagedDispatching {
		return
	}
	driver.dispatchState = workflowContextManagedDispatchFailed
	if succeeded {
		driver.dispatchState = workflowContextManagedDispatchCompleted
	}
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: provider config and process-private credential resolve to one loopback authority.
// @AX:REASON [AUTO]: option validation and effective-setting preflight depend on the exact model, endpoint, auth-header, and environment-key binding.
// @AX:WARN [AUTO]: credential authority validation has cyclomatic complexity 38 and 10 fail-closed if branches.
// @AX:REASON [AUTO]: URL origin, private file identity, strict YAML shape, provider/model identity, and dedicated credential isolation must agree.
func loadWorkflowContextManagedRPCCredentialAuthority(
	options WorkflowContextManagedRPCOptions,
) (workflowContextManagedRPCCredentialAuthority, error) {
	providerID, modelID, ok := strings.Cut(options.Model, "/")
	endpoint, err := url.Parse(options.AllowedEndpoint)
	if err != nil || endpoint == nil {
		return workflowContextManagedRPCCredentialAuthority{}, errors.New("managed OMP product model authority is invalid")
	}
	port, portErr := strconv.Atoi(endpoint.Port())
	ip := net.ParseIP(endpoint.Hostname())
	if !ok || providerID == "" || modelID == "" || endpoint.Scheme != "http" ||
		endpoint.User != nil || (endpoint.Path != "" && endpoint.Path != "/") || endpoint.RawQuery != "" ||
		endpoint.Fragment != "" || ip == nil || !ip.IsLoopback() || portErr != nil || port < 1 || port > 65535 {
		return workflowContextManagedRPCCredentialAuthority{}, errors.New("managed OMP product model authority is invalid")
	}
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: models.yml is owner-private and capped at 1 MiB before strict decoding.
	path := filepath.Join(options.RuntimeRoot, "models.yml")
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm()&^fs.FileMode(0o600) != 0 || info.Size() > 1<<20 {
		return workflowContextManagedRPCCredentialAuthority{}, errors.New("managed OMP product model config is unavailable")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return workflowContextManagedRPCCredentialAuthority{}, errors.New("managed OMP product model config is unavailable")
	}
	var config struct {
		Providers map[string]struct {
			BaseURL    string `yaml:"baseUrl"`
			APIKey     string `yaml:"apiKey"`
			AuthHeader bool   `yaml:"authHeader"`
			Auth       string `yaml:"auth"`
			API        string `yaml:"api"`
			Models     []struct {
				ID            string   `yaml:"id"`
				Name          string   `yaml:"name"`
				Reasoning     bool     `yaml:"reasoning"`
				Input         []string `yaml:"input"`
				ContextWindow int      `yaml:"contextWindow"`
				MaxTokens     int      `yaml:"maxTokens"`
			} `yaml:"models"`
		} `yaml:"providers"`
	}
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	if decoder.Decode(&config) != nil || decoder.Decode(&struct{}{}) != io.EOF || len(config.Providers) != 1 {
		return workflowContextManagedRPCCredentialAuthority{}, errors.New("managed OMP product model config is invalid")
	}
	provider, found := config.Providers[providerID]
	credentialKey := strings.TrimSpace(provider.APIKey)
	wantBase := strings.TrimSuffix(endpoint.String(), "/") + "/v1"
	if !found || provider.BaseURL != wantBase || provider.API != "openai-completions" ||
		provider.Auth != "" || !provider.AuthHeader || !validWorkflowContextManagedEnvironmentKey(credentialKey) ||
		len(provider.Models) != 1 || provider.Models[0].ID != modelID {
		return workflowContextManagedRPCCredentialAuthority{},
			errors.New("managed OMP product model config does not match credential authority")
	}
	if !strings.HasPrefix(credentialKey, "AUTOPUS_OMP_CONTEXT_PROVIDER_") {
		return workflowContextManagedRPCCredentialAuthority{}, errors.New("managed OMP product credential key is not dedicated")
	}
	if _, reserved := workflowContextManagedReservedEnvironment[credentialKey]; reserved {
		return workflowContextManagedRPCCredentialAuthority{}, errors.New("managed OMP product credential key is reserved")
	}
	if _, ambient := workflowContextManagedAllowedEnvironment[credentialKey]; ambient {
		return workflowContextManagedRPCCredentialAuthority{}, errors.New("managed OMP product credential key is ambiguous")
	}
	if _, err := workflowContextManagedRPCCredentialValue(options.Environment, credentialKey); err != nil {
		return workflowContextManagedRPCCredentialAuthority{}, err
	}
	return workflowContextManagedRPCCredentialAuthority{environmentKey: credentialKey}, nil
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: bearer input is capped at 64 KiB and must remain a single non-path environment value.
func workflowContextManagedRPCCredentialValue(environment []string, credentialKey string) (string, error) {
	value, found := "", false
	for _, entry := range environment {
		key, candidate, ok := strings.Cut(entry, "=")
		if !ok || key != credentialKey {
			continue
		}
		if found {
			return "", fmt.Errorf("managed OMP environment key is duplicated: %s", credentialKey)
		}
		value, found = candidate, true
	}
	if !found || value == "" || len(value) > 64<<10 || strings.ContainsAny(value, "\x00\r\n") || filepath.IsAbs(value) {
		return "", errors.New("managed OMP product credential value is unavailable")
	}
	return value, nil
}
