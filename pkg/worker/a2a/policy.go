package a2a

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// policyDir returns a secure directory for caching policy files.
// Uses a per-user subdirectory under os.TempDir to avoid symlink attacks (SEC-001).
func policyDir() (string, error) {
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("autopus-%d", os.Getuid()))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("create policy dir: %w", err)
	}
	return dir, nil
}

// cacheSecurityPolicy writes the security policy to a secure temp file for subprocess access.
func cacheSecurityPolicy(taskID string, policy SecurityPolicy, signature string) error {
	data, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal policy: %w", err)
	}

	dir, err := policyDir()
	if err != nil {
		return err
	}

	// Write to a temp file then rename for atomic creation (SEC-001).
	tmp, err := os.CreateTemp(dir, "policy-*.json")
	if err != nil {
		return fmt.Errorf("create temp policy file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		removeIfExists(tmpPath)
		return fmt.Errorf("write policy: %w", err)
	}
	if err := tmp.Close(); err != nil {
		removeIfExists(tmpPath)
		return fmt.Errorf("close policy file: %w", err)
	}

	target := filepath.Join(dir, fmt.Sprintf("autopus-policy-%s.json", taskID))
	if err := os.Rename(tmpPath, target); err != nil {
		removeIfExists(tmpPath)
		return fmt.Errorf("rename policy file: %w", err)
	}
	if err := writePolicySignature(target, signature); err != nil {
		return err
	}
	return nil
}

func removeIfExists(path string) {
	_ = os.Remove(path)
}

// marshalJSON marshals v to JSON, returning an error instead of panicking (SEC-002).
func marshalJSON(v any) (json.RawMessage, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal json: %w", err)
	}
	return data, nil
}
