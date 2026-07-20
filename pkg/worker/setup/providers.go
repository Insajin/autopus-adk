package setup

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/processprobe"
)

const providerVersionTimeout = 2 * time.Second

// ProviderStatus describes the installation state of a CLI provider.
type ProviderStatus struct {
	Name      string
	Binary    string
	Installed bool
	Version   string
}

// providerPackages maps npm-installable provider names to their package names.
var providerPackages = map[string]string{
	"claude":   "@anthropic-ai/claude-code",
	"codex":    "@openai/codex",
	"opencode": "opencode",
}

// providerBinaries is the ordered list of provider names and binaries to detect.
var providerBinaries = []ProviderStatus{
	{Name: "claude", Binary: "claude"},
	{Name: "codex", Binary: "codex"},
	{Name: "gemini", Binary: "agy"},
	{Name: "opencode", Binary: "opencode"},
}

// DetectProviders checks which CLI providers are installed on the system.
func DetectProviders() []ProviderStatus {
	results := make([]ProviderStatus, 0, len(providerBinaries))
	for _, provider := range providerBinaries {
		ps := ProviderStatus{
			Name:   provider.Name,
			Binary: provider.Binary,
		}

		path, err := exec.LookPath(provider.Binary)
		if err != nil {
			results = append(results, ps)
			continue
		}

		ps.Installed = true
		ps.Version = detectVersion(path)
		results = append(results, ps)
	}
	return results
}

// detectVersion runs "{binary} --version" and returns the output.
func detectVersion(binaryPath string) string {
	ctx, cancel := context.WithTimeout(context.Background(), providerVersionTimeout)
	defer cancel()
	out, err := processprobe.Output(exec.CommandContext(ctx, binaryPath, "--version"))
	if err != nil {
		return "unknown"
	}
	return strings.TrimSpace(string(out))
}

// InstallProvider installs a provider via npm.
func InstallProvider(name string) error {
	if name == "gemini" {
		return installAntigravityCLI()
	}

	pkg, ok := providerPackages[name]
	if !ok {
		return fmt.Errorf("unknown provider: %s", name)
	}

	if !checkNPM() {
		return fmt.Errorf("npm is not installed; install Node.js first")
	}

	cmd := exec.Command("npm", "install", "-g", pkg)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install %s (%s): %w", name, pkg, err)
	}
	return nil
}

func installAntigravityCLI() error {
	installCmd := antigravityInstallCommand()
	shell, args := shellCommand(installCmd)
	cmd := exec.Command(shell, args...)
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("install gemini (Antigravity CLI): %w", err)
	}
	return nil
}

func antigravityInstallCommand() string {
	if runtime.GOOS == "windows" {
		return "irm https://antigravity.google/cli/install.ps1 | iex"
	}
	return "curl -fsSL https://antigravity.google/cli/install.sh | bash"
}

func shellCommand(command string) (string, []string) {
	if runtime.GOOS == "windows" {
		return "powershell", []string{"-NoProfile", "-ExecutionPolicy", "Bypass", "-Command", command}
	}
	return "sh", []string{"-c", command}
}

// CheckNodeJS returns true if node is available on PATH.
func CheckNodeJS() bool {
	_, err := exec.LookPath("node")
	return err == nil
}

// checkNPM returns true if npm is available on PATH.
func checkNPM() bool {
	_, err := exec.LookPath("npm")
	return err == nil
}

// InstallNodeJS attempts to install Node.js via Homebrew (macOS).
func InstallNodeJS() error {
	brewPath, err := exec.LookPath("brew")
	if err != nil {
		return fmt.Errorf("brew not found; install Node.js manually from https://nodejs.org")
	}

	cmd := exec.Command(brewPath, "install", "node")
	cmd.Stdout = nil
	cmd.Stderr = nil
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("brew install node: %w", err)
	}
	return nil
}
