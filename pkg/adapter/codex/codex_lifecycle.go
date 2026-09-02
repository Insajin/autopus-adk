package codex

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

const minProjectDocMaxBytes = 262144

// Validate checks the validity of installed files.
func (a *Adapter) Validate(_ context.Context) ([]adapter.ValidationError, error) {
	var errs []adapter.ValidationError

	if !a.openCodeOwnsRootDoc() && a.managesFile("AGENTS.md") {
		agentsPath := filepath.Join(a.root, "AGENTS.md")
		data, err := os.ReadFile(agentsPath)
		if err != nil {
			errs = append(errs, adapter.ValidationError{
				File:    "AGENTS.md",
				Message: "AGENTS.md를 읽을 수 없음",
				Level:   "error",
			})
			return errs, nil
		}

		content := string(data)
		if !strings.Contains(content, markerBegin) || !strings.Contains(content, markerEnd) {
			errs = append(errs, adapter.ValidationError{
				File:    "AGENTS.md",
				Message: "AUTOPUS 마커 섹션이 없음",
				Level:   "warning",
			})
		}
	}

	skillsDir := filepath.Join(a.root, ".codex", "skills")
	if _, err := os.Stat(skillsDir); os.IsNotExist(err) {
		errs = append(errs, adapter.ValidationError{
			File:    ".codex/skills",
			Message: ".codex/skills 디렉터리가 없음",
			Level:   "error",
		})
	}

	marketplacePath := filepath.Join(a.root, ".agents", "plugins", "marketplace.json")
	if _, err := os.Stat(marketplacePath); os.IsNotExist(err) {
		errs = append(errs, adapter.ValidationError{
			File:    ".agents/plugins/marketplace.json",
			Message: "로컬 Codex plugin marketplace가 없음",
			Level:   "warning",
		})
	}

	a.validateRouterPrompt(&errs)
	a.validateConfig(&errs)
	a.validateNativeCodexSurface(&errs)
	a.validateCodexNativeDocuments(&errs)
	return errs, nil
}

// Clean removes files created by this adapter.
// @AX:WARN [AUTO]: manifest-aware Codex cleanup contains more than eight conditional branches.
// @AX:REASON [AUTO]: owned-file removal, shared-document preservation, structured hook/config cleanup, manifest deletion, and empty-directory pruning converge here.
func (a *Adapter) Clean(_ context.Context) error {
	manifestRel := filepath.Join(".autopus", adapterName+"-manifest.json")
	if _, err := safeCodexCleanTarget(a.root, manifestRel); err != nil {
		return fmt.Errorf("Codex manifest path 검증 실패: %w", err)
	}
	if err := a.validateEmptyPruneRoots(); err != nil {
		return fmt.Errorf("Codex prune root 검증 실패: %w", err)
	}
	manifest, err := adapter.LoadManifest(a.root, adapterName)
	if err != nil {
		return fmt.Errorf("Codex manifest 로드 실패: %w", err)
	}
	if manifest == nil {
		return a.cleanRootDocMarkerSafely()
	}

	special := map[string]bool{
		filepath.ToSlash("AGENTS.md"):                                             true,
		filepath.ToSlash(filepath.Join(".codex", "hooks.json")):                   true,
		filepath.ToSlash(filepath.Join(".agents", "plugins", "marketplace.json")): true,
		filepath.ToSlash(codexConfigRelPath):                                      true,
	}
	allowedCleanPaths, err := codexCleanAllowedPaths()
	if err != nil {
		return err
	}
	openCodeOwnsSharedSkills := a.openCodeOwnsRootDoc()
	for path, owned := range manifest.Files {
		clean := filepath.ToSlash(path)
		if special[clean] {
			continue
		}
		if openCodeOwnsSharedSkills && strings.HasPrefix(clean, ".agents/skills/") {
			continue
		}
		if !allowedCleanPaths[clean] {
			return fmt.Errorf("Codex manifest-owned path가 managed allowlist 밖임: %s", path)
		}
		if err := removeUnmodifiedCodexManagedFile(a.root, clean, owned); err != nil {
			return fmt.Errorf("Codex manifest-owned file 제거 실패 %s: %w", path, err)
		}
	}

	if manifestOwnsCodexPath(manifest, filepath.Join(".codex", "hooks.json")) {
		if err := cleanCodexManagedMergeFile(a.root, filepath.Join(".codex", "hooks.json"), cleanCodexHooksFile); err != nil {
			return err
		}
	}
	if manifestOwnsCodexPath(manifest, filepath.Join(".agents", "plugins", "marketplace.json")) {
		if err := cleanCodexManagedMergeFile(a.root, filepath.Join(".agents", "plugins", "marketplace.json"), cleanCodexMarketplaceFile); err != nil {
			return err
		}
	}
	if manifestOwnsCodexPath(manifest, codexConfigRelPath) {
		if err := cleanCodexManagedMergeFile(a.root, codexConfigRelPath, cleanCodexConfigFile); err != nil {
			return err
		}
	}
	if manifestOwnsCodexPath(manifest, "AGENTS.md") {
		if err := a.cleanRootDocMarkerSafely(); err != nil {
			return err
		}
	}
	if err := removeCodexManifestSafely(a.root); err != nil {
		return fmt.Errorf("Codex manifest 제거 실패: %w", err)
	}
	return a.pruneEmptyManagedDirs()
}

func manifestOwnsCodexPath(manifest *adapter.Manifest, path string) bool {
	if manifest == nil {
		return false
	}
	_, ok := manifest.Files[path]
	if ok {
		return true
	}
	_, ok = manifest.Files[filepath.ToSlash(path)]
	return ok
}

func (a *Adapter) openCodeOwnsRootDoc() bool {
	manifest, err := adapter.LoadManifest(a.root, "opencode")
	if err == nil && manifest != nil {
		return true
	}
	_, statErr := os.Stat(filepath.Join(a.root, ".autopus", "opencode-manifest.json"))
	return statErr == nil
}

// openCodeOwnsSharedSkills reports whether `.agents/skills/<workflow>` belongs
// to the OpenCode adapter (opencode_skills.go emits it) rather than being a
// retired Codex leftover.
//
// openCodeOwnsRootDoc alone is not enough on the delete path. It reads
// `.autopus/opencode-manifest.json`, which is gitignored, so a freshly cloned
// workspace with OpenCode installed answers "no" and the obsolete-surface
// prune would delete a live OpenCode surface. The configured platform list is
// the manifest-independent half, so ownership is claimed when either source
// claims it: a false negative here destroys another platform's files, while a
// false positive only leaves a retired directory for the doctor to report.
func (a *Adapter) openCodeOwnsSharedSkills(cfg *config.HarnessConfig) bool {
	return a.openCodeOwnsRootDoc() || !codexOwnsRootDoc(cfg)
}

func (a *Adapter) cleanRootDocMarker() error {
	if a.openCodeOwnsRootDoc() {
		return nil
	}
	agentsPath := filepath.Join(a.root, "AGENTS.md")
	data, err := os.ReadFile(agentsPath)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("AGENTS.md 읽기 실패: %w", err)
	}
	return os.WriteFile(agentsPath, []byte(removeMarkerSection(string(data))), 0o644)
}

// InstallHooks is a no-op — hooks are managed via .codex/hooks.json template.
func (a *Adapter) InstallHooks(_ context.Context, _ []adapter.HookConfig, _ *adapter.PermissionSet) error {
	return nil
}

func (a *Adapter) managesFile(targetPath string) bool {
	manifest, err := adapter.LoadManifest(a.root, adapterName)
	if err == nil && manifest != nil {
		_, ok := manifest.Files[targetPath]
		return ok
	}

	if isSharedSurfacePath(targetPath) {
		opencodeManifest, loadErr := adapter.LoadManifest(a.root, "opencode")
		if loadErr == nil && opencodeManifest != nil {
			return false
		}
	}

	return true
}

func isSharedSurfacePath(targetPath string) bool {
	return strings.HasPrefix(targetPath, filepath.Join(".agents", "skills")+string(os.PathSeparator))
}

func (a *Adapter) validateRouterPrompt(errs *[]adapter.ValidationError) {
	routerSkillRel := codexProjectSkillPath("auto")
	data, err := os.ReadFile(filepath.Join(a.root, routerSkillRel))
	if err != nil {
		*errs = append(*errs, adapter.ValidationError{
			File: routerSkillRel, Message: "Codex native router skill을 읽을 수 없음", Level: "error",
		})
		return
	}
	content := string(data)
	required := []string{
		"name: codex-auto", "@auto", "$codex-auto", "## Router Execution Contract",
		"## Codex Multi-Agent V2 Contract", "## SPEC Path Resolution",
	}
	for _, fragment := range required {
		if strings.Contains(content, fragment) {
			continue
		}
		*errs = append(*errs, adapter.ValidationError{
			File: routerSkillRel, Message: "Codex native router skill 계약이 불완전함", Level: "error",
		})
		return
	}
}
