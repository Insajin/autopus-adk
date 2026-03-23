// Package detect는 코딩 CLI 바이너리와 의존성의 설치 여부를 감지한다.
package detect

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Platform은 감지된 코딩 CLI 정보이다.
type Platform struct {
	Name    string // claude-code, codex, gemini-cli 등
	Binary  string // 실행 파일명
	Version string // 감지된 버전
}

// knownCLIs는 알려진 코딩 CLI 목록이다.
var knownCLIs = []struct {
	name       string
	binary     string
	versionArg string
}{
	{"claude-code", "claude", "--version"},
	{"codex", "codex", "--version"},
	{"gemini-cli", "gemini", "--version"},
	{"opencode", "opencode", "--version"},
	{"cursor", "cursor", "--version"},
}

// DetectPlatforms는 PATH에서 코딩 CLI를 감지한다.
func DetectPlatforms() []Platform {
	var platforms []Platform
	for _, cli := range knownCLIs {
		if v, ok := detectBinary(cli.binary, cli.versionArg); ok {
			platforms = append(platforms, Platform{
				Name:    cli.name,
				Binary:  cli.binary,
				Version: v,
			})
		}
	}
	return platforms
}

// @AX:ANCHOR [AUTO]: Do not rename or change the signature of IsInstalled
// @AX:REASON: Called by 6+ consumers — doctor, doctor_fix, spec_review, verify, orchestra, detect internals
// IsInstalled는 특정 바이너리의 설치 여부를 확인한다.
func IsInstalled(binary string) bool {
	_, err := exec.LookPath(binary)
	return err == nil
}

func detectBinary(binary, versionArg string) (string, bool) {
	path, err := exec.LookPath(binary)
	if err != nil {
		return "", false
	}
	_ = path
	out, err := exec.Command(binary, versionArg).Output()
	if err != nil {
		return "unknown", true
	}
	return strings.TrimSpace(string(out)), true
}

// OrchestraProvider represents an orchestra provider and its install status.
type OrchestraProvider struct {
	Name      string // claude, codex, gemini
	Binary    string // binary name to look up
	Installed bool   // whether the binary is in PATH
}

// @AX:NOTE [AUTO]: Fixed set of 3 orchestra providers — expand here when adding a new provider binary
// knownOrchestraProviders lists the known orchestra provider binaries.
var knownOrchestraProviders = []struct {
	name   string
	binary string
}{
	{"claude", "claude"},
	{"codex", "codex"},
	{"gemini", "gemini"},
}

// DetectOrchestraProviders checks which orchestra provider binaries are installed.
func DetectOrchestraProviders() []OrchestraProvider {
	var providers []OrchestraProvider
	for _, p := range knownOrchestraProviders {
		providers = append(providers, OrchestraProvider{
			Name:      p.name,
			Binary:    p.binary,
			Installed: IsInstalled(p.binary),
		})
	}
	return providers
}

// InstalledOrchestraProviders returns only the installed orchestra providers.
func InstalledOrchestraProviders() []string {
	var names []string
	for _, p := range DetectOrchestraProviders() {
		if p.Installed {
			names = append(names, p.Name)
		}
	}
	return names
}

// Dependency는 외부 도구 의존성이다.
type Dependency struct {
	Name           string
	Binary         string
	InstallCmd     string
	Required       bool   // true이면 필수, false이면 권장
	Description    string
	DependsOn      string // dependency name that must be installed first
	PostInstallCmd string // command to run after install (e.g., browser download)
}

// IsNpmBased reports whether this dependency is installed via npm.
// @AX:NOTE [AUTO] public method with single call site; add test coverage for non-npm prefix cases
func (d Dependency) IsNpmBased() bool {
	return strings.HasPrefix(d.InstallCmd, "npm ")
}

// FullModeDeps는 Full 모드의 의존성 목록이다.
var FullModeDeps = []Dependency{
	{Name: "ast-grep", Binary: "sg", InstallCmd: "npm i -g @ast-grep/cli", Required: true, Description: "Structural code search"},
	{Name: "node", Binary: "node", InstallCmd: "https://nodejs.org", Required: false, Description: "Node.js runtime (required for Playwright)"},
	{Name: "playwright", Binary: "playwright", InstallCmd: "npm i -g playwright", Required: false, Description: "E2E testing + screenshots", DependsOn: "node", PostInstallCmd: "npx playwright install chromium"},
	{Name: "agent-browser", Binary: "agent-browser", InstallCmd: "npm i -g agent-browser", Required: true, Description: "Web browsing"},
	{Name: "gh", Binary: "gh", InstallCmd: "", Required: false, Description: "GitHub CLI"},
}

// CheckDependencies는 의존성 상태를 확인한다.
func CheckDependencies(deps []Dependency) []DependencyStatus {
	var statuses []DependencyStatus
	for _, d := range deps {
		statuses = append(statuses, DependencyStatus{
			Dependency: d,
			Installed:  IsInstalled(d.Binary),
		})
	}
	return statuses
}

// DependencyStatus는 의존성 상태이다.
type DependencyStatus struct {
	Dependency
	Installed bool
}

// ParentRuleConflict는 부모 디렉터리에서 발견된 규칙 충돌 정보이다.
type ParentRuleConflict struct {
	ParentDir string // 충돌이 발견된 부모 디렉터리
	RulesDir  string // 부모의 .claude/rules/ 경로
	Namespace string // 규칙 네임스페이스 (예: "moai")
}

// CheckParentRuleConflicts는 부모 디렉터리에 다른 하네스의 .claude/rules/가 있는지 탐지한다.
// Claude Code는 상위 디렉터리를 탐색하며 규칙을 로드하므로,
// 부모에 다른 하네스 규칙이 있으면 현재 프로젝트에 의도치 않게 적용된다.
func CheckParentRuleConflicts(projectDir string) []ParentRuleConflict {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil
	}

	var conflicts []ParentRuleConflict
	current := filepath.Dir(absDir) // 부모부터 시작

	for current != "/" && current != "." {
		rulesDir := filepath.Join(current, ".claude", "rules")
		entries, err := os.ReadDir(rulesDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() && entry.Name() != "autopus" {
					conflicts = append(conflicts, ParentRuleConflict{
						ParentDir: current,
						RulesDir:  rulesDir,
						Namespace: entry.Name(),
					})
				}
			}
		}
		current = filepath.Dir(current)
	}

	return conflicts
}
