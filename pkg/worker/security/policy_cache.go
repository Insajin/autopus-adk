package security

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// PolicyCache provides file-based storage for SecurityPolicy objects.
// Policies are stored in a per-user temp directory for isolation.
type PolicyCache struct {
	dir string
}

// NewPolicyCache creates a PolicyCache using a secure per-user directory.
func NewPolicyCache() *PolicyCache {
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("autopus-%d", os.Getuid()))
	return &PolicyCache{dir: dir}
}

// Write atomically writes a SecurityPolicy to disk with mode 0600.
// Uses temp file + rename to prevent partial reads.
// Rejects writes to symlink targets (symlink defense).
func (c *PolicyCache) Write(taskID string, policy SecurityPolicy) error {
	return c.WriteWithLstatGuard(taskID, policy)
}

// WriteWithLstatGuard writes a SecurityPolicy after verifying the target
// path is not a symlink. Uses Lstat to detect symlinks before writing.
func (c *PolicyCache) WriteWithLstatGuard(taskID string, policy SecurityPolicy) error {
	target := c.PolicyPath(taskID)

	// Check directory for symlinks
	if err := checkSymlink(c.dir); err != nil {
		return err
	}

	// Check target path for symlinks (only if file already exists)
	if err := checkSymlink(target); err != nil {
		return err
	}

	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}

	if err := os.MkdirAll(c.dir, 0700); err != nil {
		return fmt.Errorf("create policy dir: %w", err)
	}

	tmp, err := os.CreateTemp(c.dir, "policy-*.json")
	if err != nil {
		return fmt.Errorf("create temp policy file: %w", err)
	}
	tmpPath := tmp.Name()

	// Explicitly set restrictive permissions on the temp policy file.
	if err := os.Chmod(tmpPath, 0600); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("chmod policy file: %w", err)
	}

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("write policy: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close policy file: %w", err)
	}

	if err := os.Rename(tmpPath, target); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename policy file: %w", err)
	}
	return nil
}

// checkSymlink uses Lstat to detect if a path is a symlink.
// Returns error only if the path is confirmed to be a symlink.
// Non-existent paths or other errors are treated as safe (non-symlink).
func checkSymlink(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		// Path doesn't exist or can't be stat'd — not a symlink concern
		return nil
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("symlink detected at policy path: refusing to write")
	}
	return nil
}

// Read loads a SecurityPolicy from disk for the given task ID.
// Returns an error if the file does not exist or cannot be parsed.
func (c *PolicyCache) Read(taskID string) (*SecurityPolicy, error) {
	data, err := os.ReadFile(c.PolicyPath(taskID))
	if err != nil {
		return nil, fmt.Errorf("read policy for %s: %w", taskID, err)
	}

	var policy SecurityPolicy
	if err := json.Unmarshal(data, &policy); err != nil {
		return nil, fmt.Errorf("unmarshal policy for %s: %w", taskID, err)
	}
	return &policy, nil
}

// Delete removes the cached policy file for the given task ID.
// Ignores not-exist errors.
func (c *PolicyCache) Delete(taskID string) {
	_ = os.Remove(c.PolicyPath(taskID))
}

// PolicyPath returns the file path for the given task ID's policy.
// Uses filepath.Base to prevent path traversal via malicious task IDs.
func (c *PolicyCache) PolicyPath(taskID string) string {
	safe := filepath.Base(taskID)
	return filepath.Join(c.dir, fmt.Sprintf("autopus-policy-%s.json", safe))
}
