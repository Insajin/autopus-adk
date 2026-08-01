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

	// Bounds for the --version identity probe, matching pkg/detect's limits so
	// a wedged binary cannot stall a detection sweep.
	ompVersionTimeout   = 5 * time.Second
	ompVersionWaitDelay = 250 * time.Millisecond
)

// Adapter is the oh-my-pi (omp) platform adapter.
type Adapter struct {
	root   string
	engine *tmpl.Engine
}

// New creates an adapter rooted at the current directory.
func New() *Adapter {
	return &Adapter{root: ".", engine: tmpl.New()}
}

// NewWithRoot creates an adapter rooted at the specified path.
func NewWithRoot(root string) *Adapter {
	return &Adapter{root: root, engine: tmpl.New()}
}

func (a *Adapter) Name() string        { return adapterName }
func (a *Adapter) Version() string     { return adapterVer }
func (a *Adapter) CLIBinary() string   { return cliBinary }
func (a *Adapter) SupportsHooks() bool { return false }

// Detect checks whether the omp binary is installed in PATH.
// REQ-019: treat it as the omp platform only when its version output matches the oh-my-pi version shape.
// @AX:NOTE [AUTO]: the "omp/" version prefix is the identity gate, not the binary name — an unrelated
// binary named omp on PATH must not be adopted as this platform. Keep the prefix check paired with LookPath.
// The probe is bounded: a binary named omp that never answers --version would
// otherwise hang `auto doctor` forever, so it inherits the caller's context and
// adds a timeout plus a WaitDelay for output pipes left open after signalling.
func (a *Adapter) Detect(ctx context.Context) (bool, error) {
	path, err := exec.LookPath(cliBinary)
	if err != nil {
		return false, nil
	}
	if ctx == nil {
		ctx = context.Background()
	}

	probeCtx, cancel := context.WithTimeout(ctx, ompVersionTimeout)
	defer cancel()

	cmd := exec.CommandContext(probeCtx, path, "--version")
	cmd.WaitDelay = ompVersionWaitDelay
	// processprobe.Output, not cmd.Output: a probe that leaks its stdout pipe to
	// a grandchild keeps Wait blocked past process exit, and only this helper
	// bounds that drain and terminates the leftover process group.
	out, err := processprobe.Output(cmd)
	if err != nil {
		return false, nil
	}
	version := strings.TrimSpace(string(out))
	if !strings.HasPrefix(version, detect.OMPVersionPrefix) {
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
func (a *Adapter) Generate(ctx context.Context, cfg *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	files, err := a.prepareFiles(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := writeMappings(a.root, files); err != nil {
		return nil, err
	}

	pf := &adapter.PlatformFiles{Files: files, Checksum: adapter.Checksum(fmt.Sprintf("%d", len(files)))}
	m := adapter.ManifestFromFiles(adapterName, pf)
	if err := m.Save(a.root); err != nil {
		return nil, fmt.Errorf("매니페스트 저장 실패: %w", err)
	}
	return pf, nil
}

// Update updates omp files based on the manifest diff.
func (a *Adapter) Update(ctx context.Context, cfg *config.HarnessConfig) (*adapter.PlatformFiles, error) {
	oldManifest, err := adapter.LoadManifest(a.root, adapterName)
	if err != nil {
		return nil, fmt.Errorf("매니페스트 로드 실패: %w", err)
	}

	files, err := a.prepareFiles(ctx, cfg)
	if err != nil {
		return nil, err
	}
	plan, pf := a.buildUpdateTransactionPlan(oldManifest, files, cfg)
	if _, err := adapter.ApplyTransaction(a.root, adapterName, plan); err != nil {
		return nil, err
	}
	return pf, nil
}

func (a *Adapter) prepareFiles(ctx context.Context, cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	if cfg == nil {
		return nil, fmt.Errorf("하네스 설정이 필요합니다")
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
	if err := appendFiles(a.prepareAgentMappings()); err != nil {
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
	// 5. Config
	configMapping, err := a.prepareConfigMapping(cfg)
	if err != nil {
		return nil, err
	}
	files = append(files, configMapping)

	return files, nil
}

func (a *Adapter) buildUpdateTransactionPlan(
	oldManifest *adapter.Manifest,
	files []adapter.FileMapping,
	cfg *config.HarnessConfig,
) (adapter.TransactionPlan, *adapter.PlatformFiles) {
	finalFiles := make([]adapter.FileMapping, 0, len(files))
	for _, file := range files {
		action := adapter.ResolveAction(a.root, file.TargetPath, file.OverwritePolicy, oldManifest)
		if action == adapter.ActionSkip {
			continue
		}
		finalFiles = append(finalFiles, file)
	}

	pf := &adapter.PlatformFiles{
		Files:    finalFiles,
		Checksum: adapter.Checksum(fmt.Sprintf("%d", len(finalFiles))),
	}

	diff := adapter.BuildManifestDiff(oldManifest, files, PruneRoots(cfg))
	removes := adapter.TransactionRemovesFromManifestDiff(diff, false)

	return adapter.TransactionPlan{
		Writes:   adapter.TransactionWritesFromFiles(finalFiles, a.fileModeResolver()),
		Removes:  removes,
		Manifest: adapter.ManifestFromFiles(adapterName, pf),
	}, pf
}

// ompConfigFileMode is the permission a freshly created .omp/config.yml gets.
// The file carries provider settings and can hold credentials, so it starts
// owner-only instead of world readable.
const ompConfigFileMode = os.FileMode(0600)

// fileModeResolver decides the permission each managed file is written with.
// Everything except the config keeps the ordinary 0644. The config preserves
// whatever mode it already has, because the update transaction otherwise
// rewrote a user-tightened 0600 file back to 0644 and silently widened it.
func (a *Adapter) fileModeResolver() func(string) os.FileMode {
	return func(path string) os.FileMode {
		if filepath.ToSlash(path) != configFile {
			return 0644
		}
		if info, err := os.Stat(filepath.Join(a.root, path)); err == nil {
			return info.Mode().Perm()
		}
		return ompConfigFileMode
	}
}

func writeMappings(root string, files []adapter.FileMapping) error {
	for _, file := range files {
		if err := writeMapping(root, file); err != nil {
			return err
		}
	}
	return nil
}

func writeMapping(root string, file adapter.FileMapping) error {
	// Contain the write: refuse absolute paths, `..` segments that escape the
	// workspace, and any path crossing a symlink. Update already enforces this
	// through the transaction path; Generate writes the same file set and must
	// not be the weaker door.
	targetPath, err := adapter.SafeWorkspacePath(root, file.TargetPath)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return fmt.Errorf("디렉터리 생성 실패 %s: %w", filepath.Dir(targetPath), err)
	}
	perm := os.FileMode(0644)
	if filepath.ToSlash(file.TargetPath) == configFile {
		perm = ompConfigFileMode
		if info, statErr := os.Stat(targetPath); statErr == nil {
			perm = info.Mode().Perm()
		}
	}

	// No marker-policy special case here: prepareConfigMapping already merged the
	// managed section into file.Content, so both policies write the same bytes.
	if err := os.WriteFile(targetPath, file.Content, perm); err != nil {
		return fmt.Errorf("파일 쓰기 실패 %s: %w", file.TargetPath, err)
	}
	return nil
}
