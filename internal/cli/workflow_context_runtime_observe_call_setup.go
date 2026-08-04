package cli

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	ompadapter "github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/processprobe"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

type workflowContextObserveCallSetup struct {
	taskRoot   string
	projectDir string
	delivery   promptlayer.ContextDeliveryResult
	options    WorkflowContextManagedRPCOptions
	version    string
}

func prepareWorkflowContextObserveCall(
	ctx context.Context,
	request workflowContextObserveCallRequest,
	options workflowContextObserveCallOptions,
) (workflowContextObserveCallSetup, error) {
	projectDir, err := canonicalWorkflowContextObserveProject(options.ProjectDir)
	if err != nil {
		return workflowContextObserveCallSetup{}, err
	}
	specDir := filepath.ToSlash(filepath.Join(".autopus", "specs", options.SpecID))
	delivery, err := promptlayer.BuildContextDelivery(promptlayer.ContextDeliveryOptions{
		Root: projectDir, Command: "go", SpecDir: specDir,
	})
	if err != nil {
		return workflowContextObserveCallSetup{}, fmt.Errorf("build observe-call canonical delivery: %w", err)
	}
	executable, err := exec.LookPath(options.Executable)
	if err != nil {
		return workflowContextObserveCallSetup{}, errors.New("observe-call OMP executable is unavailable")
	}
	endpoint, err := validateWorkflowContextObserveEndpoint(options.Endpoint)
	if err != nil {
		return workflowContextObserveCallSetup{}, err
	}
	credential, found := os.LookupEnv(options.CredentialLocator)
	if !found || strings.TrimSpace(credential) == "" || strings.ContainsRune(credential, 0) {
		return workflowContextObserveCallSetup{}, errors.New("observe-call dedicated credential is unavailable")
	}
	taskRoot, err := os.MkdirTemp("", "autopus-omp-observe-")
	if err != nil {
		return workflowContextObserveCallSetup{}, fmt.Errorf("create observe-call task root: %w", err)
	}
	setup := workflowContextObserveCallSetup{taskRoot: taskRoot}
	failed := true
	defer func() {
		if failed {
			_ = os.RemoveAll(taskRoot)
		}
	}()
	if err := os.Chmod(taskRoot, 0o700); err != nil {
		return setup, err
	}
	runtimeBase := filepath.Join(taskRoot, "runtime-base")
	workspace := filepath.Join(runtimeBase, "workspace")
	runtimeRoot := filepath.Join(runtimeBase, "omp-runtime")
	sessionDir := filepath.Join(runtimeRoot, "sessions", "observe")
	isolatedProject := filepath.Join(taskRoot, "project")
	paths := []string{runtimeBase, workspace, runtimeRoot, sessionDir, isolatedProject}
	for _, path := range paths {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return setup, fmt.Errorf("create observe-call runtime: %w", err)
		}
		if err := os.Chmod(path, 0o700); err != nil {
			return setup, err
		}
	}
	if err := copyWorkflowContextObserveDocuments(projectDir, isolatedProject, delivery.RequiredDocuments); err != nil {
		return setup, err
	}
	harness := workflowContextObserveHarnessConfig(options.SpecID)
	if err := config.Save(isolatedProject, harness); err != nil {
		return setup, fmt.Errorf("write observe-call harness config: %w", err)
	}
	if _, err := ompadapter.NewWithRoot(workspace).Generate(ctx, harness); err != nil {
		return setup, fmt.Errorf("generate observe-call workspace surface: %w", err)
	}
	if _, err := ompadapter.NewWithRoot(isolatedProject).Generate(ctx, harness); err != nil {
		return setup, fmt.Errorf("generate observe-call project surface: %w", err)
	}
	_, configPath, err := newWorkflowContextProductOverlay(runtimeRoot, config.OMPContextMemoryOff)
	if err != nil {
		return setup, err
	}
	if err := writeWorkflowContextObserveModels(runtimeRoot, options, endpoint); err != nil {
		return setup, err
	}
	for _, name := range []string{"home", "tmp", "cache", "config", "data", "state"} {
		path := filepath.Join(runtimeRoot, name)
		if err := os.MkdirAll(path, 0o700); err != nil {
			return setup, err
		}
		if err := os.Chmod(path, 0o700); err != nil {
			return setup, err
		}
	}
	environment := []string{
		"PI_CODING_AGENT_DIR=" + runtimeRoot, "PI_CONFIG_FILES=" + configPath,
		options.CredentialLocator + "=" + credential, "HOME=" + filepath.Join(runtimeRoot, "home"),
		"TMPDIR=" + filepath.Join(runtimeRoot, "tmp"), "XDG_CACHE_HOME=" + filepath.Join(runtimeRoot, "cache"),
		"XDG_CONFIG_HOME=" + filepath.Join(runtimeRoot, "config"), "XDG_DATA_HOME=" + filepath.Join(runtimeRoot, "data"),
		"XDG_STATE_HOME=" + filepath.Join(runtimeRoot, "state"), "PATH=" + os.Getenv("PATH"),
	}
	managed := WorkflowContextManagedRPCOptions{
		Executable: executable, ProjectDir: isolatedProject, Workspace: workspace, RuntimeBase: runtimeBase,
		RuntimeRoot: runtimeRoot, SessionDir: sessionDir, ConfigPath: configPath,
		Model: options.Provider + "/" + options.Model, AllowedEndpoint: endpoint,
		Environment: environment, MaxTime: 2 * time.Minute, CaptureOutput: true, CaptureStats: true,
	}
	version, err := probeWorkflowContextObserveVersion(ctx, managed)
	if err != nil {
		return setup, err
	}
	isolatedDelivery, err := promptlayer.BuildContextDelivery(promptlayer.ContextDeliveryOptions{
		Root: isolatedProject, Command: "go", SpecDir: specDir,
	})
	if err != nil {
		return setup, fmt.Errorf("rebuild observe-call isolated delivery: %w", err)
	}
	setup.projectDir, setup.delivery = isolatedProject, isolatedDelivery
	setup.options, setup.version = managed, version
	failed = false
	return setup, nil
}

func canonicalWorkflowContextObserveProject(path string) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", errors.New("observe-call project directory is invalid")
	}
	canonical, err := filepath.EvalSymlinks(filepath.Clean(absolute))
	if err != nil {
		return "", errors.New("observe-call project directory is invalid")
	}
	info, err := os.Lstat(canonical)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return "", errors.New("observe-call project directory is invalid")
	}
	return canonical, nil
}

func validateWorkflowContextObserveEndpoint(raw string) (string, error) {
	endpoint, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || endpoint == nil || endpoint.Scheme != "http" || endpoint.User != nil ||
		(endpoint.Path != "" && endpoint.Path != "/") || endpoint.RawQuery != "" || endpoint.Fragment != "" {
		return "", errors.New("observe-call task endpoint is invalid")
	}
	ip := net.ParseIP(endpoint.Hostname())
	if ip == nil || !ip.IsLoopback() || endpoint.Port() == "" {
		return "", errors.New("observe-call task endpoint is invalid")
	}
	return strings.TrimSuffix(endpoint.String(), "/"), nil
}

func copyWorkflowContextObserveDocuments(
	sourceRoot, targetRoot string,
	documents []promptlayer.ContextDeliveryDocument,
) error {
	for _, document := range documents {
		source := filepath.Join(sourceRoot, filepath.FromSlash(document.SourceRef))
		info, err := os.Lstat(source)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() > 1<<20 {
			return errors.New("observe-call canonical source identity is invalid")
		}
		body, err := os.ReadFile(source)
		if err != nil {
			return errors.New("observe-call canonical source is unavailable")
		}
		target := filepath.Join(targetRoot, filepath.FromSlash(document.SourceRef))
		if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
			return err
		}
		if err := os.WriteFile(target, body, 0o600); err != nil {
			return err
		}
	}
	return nil
}

func workflowContextObserveHarnessConfig(specID string) *config.HarnessConfig {
	result := config.DefaultFullConfig("omp-observe-" + strings.ToLower(specID))
	result.Platforms = []string{"omp"}
	result.OMPContextPolicy = config.OMPContextPolicyConf{Profile: "active", Profiles: map[string]config.OMPContextProfileConf{
		"active": {
			HistoryMode: config.OMPContextHistoryActive, MemoryMode: config.OMPContextMemoryOff,
			HistoryTargetTokens: config.DefaultOMPContextHistoryTargetTokens,
			Fallback:            config.OMPContextFallbackCanonicalFull, CapabilityPolicy: config.OMPContextCapabilityProbeRequired,
			RuntimeRootPolicy: config.OMPContextRuntimeIsolatedTaskOwned, MutationScope: config.OMPContextMutationSessionOverlay,
		},
	}}
	return result
}

func writeWorkflowContextObserveModels(
	runtimeRoot string,
	options workflowContextObserveCallOptions,
	endpoint string,
) error {
	body := fmt.Sprintf("providers:\n  %s:\n    baseUrl: %s/v1\n    apiKey: %s\n    authHeader: true\n    api: openai-completions\n    models:\n      - id: %s\n        name: Observe Call\n        reasoning: true\n        input: [text]\n        contextWindow: 262144\n        maxTokens: 32768\n",
		options.Provider, endpoint, options.CredentialLocator, options.Model)
	path := filepath.Join(runtimeRoot, "models.yml")
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		return fmt.Errorf("write observe-call model authority: %w", err)
	}
	return os.Chmod(path, 0o600)
}

func probeWorkflowContextObserveVersion(ctx context.Context, options WorkflowContextManagedRPCOptions) (string, error) {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, options.Executable, "--version")
	cmd.Dir, cmd.Env = options.Workspace, options.Environment
	if _, err := configureWorkflowContextManagedRPCSandbox(cmd, options.AllowedEndpoint); err != nil {
		return "", err
	}
	output, err := processprobe.OutputLimited(cmd, 4<<10)
	version := strings.TrimSpace(string(output))
	if err != nil || !installedOMPVersionPattern.MatchString(version) {
		return "", errors.New("observe-call OMP identity probe failed")
	}
	return version, nil
}

func workflowContextObservePathGone(path string) bool {
	_, err := os.Lstat(path)
	return errors.Is(err, fs.ErrNotExist)
}
