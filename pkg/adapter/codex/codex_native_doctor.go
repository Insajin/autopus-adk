package codex

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
)

func (a *Adapter) validateCodexNativeDocuments(errs *[]adapter.ValidationError) {
	validateCodexAgentDocuments(a.root, errs)
	validateCodexHooksDocument(a.root, errs)
	validateCodexPluginDocuments(a.root, errs)
}

func validateCodexAgentDocuments(root string, errs *[]adapter.ValidationError) {
	entries, err := os.ReadDir(filepath.Join(root, ".codex", "agents"))
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		path := filepath.Join(".codex", "agents", entry.Name())
		data, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			appendInvalidCodexDocument(errs, path, "Codex agent TOML을 읽을 수 없음")
			continue
		}
		body := string(data)
		valid := strings.Contains(body, codexV2ContractHeading) &&
			strings.Contains(body, "shared cwd") && strings.Contains(body, "disjoint write ownership")
		for _, tool := range codexV2ToolNames() {
			valid = valid && strings.Contains(body, tool)
		}
		if !valid {
			appendInvalidCodexDocument(errs, path, "Codex agent에 V2 collaboration 계약이 없음")
		}
	}
}

func validateCodexHooksDocument(root string, errs *[]adapter.ValidationError) {
	path := filepath.Join(".codex", "hooks.json")
	data, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		return
	}
	var doc hooksDoc
	if json.Unmarshal(data, &doc) != nil {
		appendInvalidCodexDocument(errs, path, "Codex hooks JSON이 malformed 상태임")
		return
	}
	for _, groups := range doc.Hooks {
		for _, group := range groups {
			for _, handler := range group.Hooks {
				if isAutopusHookHandler(handler) {
					return
				}
			}
		}
	}
	appendInvalidCodexDocument(errs, path, "Autopus Codex native hook entry가 없음")
}

func validateCodexPluginDocuments(root string, errs *[]adapter.ValidationError) {
	manifestPath := filepath.Join(".autopus", "plugins", "auto", ".codex-plugin", "plugin.json")
	data, err := os.ReadFile(filepath.Join(root, manifestPath))
	if err == nil {
		var doc pluginManifest
		if json.Unmarshal(data, &doc) != nil || doc.Name != "auto" || doc.Skills != "./skills" {
			appendInvalidCodexDocument(errs, manifestPath, "Codex plugin manifest가 올바르지 않음")
		}
	}
	marketPath := filepath.Join(".agents", "plugins", "marketplace.json")
	market, err := os.ReadFile(filepath.Join(root, marketPath))
	if err != nil {
		return
	}
	var doc map[string]json.RawMessage
	var plugins []json.RawMessage
	if json.Unmarshal(market, &doc) != nil || json.Unmarshal(doc["plugins"], &plugins) != nil {
		appendInvalidCodexDocument(errs, marketPath, "Codex marketplace JSON이 malformed 상태임")
		return
	}
	for _, plugin := range plugins {
		if isAutopusMarketplaceEntry(plugin) {
			return
		}
	}
	appendInvalidCodexDocument(errs, marketPath, "Autopus Codex marketplace entry가 없음")
}

func codexV2ToolNames() []string {
	return []string{"spawn_agent", "send_message", "followup_task", "wait_agent", "interrupt_agent", "list_agents"}
}

func appendInvalidCodexDocument(errs *[]adapter.ValidationError, path, message string) {
	*errs = append(*errs, adapter.ValidationError{File: path, Message: message, Level: "error"})
}
