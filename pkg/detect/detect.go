// Package detect는 코딩 CLI 바이너리와 의존성의 설치 여부를 감지한다.
package detect

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/processprobe"
)

// Platform is a detected coding CLI.
type Platform struct {
	Name    string // claude-code, codex, antigravity-cli, etc.
	Binary  string // executable binary name
	Version string // detected version
}

// knownCLIs lists supported coding CLIs.
var knownCLIs = []struct {
	name       string
	binary     string
	versionArg string
}{
	{"claude-code", "claude", "--version"},
	{"codex", "codex", "--version"},
	{"antigravity-cli", "agy", "--version"},
	{"opencode", "opencode", "--version"},
	{"cursor", "cursor", "--version"},
}

const (
	cliVersionTimeout   = 5 * time.Second
	cliVersionWaitDelay = 250 * time.Millisecond
)

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
	return detectBinaryWithLimits(binary, versionArg, cliVersionTimeout, cliVersionWaitDelay)
}

func detectBinaryWithLimits(binary, versionArg string, timeout, waitDelay time.Duration) (string, bool) {
	path, err := exec.LookPath(binary)
	if err != nil {
		return "", false
	}
	// Timeout prevents hang when a CLI doesn't respond to --version
	// (e.g., opens GUI or waits for input on Windows).
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, versionArg)
	cmd.WaitDelay = waitDelay
	out, err := processprobe.Output(cmd)
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
	{"gemini", "agy"},
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

// Dependency describes an external tool dependency.
type Dependency struct {
	Name            string
	Binary          string
	InstallCmd      string
	InstallViaShell bool // run InstallCmd through the OS shell for pipes/redirects
	Required        bool // true means required, false means recommended
	Description     string
	DependsOn       string // dependency name that must be installed first
	PostInstallCmd  string // command to run after install (e.g., browser download)
}

// IsNpmBased reports whether this dependency is installed via npm.
// @AX:NOTE [AUTO] public method with single call site; add test coverage for non-npm prefix cases
func (d Dependency) IsNpmBased() bool {
	return strings.HasPrefix(d.InstallCmd, "npm ")
}

// FullModeDeps는 Full 모드의 의존성 목록이다.
var FullModeDeps = []Dependency{
	// Core tools
	{Name: "git", Binary: "git", InstallCmd: platformInstallCmd("git"), Required: true, Description: "Version control"},
	{Name: "node", Binary: "node", InstallCmd: platformInstallCmd("node"), Required: true, Description: "Node.js runtime (npm packages, Playwright)"},
	{Name: "go", Binary: "go", InstallCmd: platformInstallCmd("go"), Required: false, Description: "Go toolchain (for Go projects)"},
	{Name: "python", Binary: pythonBinary(), InstallCmd: platformInstallCmd("python"), Required: false, Description: "Python runtime (for Python projects)"},
	// AI coding CLIs
	{Name: "claude", Binary: "claude", InstallCmd: "npm i -g @anthropic-ai/claude-code", Required: true, Description: "Claude Code CLI", DependsOn: "node"},
	{Name: "codex", Binary: "codex", InstallCmd: "npm i -g @openai/codex", Required: true, Description: "OpenAI Codex CLI", DependsOn: "node"},
	{Name: "antigravity", Binary: "agy", InstallCmd: antigravityInstallCmd(), InstallViaShell: true, Required: true, Description: "Google Antigravity CLI"},
	// Dev tools
	{Name: "ast-grep", Binary: "sg", InstallCmd: "npm i -g @ast-grep/cli", Required: true, Description: "Structural code search", DependsOn: "node"},
	{Name: "playwright", Binary: "playwright", InstallCmd: "npm i -g playwright", Required: false, Description: "E2E testing + screenshots", DependsOn: "node", PostInstallCmd: "npx playwright install chromium"},
	{Name: "agent-browser", Binary: "agent-browser", InstallCmd: "npm i -g agent-browser", Required: true, Description: "Web browsing", DependsOn: "node"},
	{Name: "gh", Binary: "gh", InstallCmd: platformInstallCmd("gh"), Required: true, Description: "GitHub CLI"},
}

// pythonBinary returns the python binary name for the current OS.
func pythonBinary() string {
	if runtime.GOOS == "windows" {
		return "python"
	}
	return "python3"
}

// platformInstallCmd returns the install command appropriate for the current OS.
func platformInstallCmd(name string) string {
	switch runtime.GOOS {
	case "darwin":
		return darwinInstallCmd(name)
	case "linux":
		return linuxInstallCmd(name)
	case "windows":
		return windowsInstallCmd(name)
	default:
		return ""
	}
}

func darwinInstallCmd(name string) string {
	cmds := map[string]string{
		"git":    "brew install git",
		"node":   "brew install node",
		"go":     "brew install go",
		"python": "brew install python",
		"gh":     "brew install gh",
	}
	return cmds[name]
}

func linuxInstallCmd(name string) string {
	cmds := map[string]string{
		"git":    "sudo apt-get install -y git",
		"node":   "sudo apt-get install -y nodejs npm",
		"go":     "sudo snap install go --classic",
		"python": "sudo apt-get install -y python3 python3-pip",
		"gh":     "sudo apt-get install -y gh",
	}
	return cmds[name]
}

func windowsInstallCmd(name string) string {
	// --accept-source-agreements --accept-package-agreements: prevent interactive hang
	// --disable-interactivity: no prompts (winget 1.6+)
	const wingetFlags = " --accept-source-agreements --accept-package-agreements --disable-interactivity"
	cmds := map[string]string{
		"git":    "winget install Git.Git" + wingetFlags,
		"node":   "winget install OpenJS.NodeJS.LTS" + wingetFlags,
		"go":     "winget install GoLang.Go" + wingetFlags,
		"python": "winget install Python.Python.3.12" + wingetFlags,
		"gh":     "winget install GitHub.cli" + wingetFlags,
	}
	return cmds[name]
}

func antigravityInstallCmd() string {
	if runtime.GOOS == "windows" {
		return "irm https://antigravity.google/cli/install.ps1 | iex"
	}
	return "curl -fsSL https://antigravity.google/cli/install.sh | bash"
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

// CheckParentRuleConflicts는 부모 디렉터리에 상속될 .claude/rules/ 네임스페이스가 있는지 탐지한다.
// Claude Code는 상위 디렉터리를 탐색하며 규칙을 로드하므로,
// 부모에 어떤 규칙 네임스페이스라도 있으면 현재 프로젝트에 의도치 않게 중첩 적용될 수 있다.
func CheckParentRuleConflicts(projectDir string) []ParentRuleConflict {
	absDir, err := filepath.Abs(projectDir)
	if err != nil {
		return nil
	}

	var conflicts []ParentRuleConflict
	current := filepath.Dir(absDir) // 부모부터 시작

	for {
		parent := filepath.Dir(current)
		if parent == current {
			// Reached filesystem root (Unix: /, Windows: C:\).
			break
		}
		rulesDir := filepath.Join(current, ".claude", "rules")
		entries, err := os.ReadDir(rulesDir)
		if err == nil {
			for _, entry := range entries {
				if entry.IsDir() {
					conflicts = append(conflicts, ParentRuleConflict{
						ParentDir: current,
						RulesDir:  rulesDir,
						Namespace: entry.Name(),
					})
				}
			}
		}
		current = parent
	}

	return conflicts
}
