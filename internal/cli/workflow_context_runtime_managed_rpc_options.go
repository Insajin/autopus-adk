package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	ompadapter "github.com/insajin/autopus-adk/pkg/adapter/omp"
)

var workflowContextManagedReservedEnvironment = map[string]struct{}{
	"AUTOPUS_OMP_CONTEXT_BINDING_HASH":   {},
	"AUTOPUS_OMP_CONTEXT_OPTIONS_HASH":   {},
	"AUTOPUS_OMP_CONTEXT_SESSION_HASH":   {},
	"AUTOPUS_OMP_CONTEXT_NONCE_HASH":     {},
	"AUTOPUS_OMP_CONTEXT_ACK_TIMEOUT_MS": {},
}

// @AX:WARN [AUTO]: managed RPC option validation contains path, mode, environment, and generated-source checks.
// @AX:REASON [AUTO]: all process authority must be canonical and unambiguous before a runtime lease is acquired.
func validateWorkflowContextManagedRPCOptions(
	options WorkflowContextManagedRPCOptions,
) (WorkflowContextManagedRPCOptions, error) {
	for name, value := range map[string]string{
		"executable": options.Executable, "workspace": options.Workspace, "runtime base": options.RuntimeBase,
		"runtime root": options.RuntimeRoot, "session directory": options.SessionDir,
		"config": options.ConfigPath, "model": options.Model, "endpoint": options.AllowedEndpoint,
	} {
		if strings.TrimSpace(value) == "" {
			return options, fmt.Errorf("managed OMP %s is required", name)
		}
	}
	paths := []struct {
		value        *string
		allowSymlink bool
	}{
		{&options.Executable, true}, {&options.Workspace, false}, {&options.RuntimeBase, false},
		{&options.RuntimeRoot, false}, {&options.SessionDir, false}, {&options.ConfigPath, false},
	}
	for _, path := range paths {
		canonical, err := canonicalWorkflowContextManagedPath(*path.value, path.allowSymlink)
		if err != nil {
			return options, err
		}
		*path.value = canonical
	}
	if err := validateWorkflowContextManagedPathLayout(options); err != nil {
		return options, err
	}
	if err := validateWorkflowContextManagedPathModes(options); err != nil {
		return options, err
	}
	environment, err := validateWorkflowContextManagedEnvironment(options)
	if err != nil {
		return options, err
	}
	options.Environment = environment
	if err := validateWorkflowContextManagedExtensionAllowlist(options.Workspace); err != nil {
		return options, err
	}
	if options.MaxTime <= 0 || options.MaxTime > 2*time.Minute {
		options.MaxTime = 45 * time.Second
	}
	return options, nil
}

func canonicalWorkflowContextManagedPath(path string, allowFinalSymlink bool) (string, error) {
	absolute, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve managed OMP path: %w", err)
	}
	absolute = filepath.Clean(absolute)
	if !allowFinalSymlink {
		info, err := os.Lstat(absolute)
		if err != nil || info.Mode()&os.ModeSymlink != 0 {
			return "", errors.New("managed OMP path identity is invalid")
		}
	}
	resolved, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("resolve managed OMP canonical path: %w", err)
	}
	return filepath.Clean(resolved), nil
}

func validateWorkflowContextManagedPathLayout(options WorkflowContextManagedRPCOptions) error {
	if options.RuntimeBase == string(filepath.Separator) || options.RuntimeRoot == string(filepath.Separator) ||
		filepath.Dir(options.RuntimeRoot) == options.RuntimeRoot {
		return errors.New("managed OMP runtime root is unsafe")
	}
	if relative, err := filepath.Rel(options.RuntimeBase, options.RuntimeRoot); err != nil || relative != "omp-runtime" {
		return errors.New("managed OMP runtime root is not the allowed base child")
	}
	if relative, err := filepath.Rel(options.RuntimeBase, options.Workspace); err != nil || relative != "workspace" {
		return errors.New("managed OMP workspace is not the isolated base child")
	}
	if !workflowContextManagedPathBelow(options.RuntimeRoot, options.SessionDir) {
		return errors.New("managed OMP session directory is not runtime-owned")
	}
	if !workflowContextManagedPathBelow(options.RuntimeRoot, options.ConfigPath) {
		return errors.New("managed OMP config is not runtime-owned")
	}
	return nil
}

func workflowContextManagedPathBelow(root, path string) bool {
	relative, err := filepath.Rel(root, path)
	return err == nil && relative != "." && relative != ".." &&
		!strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func validateWorkflowContextManagedPathModes(options WorkflowContextManagedRPCOptions) error {
	for _, path := range []string{options.Workspace, options.RuntimeRoot, options.SessionDir} {
		info, err := os.Lstat(path)
		if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm()&0o077 != 0 {
			return errors.New("managed OMP directory identity or mode is invalid")
		}
	}
	executable, err := os.Lstat(options.Executable)
	if err != nil || !executable.Mode().IsRegular() || executable.Mode()&0o111 == 0 {
		return errors.New("managed OMP executable identity is invalid")
	}
	configInfo, err := os.Lstat(options.ConfigPath)
	if err != nil || !configInfo.Mode().IsRegular() || configInfo.Mode()&os.ModeSymlink != 0 ||
		configInfo.Mode().Perm()&^fs.FileMode(0o600) != 0 {
		return errors.New("managed OMP config identity or mode is invalid")
	}
	return nil
}

func validateWorkflowContextManagedEnvironment(
	options WorkflowContextManagedRPCOptions,
) ([]string, error) {
	seen := make(map[string]struct{}, len(options.Environment))
	result := append([]string(nil), options.Environment...)
	required := map[string]string{"PI_CODING_AGENT_DIR": options.RuntimeRoot, "PI_CONFIG_FILES": options.ConfigPath}
	for index, entry := range result {
		key, value, found := strings.Cut(entry, "=")
		if !found || !validWorkflowContextManagedEnvironmentKey(key) || strings.ContainsRune(value, '\x00') {
			return nil, errors.New("managed OMP environment entry is malformed")
		}
		if _, duplicate := seen[key]; duplicate {
			return nil, fmt.Errorf("managed OMP environment key is duplicated: %s", key)
		}
		seen[key] = struct{}{}
		if _, reserved := workflowContextManagedReservedEnvironment[key]; reserved {
			return nil, fmt.Errorf("managed OMP bridge environment key is reserved: %s", key)
		}
		if expected, requiredKey := required[key]; requiredKey {
			canonical, err := canonicalWorkflowContextManagedPath(value, false)
			if err != nil || canonical != expected {
				return nil, errors.New("managed OMP environment is not bound to the task runtime")
			}
			result[index] = key + "=" + expected
		}
	}
	for key := range required {
		if _, ok := seen[key]; !ok {
			return nil, errors.New("managed OMP environment is missing a task runtime key")
		}
	}
	return result, nil
}

func validWorkflowContextManagedEnvironmentKey(key string) bool {
	if key == "" || !((key[0] >= 'A' && key[0] <= 'Z') || (key[0] >= 'a' && key[0] <= 'z') || key[0] == '_') {
		return false
	}
	for index := 1; index < len(key); index++ {
		char := key[index]
		if !((char >= 'A' && char <= 'Z') || (char >= 'a' && char <= 'z') ||
			(char >= '0' && char <= '9') || char == '_') {
			return false
		}
	}
	return true
}

func validateWorkflowContextManagedExtensionAllowlist(workspace string) error {
	expected := ompadapter.ExpectedOMPContextBridgeSourceIdentity()
	ompDirectory := filepath.Join(workspace, ".omp")
	ompInfo, err := os.Lstat(ompDirectory)
	if err != nil || !ompInfo.IsDir() || ompInfo.Mode()&os.ModeSymlink != 0 {
		return errors.New("managed OMP workspace extension root identity is invalid")
	}
	directory := filepath.Join(workspace, filepath.Dir(filepath.FromSlash(expected.TargetPath)))
	info, err := os.Lstat(directory)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("managed OMP extension directory identity is invalid")
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 1 || entries[0].Name() != filepath.Base(expected.TargetPath) ||
		entries[0].Type()&os.ModeSymlink != 0 || !entries[0].Type().IsRegular() {
		return errors.New("managed OMP extension allowlist is invalid")
	}
	return nil
}

func validateWorkflowContextManagedBinding(binding WorkflowContextBridgeBinding) error {
	if binding.SchemaVersion != workflowContextBridgeSchemaVersion {
		return errors.New("managed OMP bridge schema is invalid")
	}
	for _, value := range []string{binding.BindingHash, binding.OptionsHash, binding.SessionHash, binding.NonceHash} {
		if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
			return errors.New("managed OMP bridge hash is invalid")
		}
	}
	return nil
}
