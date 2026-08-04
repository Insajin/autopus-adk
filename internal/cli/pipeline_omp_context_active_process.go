package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	ompadapter "github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/config"
)

const (
	pipelineOMPActiveEndpointKey    = "AUTOPUS_OMP_CONTEXT_PROVIDER_ENDPOINT"
	pipelineOMPActiveCredentialKey  = "AUTOPUS_OMP_CONTEXT_PROVIDER_TOKEN"
	pipelineOMPActiveRPCIdentity    = "autopus.omp-pipeline-managed-rpc.v2"
	pipelineOMPActivePolicyIdentity = "manual-compact;auto-compaction=off;retry-off;ambient-off;sandbox=candidate-managed|producer-inherited-external-live-image-darwin-v2;tools=read,bash,edit,write,grep,glob,todo"
)

type pipelineOMPActiveProcessConfig struct {
	backend              pipelineOMPBackendConfig
	candidate            pipelineOMPManagedActiveCandidate
	prepared             pipelineOMPManagedActivePrepared
	binding              WorkflowContextBridgeBinding
	endpoint, credential string
	sandboxMode          pipelineOMPActiveSandboxMode
}

func preparePipelineOMPActiveProcessConfig(
	backend pipelineOMPBackendConfig,
	candidate pipelineOMPManagedActiveCandidate,
	prepared pipelineOMPManagedActivePrepared,
) (pipelineOMPActiveProcessConfig, error) {
	endpointRaw, endpointFound := pipelineOMPEnvironmentValue(backend.Environment, pipelineOMPActiveEndpointKey)
	credential, credentialFound := pipelineOMPEnvironmentValue(backend.Environment, pipelineOMPActiveCredentialKey)
	endpoint, err := validatePipelineOMPActiveEndpoint(endpointRaw)
	if !endpointFound || !credentialFound || err != nil || credential == "" || strings.ContainsRune(credential, 0) {
		return pipelineOMPActiveProcessConfig{}, errors.New("pipeline: managed active broker authority is unavailable")
	}
	nonce, err := newWorkflowContextRunNonceHash()
	if err != nil {
		return pipelineOMPActiveProcessConfig{}, err
	}
	implementation := pipelineOMPActiveImplementationDigest()
	return pipelineOMPActiveProcessConfig{
		backend: backend, candidate: candidate, prepared: prepared, endpoint: endpoint, credential: credential,
		binding: WorkflowContextBridgeBinding{
			SchemaVersion: workflowContextBridgeSchemaVersion,
			BindingHash: workflowContextRuntimeHash(strings.Join([]string{
				prepared.Binding.GrantDigest, prepared.Binding.WorkspaceID, prepared.Binding.SpecID,
				prepared.Binding.GitCommitHash, prepared.Binding.AutoSourceCommit, prepared.Binding.AutoSourceTree,
			}, "\x00")),
			OptionsHash: workflowContextRuntimeHash(prepared.Binding.PolicyDigest + "\x00" + implementation),
			SessionHash: workflowContextRuntimeHash(prepared.Binding.WorkspaceID + "\x00" + prepared.Binding.SpecID + "\x00" + prepared.Binding.GitCommitHash),
			NonceHash:   nonce,
		},
	}, nil
}

// @AX:WARN [AUTO]: managed active process startup contains 17 if branches.
// @AX:REASON [AUTO]: runtime ownership, executable identity, overlay, sandbox, process group, pipes, and readiness gates converge before admission.
func startPipelineOMPActiveProcess(
	ctx context.Context,
	active pipelineOMPActiveProcessConfig,
) (result *pipelineOMPProcess, resultErr error) {
	runtimeRoot, err := os.MkdirTemp(active.backend.RuntimeBase, "pipeline-active-")
	if err != nil {
		return nil, fmt.Errorf("create managed active runtime: %w", err)
	}
	cleanup := func() { _ = os.RemoveAll(runtimeRoot) }
	if err := os.Chmod(runtimeRoot, 0o700); err != nil {
		cleanup()
		return nil, err
	}
	runtimeInfo, err := os.Lstat(runtimeRoot)
	if err != nil {
		cleanup()
		return nil, err
	}
	sessionDir := filepath.Join(runtimeRoot, "sessions")
	if err := os.Mkdir(sessionDir, 0o700); err != nil {
		cleanup()
		return nil, err
	}
	privateExecutable, err := materializePipelineOMPExecutable(
		active.backend.Executable, active.backend.executableID, runtimeRoot,
	)
	if err != nil {
		cleanup()
		return nil, err
	}
	bridgePath, err := materializePipelineOMPActiveBridge(runtimeRoot)
	if err != nil {
		cleanup()
		return nil, err
	}
	configPath, err := newWorkflowContextManagedManualCompactionOverlay(runtimeRoot, config.OMPContextMemoryOff)
	if err != nil {
		cleanup()
		return nil, err
	}
	if err := writePipelineOMPActiveModels(runtimeRoot, active); err != nil {
		cleanup()
		return nil, err
	}
	environment, err := pipelineOMPActiveEnvironment(runtimeRoot, configPath, active)
	if err != nil {
		cleanup()
		return nil, err
	}
	args := []string{
		"--mode", "rpc", "--no-session", "--no-extensions", "-e", bridgePath,
		"--session-dir", sessionDir,
		"--cwd", active.backend.ProjectDir, "--model", active.candidate.Provider + "/" + active.candidate.Model,
		"--config", configPath, "--tools", "read,bash,edit,write,grep,glob,todo",
		"--no-skills", "--no-rules", "--no-lsp", "--no-pty", "--no-title",
		"--max-time", pipelineOMPMaxTimeSeconds(active.backend.MaxTime),
	}
	privateExecutable, privateIdentity, err := canonicalPipelineOMPExecutable(privateExecutable)
	if err != nil || privateIdentity.digest != active.backend.executableID.digest {
		cleanup()
		return nil, errors.New("managed active OMP child executable identity is invalid")
	}
	verifiedCommand, err := newPipelineOMPVerifiedExecCommandWithGate(ctx, privateExecutable, privateIdentity, args...)
	if err != nil {
		cleanup()
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, verifiedCommand.Close()) }()
	cmd := verifiedCommand.cmd
	cmd.Dir, cmd.Env, cmd.Stderr = active.backend.ProjectDir,
		workflowContextManagedRPCEnvironment(environment, active.binding), io.Discard
	cmd.WaitDelay = 500 * time.Millisecond
	if err := configurePipelineOMPActiveSandbox(cmd, active.endpoint, active.sandboxMode); err != nil {
		cleanup()
		return nil, err
	}
	if err := configurePipelineOMPVerifiedExecSandboxMode(verifiedCommand, active.sandboxMode, true); err != nil {
		cleanup()
		return nil, err
	}
	if err := configureWorkflowContextManagedRPCProcessGroup(cmd); err != nil {
		cleanup()
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		cleanup()
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		cleanup()
		return nil, err
	}
	if err := verifiedCommand.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		cleanup()
		return nil, fmt.Errorf("start managed active OMP RPC: %w", err)
	}
	frameCtx, stopFrames := context.WithCancel(context.Background())
	frames, done := readPipelineOMPFrames(frameCtx, stdout)
	process := &pipelineOMPProcess{
		cmd: cmd, stdin: stdin, frames: frames, done: done,
		runtimeRoot: runtimeRoot, runtimeInfo: runtimeInfo, stopFrames: stopFrames,
	}
	readyCtx, cancel := context.WithTimeout(ctx, active.backend.MaxTime)
	defer cancel()
	frame, err := process.next(readyCtx)
	if err != nil || frame.Type != "ready" {
		_ = process.Close()
		return nil, errors.New("managed active OMP RPC readiness was not observed")
	}
	return process, nil
}

func materializePipelineOMPActiveBridge(runtimeRoot string) (string, error) {
	identity := ompadapter.ExpectedOMPContextBridgeSourceIdentity()
	source := ompadapter.ExpectedOMPContextBridgeSource()
	if int64(len(source)) != identity.Size || pipelineOMPActiveHash(source) != "sha256:"+identity.SHA256 {
		return "", errors.New("pipeline: embedded managed active bridge identity is invalid")
	}
	extensions := filepath.Join(runtimeRoot, "extensions")
	if err := os.Mkdir(extensions, 0o700); err != nil {
		return "", err
	}
	path := filepath.Join(extensions, "autopus-context.ts")
	if err := os.WriteFile(path, source, 0o600); err != nil {
		return "", err
	}
	actual, err := captureWorkflowContextManagedSourceIdentity(path)
	if err != nil || actual.size != identity.Size || actual.sha256 != identity.SHA256 || actual.mode.Perm() != 0o600 {
		return "", errors.New("pipeline: private managed active bridge identity is invalid")
	}
	return path, nil
}

func pipelineOMPActiveEnvironment(
	runtimeRoot, configPath string,
	active pipelineOMPActiveProcessConfig,
) ([]string, error) {
	paths := map[string]string{
		"HOME": filepath.Join(runtimeRoot, "home"), "TMPDIR": filepath.Join(runtimeRoot, "tmp"),
		"XDG_CACHE_HOME": filepath.Join(runtimeRoot, "cache"), "XDG_CONFIG_HOME": filepath.Join(runtimeRoot, "config"),
		"XDG_DATA_HOME": filepath.Join(runtimeRoot, "data"), "XDG_STATE_HOME": filepath.Join(runtimeRoot, "state"),
	}
	result := []string{"PI_CODING_AGENT_DIR=" + runtimeRoot, "PI_CONFIG_FILES=" + configPath,
		pipelineOMPActiveCredentialKey + "=" + active.credential}
	for key, path := range paths {
		if err := os.Mkdir(path, 0o700); err != nil {
			return nil, err
		}
		result = append(result, key+"="+path)
	}
	if pathValue, found := pipelineOMPEnvironmentValue(active.backend.Environment, "PATH"); found {
		result = append(result, "PATH="+pathValue)
	}
	return result, nil
}

func writePipelineOMPActiveModels(runtimeRoot string, active pipelineOMPActiveProcessConfig) error {
	models := make(map[string]map[string]bool)
	for _, selector := range active.backend.PhaseModels {
		provider, model, ok := strings.Cut(selector, "/")
		if !ok || !safePipelineOMPToken(provider) || !safePipelineOMPToken(model) {
			return errors.New("pipeline: managed active model catalog is invalid")
		}
		if models[provider] == nil {
			models[provider] = make(map[string]bool)
		}
		models[provider][model] = true
	}
	providers := make([]string, 0, len(models))
	for provider := range models {
		providers = append(providers, provider)
	}
	sort.Strings(providers)
	var body strings.Builder
	body.WriteString("providers:\n")
	for _, provider := range providers {
		fmt.Fprintf(&body, "  %s:\n    baseUrl: %s/v1\n    apiKey: %s\n    authHeader: true\n    api: openai-completions\n    models:\n",
			provider, active.endpoint, pipelineOMPActiveCredentialKey)
		ids := make([]string, 0, len(models[provider]))
		for id := range models[provider] {
			ids = append(ids, id)
		}
		sort.Strings(ids)
		for _, id := range ids {
			fmt.Fprintf(&body, "      - id: %s\n        name: Managed Active\n        reasoning: true\n        input: [text]\n        contextWindow: 262144\n        maxTokens: 32768\n", id)
		}
	}
	path := filepath.Join(runtimeRoot, "models.yml")
	if err := os.WriteFile(path, []byte(body.String()), 0o600); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func validatePipelineOMPActiveEndpoint(raw string) (string, error) {
	endpoint, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || endpoint == nil || endpoint.Scheme != "http" || endpoint.User != nil || endpoint.Port() == "" ||
		(endpoint.Path != "" && endpoint.Path != "/") || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return "", errors.New("invalid endpoint")
	}
	ip := net.ParseIP(endpoint.Hostname())
	if ip == nil || !ip.IsLoopback() {
		return "", errors.New("invalid endpoint")
	}
	return strings.TrimSuffix(endpoint.String(), "/"), nil
}

func pipelineOMPEnvironmentValue(environment []string, key string) (string, bool) {
	for _, entry := range environment {
		name, value, ok := strings.Cut(entry, "=")
		if ok && name == key {
			return value, true
		}
	}
	return "", false
}

func pipelineOMPActiveImplementationDigest() string {
	bridge := ompadapter.ExpectedOMPContextBridgeSourceIdentity()
	route := ompadapter.ExpectedOMPNativePipelineRouteSourceIdentity()
	return pipelineOMPActiveHash([]byte(strings.Join([]string{
		pipelineOMPActiveRPCIdentity, pipelineOMPActivePolicyIdentity, bridge.TargetPath, bridge.SHA256,
		route.TargetPath, route.SHA256,
	}, "\x00")))
}
