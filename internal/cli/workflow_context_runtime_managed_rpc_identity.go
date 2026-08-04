package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	ompadapter "github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/processprobe"
)

type workflowContextManagedSourceIdentity struct {
	path   string
	info   fs.FileInfo
	size   int64
	mode   fs.FileMode
	sha256 string
}

func captureWorkflowContextManagedProjectIdentity(projectDir string) (fs.FileInfo, error) {
	if projectDir == "" {
		return nil, nil
	}
	info, err := os.Lstat(projectDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("managed OMP project directory identity is invalid")
	}
	return info, nil
}

func verifyWorkflowContextManagedProjectIdentity(projectDir string, expected fs.FileInfo) error {
	if projectDir == "" && expected == nil {
		return nil
	}
	actual, err := os.Lstat(projectDir)
	if err != nil || expected == nil || !actual.IsDir() || actual.Mode()&os.ModeSymlink != 0 ||
		!os.SameFile(expected, actual) {
		return errors.New("managed OMP project directory identity drifted")
	}
	return validateWorkflowContextManagedProjectExtensions(projectDir)
}

func captureWorkflowContextManagedSourceIdentity(path string) (workflowContextManagedSourceIdentity, error) {
	entry, err := os.Lstat(path)
	if err != nil || !entry.Mode().IsRegular() || entry.Mode()&os.ModeSymlink != 0 {
		return workflowContextManagedSourceIdentity{}, errors.New("managed OMP source identity is invalid")
	}
	file, err := os.Open(path)
	if err != nil {
		return workflowContextManagedSourceIdentity{}, fmt.Errorf("open managed OMP source: %w", err)
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(entry, opened) {
		return workflowContextManagedSourceIdentity{}, errors.New("managed OMP source changed while opening")
	}
	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return workflowContextManagedSourceIdentity{}, fmt.Errorf("hash managed OMP source: %w", err)
	}
	return workflowContextManagedSourceIdentity{
		path: path, info: opened, size: opened.Size(), mode: opened.Mode(),
		sha256: hex.EncodeToString(hash.Sum(nil)),
	}, nil
}

func verifyWorkflowContextManagedSourceIdentity(expected workflowContextManagedSourceIdentity) error {
	actual, err := captureWorkflowContextManagedSourceIdentity(expected.path)
	if err != nil || !os.SameFile(expected.info, actual.info) || actual.size != expected.size ||
		actual.mode != expected.mode || actual.sha256 != expected.sha256 {
		return errors.New("managed OMP source identity drifted")
	}
	return nil
}

func captureWorkflowContextManagedSourceIdentities(
	options WorkflowContextManagedRPCOptions,
) ([]workflowContextManagedSourceIdentity, error) {
	bridge := ompadapter.ExpectedOMPContextBridgeSourceIdentity()
	route := ompadapter.ExpectedOMPNativePipelineRouteSourceIdentity()
	paths := []string{options.Executable, options.ConfigPath,
		filepath.Join(options.Workspace, filepath.FromSlash(bridge.TargetPath)),
		filepath.Join(options.Workspace, filepath.FromSlash(route.TargetPath))}
	models := filepath.Join(options.RuntimeRoot, "models.yml")
	if _, err := os.Lstat(models); err == nil {
		paths = append(paths, models)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("inspect managed OMP model config: %w", err)
	}
	identities := make([]workflowContextManagedSourceIdentity, 0, len(paths))
	for _, path := range paths {
		identity, err := captureWorkflowContextManagedSourceIdentity(path)
		if err != nil {
			return nil, err
		}
		identities = append(identities, identity)
	}
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: indices 2 and 3 are exact generated extensions after executable/config.
	if identities[2].size != bridge.Size || identities[2].sha256 != bridge.SHA256 ||
		identities[3].size != route.Size || identities[3].sha256 != route.SHA256 {
		return nil, errors.New("managed OMP generated extension identity is invalid")
	}
	return identities, nil
}

func (driver *WorkflowContextManagedRPCDriver) verifyManagedSourceIdentities() error {
	if driver == nil {
		return errors.New("managed OMP driver is required")
	}
	driver.mu.Lock()
	identities := append([]workflowContextManagedSourceIdentity(nil), driver.sourceIdentities...)
	workspace := driver.options.Workspace
	projectDir, projectInfo := driver.options.ProjectDir, driver.projectInfo
	driver.mu.Unlock()
	if err := verifyWorkflowContextManagedProjectIdentity(projectDir, projectInfo); err != nil {
		return err
	}
	if projectDir != "" {
		driver.mu.Lock()
		options := driver.options
		driver.mu.Unlock()
		if err := validateWorkflowContextManagedProductSurface(options); err != nil {
			return err
		}
	}
	if err := validateWorkflowContextManagedExtensionAllowlist(workspace); err != nil {
		return err
	}
	for _, identity := range identities {
		if err := verifyWorkflowContextManagedSourceIdentity(identity); err != nil {
			return err
		}
	}
	return nil
}

func (driver *WorkflowContextManagedRPCDriver) verifyInstalledIdentity(
	ctx context.Context, expected string,
) error {
	if driver == nil || !installedOMPVersionPattern.MatchString(expected) {
		return errors.New("managed OMP expected identity is invalid")
	}
	driver.mu.Lock()
	options := driver.options
	driver.mu.Unlock()
	if err := driver.verifyManagedSourceIdentities(); err != nil {
		return err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, options.Executable, "--version")
	cmd.Dir = options.Workspace
	cmd.Env = append([]string(nil), options.Environment...)
	if _, err := configureWorkflowContextManagedRPCSandbox(cmd, options.AllowedEndpoint); err != nil {
		return err
	}
	output, err := processprobe.OutputLimited(cmd, 4<<10)
	if err != nil {
		return errors.New("managed OMP identity probe failed")
	}
	observed := strings.TrimSpace(string(output))
	if observed != expected || !installedOMPVersionPattern.MatchString(observed) {
		return errors.New("managed OMP executable identity does not match capability evidence")
	}
	return nil
}
