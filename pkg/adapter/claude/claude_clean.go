package claude

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

var claudeSharedManagedPaths = map[string]bool{
	"CLAUDE.md":                true,
	".claude/settings.json":    true,
	".mcp.json":                true,
	claudePermissionLedgerPath: true,
}

// @AX:WARN [AUTO]: managed Claude cleanup contains more than eight conditional branches.
// @AX:REASON [AUTO]: manifest ownership, user-modified checksums, shared-document retraction, transactional writes/removals, and empty-directory pruning converge here.
func (a *Adapter) cleanManagedSurfaces() error {
	if err := a.validateEmptyPruneRoots(); err != nil {
		return err
	}
	manifest, err := loadClaudeManifestSafely(a.root)
	if err != nil {
		return err
	}
	ledger, ledgerOwned, err := loadClaudePermissionLedger(a.root, manifest)
	if err != nil {
		return err
	}
	plan := adapter.TransactionPlan{}
	settingsWrite, err := a.prepareCleanSettings(ledger)
	if err != nil {
		return err
	}
	mcpWrite, err := a.prepareCleanMCP()
	if err != nil {
		return err
	}
	for _, write := range []*adapter.TransactionWrite{settingsWrite, mcpWrite} {
		if write != nil {
			plan.Writes = append(plan.Writes, *write)
		}
	}
	claudeWrite, err := a.prepareCleanClaudeMD()
	if err != nil {
		return err
	}
	if claudeWrite != nil {
		if strings.TrimSpace(string(claudeWrite.Content)) == "" {
			plan.Removes = append(plan.Removes, adapter.TransactionRemove{Path: "CLAUDE.md"})
		} else {
			plan.Writes = append(plan.Writes, *claudeWrite)
		}
	}
	if ledgerOwned {
		plan.Removes = append(plan.Removes, adapter.TransactionRemove{Path: claudePermissionLedgerPath})
	}

	if manifest != nil {
		allowedCleanPaths, allowErr := claudeCleanAllowedPaths()
		if allowErr != nil {
			return allowErr
		}
		for path, owned := range manifest.Files {
			clean := filepath.ToSlash(filepath.Clean(path))
			if claudeSharedManagedPaths[clean] {
				continue
			}
			if !allowedCleanPaths[clean] {
				return fmt.Errorf("manifest path is outside Claude generated allowlist: %s", clean)
			}
			data, exists, readErr := readClaudeCleanFile(a.root, clean)
			if readErr != nil {
				return fmt.Errorf("managed file 읽기 실패 %s: %w", clean, readErr)
			}
			if !exists || adapter.Checksum(string(data)) != owned.Checksum {
				continue
			}
			plan.Removes = append(plan.Removes, adapter.TransactionRemove{Path: clean})
		}
		plan.Removes = append(plan.Removes, adapter.TransactionRemove{Path: claudeManifestPath})
	}
	if len(plan.Writes) == 0 && len(plan.Removes) == 0 {
		return nil
	}
	if _, err = adapter.ApplyTransaction(a.root, adapterName, plan); err != nil {
		return err
	}
	return a.pruneEmptyManagedDirs()
}

// @AX:WARN [AUTO]: Claude settings cleanup contains eight conditional branches.
// @AX:REASON [AUTO]: strict JSON admission, managed hook and permission retraction, user status-line restoration, and no-op preservation converge here.
func (a *Adapter) prepareCleanSettings(ledger *claudePermissionLedger) (*adapter.TransactionWrite, error) {
	data, exists, err := readClaudeCleanFile(a.root, ".claude/settings.json")
	if err != nil {
		return nil, fmt.Errorf("settings.json 읽기 실패: %w", err)
	}
	if !exists {
		return nil, nil
	}
	var settings map[string]any
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("settings.json 파싱 실패: %w", err)
	}
	changed := false
	if hooks, ok := settings["hooks"].(map[string]any); ok {
		before := managedClaudeHookCount(hooks)
		retractManagedHookEntries(hooks)
		changed = before > 0
		if len(hooks) == 0 {
			delete(settings, "hooks")
		}
	}
	if removeManagedPermissions(settings, ledger) {
		changed = true
	}
	statusChanged, err := restoreUserStatusLine(settings, a.root)
	if err != nil {
		return nil, err
	}
	if statusChanged {
		changed = true
	}
	if !changed {
		return nil, nil
	}
	return claudeJSONWrite(".claude/settings.json", settings)
}

func managedClaudeHookCount(hooks map[string]any) int {
	count := 0
	for _, raw := range hooks {
		entries, _ := raw.([]any)
		for _, entry := range entries {
			if isManagedClaudeHookEntry(entry) {
				count++
			}
		}
	}
	return count
}

func restoreUserStatusLine(settings map[string]any, root string) (bool, error) {
	status, ok := settings["statusLine"].(map[string]any)
	if !ok {
		return false, nil
	}
	command, _ := status["command"].(string)
	switch strings.TrimSpace(command) {
	case autopusClaudeCombinedStatusLineCommand:
		data, exists, err := readClaudeCleanFile(root, ".claude/statusline-user-command.txt")
		if err != nil {
			return false, fmt.Errorf("statusline user command 읽기 실패: %w", err)
		}
		userCommand := ""
		if exists {
			userCommand = strings.TrimSpace(string(data))
		}
		if userCommand == "" {
			delete(settings, "statusLine")
		} else {
			status["command"] = userCommand
		}
		return true, nil
	case autopusClaudeStatusLineCommand:
		delete(settings, "statusLine")
		return true, nil
	default:
		return false, nil
	}
}

func (a *Adapter) prepareCleanMCP() (*adapter.TransactionWrite, error) {
	data, exists, err := readClaudeCleanFile(a.root, ".mcp.json")
	if err != nil {
		return nil, fmt.Errorf("MCP JSON 읽기 실패: %w", err)
	}
	if !exists {
		return nil, nil
	}
	var value map[string]any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, fmt.Errorf("MCP JSON 파싱 실패: %w", err)
	}
	servers, ok := value["mcpServers"].(map[string]any)
	if !ok {
		return nil, nil
	}
	changed := false
	for name, server := range servers {
		if isManagedClaudeMCPServer(name, server) {
			delete(servers, name)
			changed = true
		}
	}
	if !changed {
		return nil, nil
	}
	if len(servers) == 0 {
		delete(value, "mcpServers")
	}
	return claudeJSONWrite(".mcp.json", value)
}

func isManagedClaudeMCPServer(name string, raw any) bool {
	server, ok := raw.(map[string]any)
	if !ok {
		return false
	}
	args, _ := server["args"].([]any)
	for _, arg := range args {
		value, _ := arg.(string)
		switch name {
		case "context7":
			if strings.Contains(value, "@upstash/context7-mcp") {
				return true
			}
		case "sequential-thinking":
			if strings.Contains(value, "@modelcontextprotocol/server-sequential-thinking") {
				return true
			}
		}
	}
	return false
}

func (a *Adapter) prepareCleanClaudeMD() (*adapter.TransactionWrite, error) {
	data, exists, err := readClaudeCleanFile(a.root, "CLAUDE.md")
	if err != nil {
		return nil, fmt.Errorf("CLAUDE.md 읽기 실패: %w", err)
	}
	if !exists {
		return nil, nil
	}
	if !strings.Contains(string(data), markerBegin) {
		return nil, nil
	}
	return &adapter.TransactionWrite{Path: "CLAUDE.md", Content: []byte(removeMarkerSection(string(data))), Perm: 0o644}, nil
}

func claudeJSONWrite(path string, value map[string]any) (*adapter.TransactionWrite, error) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return nil, err
	}
	return &adapter.TransactionWrite{Path: path, Content: append(data, '\n'), Perm: 0o644}, nil
}
