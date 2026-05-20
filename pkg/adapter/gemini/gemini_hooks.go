// Package gemini provides Antigravity CLI hook file support.
package gemini

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

const antigravityHooksTarget = ".agents/hooks.json"

func (a *Adapter) configuredHooks(cfg *config.HarnessConfig) []adapter.HookConfig {
	hooks, _, _ := pkgcontent.GenerateProjectHookConfigs(cfg, adapterName, a.SupportsHooks())
	return hooks
}

func (a *Adapter) prepareAntigravityHooksJSON(hooks []adapter.HookConfig) ([]adapter.FileMapping, error) {
	if len(hooks) == 0 {
		return nil, nil
	}

	existing := make(map[string]any)
	hooksPath := filepath.Join(a.root, antigravityHooksTarget)
	if data, err := os.ReadFile(hooksPath); err == nil {
		if err := json.Unmarshal(data, &existing); err != nil {
			existing = make(map[string]any)
		}
	}

	existing["autopus"] = buildAntigravityHookSpec(hooks)
	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("antigravity hooks.json 직렬화 실패: %w", err)
	}
	out = append(out, '\n')

	return []adapter.FileMapping{{
		TargetPath:      antigravityHooksTarget,
		OverwritePolicy: adapter.OverwriteMerge,
		Checksum:        checksum(string(out)),
		Content:         out,
	}}, nil
}

func buildAntigravityHookSpec(hooks []adapter.HookConfig) map[string]any {
	spec := map[string]any{"enabled": true}
	for _, h := range hooks {
		handler := map[string]any{
			"type":    h.Type,
			"command": h.Command,
		}
		if h.Timeout > 0 {
			handler["timeout"] = h.Timeout
		}

		entry := map[string]any{
			"matcher": h.Matcher,
			"hooks":   []map[string]any{handler},
		}
		entries, _ := spec[h.Event].([]any)
		spec[h.Event] = append(entries, entry)
	}
	return spec
}

func (a *Adapter) writeAntigravityHooksJSON(hooks []adapter.HookConfig) ([]adapter.FileMapping, error) {
	files, err := a.prepareAntigravityHooksJSON(hooks)
	if err != nil || len(files) == 0 {
		return files, err
	}
	for _, f := range files {
		destPath := filepath.Join(a.root, f.TargetPath)
		if err := os.MkdirAll(filepath.Dir(destPath), 0755); err != nil {
			return nil, fmt.Errorf("antigravity hooks 디렉터리 생성 실패: %w", err)
		}
		if err := adapter.WriteFileIfChanged(destPath, f.Content, 0644); err != nil {
			return nil, fmt.Errorf("antigravity hooks 파일 쓰기 실패: %w", err)
		}
	}
	return files, nil
}

func (a *Adapter) removeAntigravityHooksJSON() error {
	hooksPath := filepath.Join(a.root, antigravityHooksTarget)
	data, err := os.ReadFile(hooksPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("antigravity hooks 파일 읽기 실패: %w", err)
	}

	var existing map[string]any
	if err := json.Unmarshal(data, &existing); err != nil {
		return nil
	}
	delete(existing, "autopus")
	if len(existing) == 0 {
		if err := os.Remove(hooksPath); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("antigravity hooks 파일 제거 실패: %w", err)
		}
		return nil
	}

	out, err := json.MarshalIndent(existing, "", "  ")
	if err != nil {
		return fmt.Errorf("antigravity hooks 파일 직렬화 실패: %w", err)
	}
	return adapter.WriteFileIfChanged(hooksPath, append(out, '\n'), 0644)
}
