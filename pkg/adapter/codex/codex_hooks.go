package codex

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/content"
)

// generateHooks renders hooks.json template and merges with existing user hooks.
// Autopus-managed hooks are identified by the "__autopus__" marker key.
// User hooks (without the marker) are preserved during merge.
func (a *Adapter) generateHooks(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	rendered, err := a.renderHooksTemplate(cfg)
	if err != nil {
		return nil, err
	}

	targetPath := filepath.Join(a.root, ".codex", "hooks.json")
	merged, err := mergeHooks(targetPath, rendered)
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
		return nil, fmt.Errorf(".codex 디렉터리 생성 실패: %w", err)
	}
	if err := os.WriteFile(targetPath, merged, 0644); err != nil {
		return nil, fmt.Errorf("codex hooks.json 쓰기 실패: %w", err)
	}

	return []adapter.FileMapping{{
		TargetPath:      filepath.Join(".codex", "hooks.json"),
		OverwritePolicy: adapter.OverwriteMerge,
		Checksum:        checksum(string(merged)),
		Content:         merged,
	}}, nil
}

// prepareHooksFile returns hooks.json file mapping without writing to disk.
// Uses merge policy to preserve user hooks on application.
func (a *Adapter) prepareHooksFile(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	rendered, err := a.renderHooksTemplate(cfg)
	if err != nil {
		return nil, err
	}

	targetPath := filepath.Join(a.root, ".codex", "hooks.json")
	merged, err := mergeHooks(targetPath, rendered)
	if err != nil {
		return nil, err
	}

	return []adapter.FileMapping{{
		TargetPath:      filepath.Join(".codex", "hooks.json"),
		OverwritePolicy: adapter.OverwriteMerge,
		Checksum:        checksum(string(merged)),
		Content:         merged,
	}}, nil
}

func (a *Adapter) prepareGitHookFiles(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	_, gitHooks, err := content.GenerateHookConfigs(cfg.Hooks, adapterName, false)
	if err != nil {
		return nil, fmt.Errorf("git hooks 생성 실패: %w", err)
	}

	files := make([]adapter.FileMapping, 0, len(gitHooks))
	for _, gh := range gitHooks {
		files = append(files, adapter.FileMapping{
			TargetPath:      gh.Path,
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        checksum(gh.Content),
			Content:         []byte(gh.Content),
		})
	}

	return files, nil
}

// renderHooksTemplate renders the codex hooks.json template.
func (a *Adapter) renderHooksTemplate(cfg *config.HarnessConfig) (string, error) {
	hooks, _, err := content.GenerateHookConfigs(cfg.Hooks, adapterName, true)
	if err != nil {
		return "", fmt.Errorf("codex hooks 생성 실패: %w", err)
	}

	doc := hooksDoc{Hooks: make(map[string][]hookEntry)}
	for _, hook := range hooks {
		doc.Hooks[hook.Event] = append(doc.Hooks[hook.Event], hookEntry{
			Type:    hook.Type,
			Command: hook.Command,
			Matcher: hook.Matcher,
			Timeout: hook.Timeout,
		})
	}

	rendered, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("codex hooks JSON 직렬화 실패: %w", err)
	}
	return string(rendered), nil
}

// mergeHooks reads existing hooks.json from disk, preserves user hooks (no __autopus__ marker),
// and upserts Autopus-managed hooks from the rendered template.
func mergeHooks(existingPath, rendered string) ([]byte, error) {
	// Parse rendered autopus hooks and stamp them with marker
	var autopusDoc hooksDoc
	if err := json.Unmarshal([]byte(rendered), &autopusDoc); err != nil {
		return nil, fmt.Errorf("rendered hooks JSON 파싱 실패: %w", err)
	}
	stampAutopusMarker(&autopusDoc)

	// Read existing file — if missing or invalid, use autopus-only result
	existingData, err := os.ReadFile(existingPath)
	if err != nil {
		return json.MarshalIndent(autopusDoc, "", "  ")
	}

	var existingDoc hooksDoc
	if err := json.Unmarshal(existingData, &existingDoc); err != nil {
		return json.MarshalIndent(autopusDoc, "", "  ")
	}

	// Merge each hook category: keep user hooks, upsert autopus hooks
	merged := mergeHookCategories(existingDoc, autopusDoc)
	return json.MarshalIndent(merged, "", "  ")
}

// hooksDoc represents the top-level hooks.json structure.
type hooksDoc struct {
	Hooks map[string][]hookEntry `json:"hooks"`
}

// hookEntry represents a single hook entry in hooks.json.
type hookEntry struct {
	Type    string `json:"type,omitempty"`
	Command string `json:"command"`
	Matcher string `json:"matcher,omitempty"`
	Timeout int    `json:"timeout,omitempty"`
	Autopus bool   `json:"__autopus__,omitempty"`
}

// stampAutopusMarker marks all hooks in the document as Autopus-managed.
func stampAutopusMarker(doc *hooksDoc) {
	for cat, entries := range doc.Hooks {
		for i := range entries {
			entries[i].Autopus = true
		}
		doc.Hooks[cat] = entries
	}
}

// mergeHookCategories merges existing and autopus hook documents.
// User hooks (Autopus==false) are preserved; autopus hooks are replaced.
func mergeHookCategories(existing, autopus hooksDoc) hooksDoc {
	result := hooksDoc{Hooks: make(map[string][]hookEntry)}

	// Collect all category names
	cats := make(map[string]bool)
	for c := range existing.Hooks {
		cats[c] = true
	}
	for c := range autopus.Hooks {
		cats[c] = true
	}

	for cat := range cats {
		// Keep user hooks from existing
		var merged []hookEntry
		for _, e := range existing.Hooks[cat] {
			if !e.Autopus {
				merged = append(merged, e)
			}
		}
		// Append all autopus hooks for this category
		merged = append(merged, autopus.Hooks[cat]...)
		result.Hooks[cat] = merged
	}

	return result
}

// installGitHooks generates and writes git hooks as fallback.
func (a *Adapter) installGitHooks(cfg *config.HarnessConfig) error {
	files, err := a.prepareGitHookFiles(cfg)
	if err != nil {
		return err
	}

	for _, file := range files {
		ghPath := filepath.Join(a.root, file.TargetPath)
		if err := os.MkdirAll(filepath.Dir(ghPath), 0755); err != nil {
			return fmt.Errorf("git hook 디렉터리 생성 실패: %w", err)
		}
		if err := os.WriteFile(ghPath, file.Content, 0755); err != nil {
			return fmt.Errorf("git hook 쓰기 실패 %s: %w", file.TargetPath, err)
		}
	}
	return nil
}
