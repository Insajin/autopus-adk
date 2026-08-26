package claude

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

const (
	claudePermissionLedgerPath    = ".autopus/claude-code-permissions.json"
	claudePermissionLedgerVersion = 1
)

type claudePermissionLedger struct {
	Version int      `json:"version"`
	Allow   []string `json:"allow,omitempty"`
	Deny    []string `json:"deny,omitempty"`
}

func (a *Adapter) preparePermissionLedgerMapping(perms *adapter.PermissionSet) (adapter.FileMapping, error) {
	manifest, err := loadClaudeManifestSafely(a.root)
	if err != nil {
		return adapter.FileMapping{}, err
	}
	ledger, _, err := loadClaudePermissionLedger(a.root, manifest)
	if err != nil {
		return adapter.FileMapping{}, err
	}
	if ledger == nil {
		ledger = &claudePermissionLedger{Version: claudePermissionLedgerVersion}
	}

	allow, deny, err := readClaudePermissionPreimage(a.root)
	if err != nil {
		return adapter.FileMapping{}, err
	}
	if perms != nil {
		ledger.Allow = appendUniqueStrings(ledger.Allow, missingStrings(perms.Allow, allow)...)
		ledger.Deny = appendUniqueStrings(ledger.Deny, missingStrings(perms.Deny, deny)...)
	}
	content, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return adapter.FileMapping{}, fmt.Errorf("permission ledger 직렬화 실패: %w", err)
	}
	content = append(content, '\n')
	return adapter.FileMapping{
		TargetPath:      claudePermissionLedgerPath,
		OverwritePolicy: adapter.OverwriteMerge,
		Checksum:        checksum(string(content)),
		Content:         content,
	}, nil
}

func loadClaudePermissionLedger(
	root string,
	manifest *adapter.Manifest,
) (*claudePermissionLedger, bool, error) {
	if manifest == nil {
		return nil, false, nil
	}
	claim, owned := manifest.Files[claudePermissionLedgerPath]
	if !owned {
		return nil, false, nil
	}
	data, exists, err := readClaudeCleanFile(root, claudePermissionLedgerPath)
	if err != nil {
		return nil, false, fmt.Errorf("permission ledger 읽기 실패: %w", err)
	}
	if !exists {
		return nil, false, fmt.Errorf("permission ledger ownership file is missing")
	}
	target, pathErr := adapter.SafePruneFilePath(root, claudePermissionLedgerPath)
	if pathErr != nil {
		return nil, false, fmt.Errorf("permission ledger path 확인 실패: %w", pathErr)
	}
	info, statErr := os.Lstat(target)
	if statErr != nil {
		return nil, false, fmt.Errorf("permission ledger stat 실패: %w", statErr)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != 0o600 {
		return nil, false, fmt.Errorf("permission ledger must be an owner-only regular file")
	}
	actual := adapter.Checksum(string(data))
	if actual != claim.Checksum {
		return nil, false, fmt.Errorf("permission ledger checksum mismatch")
	}
	if err := verifyClaudePermissionLedgerTransaction(root, actual); err != nil {
		return nil, false, err
	}
	var ledger claudePermissionLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		return nil, false, fmt.Errorf("permission ledger 파싱 실패: %w", err)
	}
	if ledger.Version != claudePermissionLedgerVersion {
		return nil, false, fmt.Errorf("unsupported permission ledger version %d", ledger.Version)
	}
	ledger.Allow = appendUniqueStrings(nil, ledger.Allow...)
	ledger.Deny = appendUniqueStrings(nil, ledger.Deny...)
	return &ledger, true, nil
}

func verifyClaudePermissionLedgerTransaction(root, checksum string) error {
	if err := adapter.RejectSymlinkComponents(root, filepath.Join(".autopus", "txns")); err != nil {
		return fmt.Errorf("permission ledger transaction 확인 실패: %w", err)
	}
	journals, err := adapter.ListCommittedTransactions(root)
	if err != nil {
		return fmt.Errorf("permission ledger transaction 로드 실패: %w", err)
	}
	for _, journal := range journals {
		if journal.Platform != adapterName || journal.Status != adapter.TransactionStatusCommitted {
			continue
		}
		for _, entry := range journal.Entries {
			if filepath.ToSlash(entry.Path) != claudePermissionLedgerPath ||
				entry.Operation != "write" || entry.AfterChecksum != checksum {
				continue
			}
			rel, relErr := filepath.Rel(root, journal.Path)
			if relErr != nil {
				return fmt.Errorf("permission ledger transaction path 확인 실패: %w", relErr)
			}
			if err := adapter.RejectSymlinkComponents(root, rel); err != nil {
				return fmt.Errorf("permission ledger transaction 확인 실패: %w", err)
			}
			return nil
		}
	}
	return fmt.Errorf("permission ledger has no committed transaction ownership")
}

func readClaudePermissionPreimage(root string) ([]string, []string, error) {
	data, exists, err := readClaudeCleanFile(root, ".claude/settings.json")
	if err != nil {
		return nil, nil, fmt.Errorf("settings.json permission preimage 읽기 실패: %w", err)
	}
	if !exists {
		return nil, nil, nil
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, nil, fmt.Errorf("settings.json permission preimage 파싱 실패: %w", err)
	}
	permissions, _ := settings["permissions"].(map[string]any)
	return toStringSlice(permissions["allow"]), toStringSlice(permissions["deny"]), nil
}

func missingStrings(desired, existing []string) []string {
	seen := make(map[string]bool, len(existing))
	for _, value := range existing {
		seen[value] = true
	}
	missing := make([]string, 0, len(desired))
	for _, value := range desired {
		if !seen[value] {
			missing = append(missing, value)
			seen[value] = true
		}
	}
	return missing
}

func appendUniqueStrings(base []string, values ...string) []string {
	seen := make(map[string]bool, len(base)+len(values))
	result := make([]string, 0, len(base)+len(values))
	for _, value := range base {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func removeManagedPermissions(settings map[string]any, ledger *claudePermissionLedger) bool {
	if ledger == nil {
		return false
	}
	permissions, ok := settings["permissions"].(map[string]any)
	if !ok {
		return false
	}
	changed := filterPermissionValues(permissions, "allow", ledger.Allow)
	changed = filterPermissionValues(permissions, "deny", ledger.Deny) || changed
	if len(permissions) == 0 {
		delete(settings, "permissions")
	}
	return changed
}

func filterPermissionValues(permissions map[string]any, key string, managed []string) bool {
	if len(managed) == 0 {
		return false
	}
	values, ok := permissions[key].([]any)
	if !ok {
		return false
	}
	owned := make(map[string]bool, len(managed))
	for _, value := range managed {
		owned[value] = true
	}
	kept := make([]any, 0, len(values))
	changed := false
	for _, value := range values {
		text, isString := value.(string)
		if isString && owned[text] {
			changed = true
			continue
		}
		kept = append(kept, value)
	}
	if len(kept) == 0 {
		delete(permissions, key)
	} else {
		permissions[key] = kept
	}
	return changed
}
