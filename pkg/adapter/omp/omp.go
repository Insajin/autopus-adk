package omp

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/detect"
	"github.com/insajin/autopus-adk/pkg/processprobe"
	tmpl "github.com/insajin/autopus-adk/pkg/template"
)

const (
	adapterName = "omp"
	cliBinary   = "omp"
	adapterVer  = "1.0.0"

	// Timeout and pipe-drain bounds match pkg/detect; the output budget matches
	// the metadata-only OMP probe policy so detection cannot buffer unbounded
	// output from an unverified PATH executable.
	ompVersionTimeout   = 5 * time.Second
	ompVersionWaitDelay = 250 * time.Millisecond
	ompVersionMaxOutput = 64 * 1024
)

// ompVersionCeiling is the deadline Detect actually enforces. A test widens it
// so a saturated machine cannot preempt the identity contract under test: the
// probe spawns a process and the production bound, not the assertion, becomes
// what a tight run measures. The shipped default never moves.
var ompVersionCeiling = ompVersionTimeout

// Adapter is the oh-my-pi (omp) platform adapter.
type Adapter struct {
	root                   string
	engine                 *tmpl.Engine
	modelIntegrationRunner OMPModelCatalogRunner
	modelIntegrationClock  func() time.Time
	rootedWorkspaceHook    func()
	rootedRootCreatedHook  func()
}

// New creates an adapter rooted at the current directory.
func New() *Adapter {
	return NewWithRoot(".")
}

// NewWithRoot creates an adapter rooted at the specified path.
func NewWithRoot(root string) *Adapter {
	return &Adapter{
		root: root, engine: tmpl.New(),
		modelIntegrationRunner: newOMPModelIntegrationExecRunner(),
		modelIntegrationClock:  time.Now,
	}
}

func (a *Adapter) Name() string        { return adapterName }
func (a *Adapter) Version() string     { return adapterVer }
func (a *Adapter) CLIBinary() string   { return cliBinary }
func (a *Adapter) SupportsHooks() bool { return false }

// Detect checks whether the omp binary is installed in PATH.
// REQ-019: treat it as the omp platform only when its version output matches the oh-my-pi version shape.
// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-002: exact "omp/X.Y.Z" release output is the identity gate,
// not the binary name; keep the whole-string release check paired with LookPath.
// The probe is bounded and credential-free: a binary named omp that never
// answers --version cannot stall detection, and an impostor cannot inherit
// provider credentials before its version output proves its identity.
func (a *Adapter) Detect(ctx context.Context) (bool, error) {
	path, err := exec.LookPath(cliBinary)
	if err != nil {
		return false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	probeCtx, cancel := context.WithTimeout(ctx, ompVersionCeiling)
	defer cancel()
	sandbox, err := os.MkdirTemp("", "autopus-omp-detect-*")
	if err != nil {
		return false, nil
	}
	defer func() { _ = os.RemoveAll(sandbox) }()
	for _, name := range []string{"tmp", "config", "data", "state", "cache"} {
		if err := os.Mkdir(filepath.Join(sandbox, name), 0o700); err != nil {
			return false, nil
		}
	}
	home, err := safeOMPModelProbeDirectory(os.Getenv("HOME"), "HOME")
	if err != nil {
		return false, nil
	}
	profile := ""
	if locator := os.Getenv("PI_CODING_AGENT_DIR"); locator != "" {
		profile, err = safeOMPModelProbeDirectory(locator, "PI_CODING_AGENT_DIR")
		if err != nil {
			return false, nil
		}
	}

	cmd := exec.CommandContext(probeCtx, path, "--version")
	cmd.WaitDelay = ompVersionWaitDelay
	cmd.Dir = sandbox
	cmd.Env = []string{
		"HOME=" + home,
		"TMPDIR=" + filepath.Join(sandbox, "tmp"),
		"XDG_CONFIG_HOME=" + filepath.Join(sandbox, "config"),
		"XDG_DATA_HOME=" + filepath.Join(sandbox, "data"),
		"XDG_STATE_HOME=" + filepath.Join(sandbox, "state"),
		"XDG_CACHE_HOME=" + filepath.Join(sandbox, "cache"),
		"LANG=C", "LC_ALL=C",
	}
	if profile != "" {
		cmd.Env = append(cmd.Env, "PI_CODING_AGENT_DIR="+profile)
	}
	// OutputLimited also terminates a leaked process group when either stream
	// exceeds the version probe budget or remains open after the child exits.
	out, err := processprobe.OutputLimited(cmd, ompVersionMaxOutput)
	if err != nil {
		return false, nil
	}
	version := strings.TrimSpace(string(out))
	if !detect.OMPVersionMatchesIdentity(version) {
		return false, nil
	}
	return true, nil
}

// InstallHooks is a no-op for omp.
func (a *Adapter) InstallHooks(_ context.Context, _ []adapter.HookConfig, _ *adapter.PermissionSet) error {
	return nil
}

// Generate creates omp platform files based on the harness config.
// @AX:ANCHOR [AUTO]: platform entry point reached from 7 CLI dispatch sites (init, platform add,
// platform update, update preview, doctor text/json, drift) — do not change the signature.
// @AX:REASON [AUTO]: every caller relies on the manifest being written here; a Generate that skips
// the manifest leaves Update and Clean with no record of the files it just wrote.
func (a *Adapter) Generate(ctx context.Context, cfg *config.HarnessConfig) (pf *adapter.PlatformFiles, returnErr error) {
	workspace, err := openOrCreateOMPRootedWorkspace(a.root, a.rootedRootCreatedHook)
	if err != nil {
		return nil, err
	}
	defer func() { joinOMPRootedCloseError(&returnErr, workspace.Close()) }()
	if a.rootedWorkspaceHook != nil {
		a.rootedWorkspaceHook()
	}
	files, err := a.prepareFilesAt(ctx, cfg, workspace)
	if err != nil {
		return nil, err
	}

	pf = &adapter.PlatformFiles{Files: files, Checksum: adapter.Checksum(fmt.Sprintf("%d", len(files)))}
	resolveMode := a.fileModeResolverAt(workspace)
	writes := make([]adapter.TransactionWrite, 0, len(files))
	for _, file := range files {
		perm := resolveMode(file.TargetPath)
		unchanged, err := ompRootedMappingUnchanged(workspace, file, perm)
		if err != nil {
			return nil, err
		}
		if !unchanged {
			writes = append(writes, adapter.TransactionWrite{
				Path: file.TargetPath, Content: file.Content, Perm: perm,
			})
		}
	}
	manifest := adapter.ManifestFromFiles(adapterName, pf)
	oldManifest, err := loadOMPManifestAt(workspace, adapterName)
	if err != nil {
		return nil, fmt.Errorf("매니페스트 로드 실패: %w", err)
	}
	if len(writes) == 0 && ompRootedManifestUnchanged(workspace, oldManifest, manifest) {
		manifest = nil
	}
	plan := adapter.TransactionPlan{Writes: writes, Manifest: manifest}
	if _, err := applyOMPTransactionAt(workspace, adapterName, plan); err != nil {
		return nil, err
	}
	return pf, nil
}

// Update updates omp files based on the manifest diff.
func (a *Adapter) Update(ctx context.Context, cfg *config.HarnessConfig) (pf *adapter.PlatformFiles, returnErr error) {
	workspace, err := openOMPRootedWorkspace(a.root)
	if err != nil {
		return nil, err
	}
	defer func() { joinOMPRootedCloseError(&returnErr, workspace.Close()) }()
	if a.rootedWorkspaceHook != nil {
		a.rootedWorkspaceHook()
	}
	oldManifest, err := loadOMPManifestAt(workspace, adapterName)
	if err != nil {
		return nil, fmt.Errorf("매니페스트 로드 실패: %w", err)
	}

	files, err := a.prepareFilesAt(ctx, cfg, workspace)
	if err != nil {
		return nil, err
	}
	plan, pf, err := a.buildUpdateTransactionPlanAt(workspace, oldManifest, files, cfg)
	if err != nil {
		return nil, err
	}
	if _, err := applyOMPTransactionAt(workspace, adapterName, plan); err != nil {
		return nil, err
	}
	return pf, nil
}

func (a *Adapter) prepareFiles(ctx context.Context, cfg *config.HarnessConfig) (files []adapter.FileMapping, returnErr error) {
	workspace, err := openOMPRootedWorkspace(a.root)
	if err != nil {
		return nil, err
	}
	defer func() { joinOMPRootedCloseError(&returnErr, workspace.Close()) }()
	return a.prepareFilesAt(ctx, cfg, workspace)
}

// @AX:WARN [AUTO]: OMP surface preparation contains 12 if branches.
// @AX:REASON [AUTO]: model integration, generated mappings, context bridge, config merge, and runtime finalization converge here.
func (a *Adapter) prepareFilesAt(
	ctx context.Context,
	cfg *config.HarnessConfig,
	workspace *ompRootedWorkspace,
) ([]adapter.FileMapping, error) {
	if cfg == nil {
		return nil, fmt.Errorf("하네스 설정이 필요합니다")
	}

	modelIntegration, err := a.prepareModelIntegration(ctx, cfg)
	if err != nil {
		return nil, err
	}

	var files []adapter.FileMapping
	appendFiles := func(items []adapter.FileMapping, err error) error {
		if err != nil {
			return err
		}
		files = append(files, items...)
		return nil
	}

	// 1. Rules
	if err := appendFiles(a.prepareRuleMappings()); err != nil {
		return nil, err
	}
	// 2. Agents
	agentMappings := a.prepareAgentMappings
	if modelIntegration != nil {
		agentMappings = modelIntegration.prepareAgentMappings
	}
	if err := appendFiles(agentMappings()); err != nil {
		return nil, err
	}
	// 3. Skills
	if err := appendFiles(a.prepareSkillMappings(cfg)); err != nil {
		return nil, err
	}
	// 4. Commands
	if err := appendFiles(a.prepareCommandMappings(cfg)); err != nil {
		return nil, err
	}
	// 5. Native context bridge (explicit OMP context-policy opt-in only)
	if err := appendFiles(prepareOMPContextBridgeMappings(cfg)); err != nil {
		return nil, err
	}
	// 6. Config
	configMapping, err := a.prepareConfigMappingAt(workspace, cfg)
	if err != nil {
		return nil, err
	}
	if modelIntegration != nil {
		configMapping, runtimeMappings, finalizeErr := modelIntegration.finalize(ctx, a, workspace, configMapping)
		if finalizeErr != nil {
			return nil, finalizeErr
		}
		files = append(files, configMapping)
		files = append(files, runtimeMappings...)
	} else {
		files = append(files, configMapping)
	}

	return files, nil
}

// ompConfigFileMode is the permission a freshly created .omp/config.yml gets.
// The file carries provider settings and can hold credentials, so it starts
// owner-only instead of world readable.
const ompConfigFileMode = os.FileMode(0600)
