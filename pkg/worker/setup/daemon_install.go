package setup

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"

	"github.com/insajin/autopus-adk/pkg/version"
)

const (
	autoMainPackagePath = "github.com/insajin/autopus-adk/cmd/auto"
	autoModulePath      = "github.com/insajin/autopus-adk"
)

type daemonExecutableIdentity struct {
	PackagePath   string
	ModulePath    string
	ModuleVersion string
	BuildVersion  string
	BuildCommit   string
}

// installAndStartDaemon installs the worker daemon for the current OS.
// It resolves the current binary path and writes OS-specific service config.
// @AX:ANCHOR: [AUTO] OS daemon installation integration boundary.
// @AX:REASON: [AUTO] Worker setup must pass executable build identity validation before launchd or systemd configuration can be written.
func installAndStartDaemon() error {
	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve binary path: %w", err)
	}
	// Resolve symlinks so daemon points to the real binary.
	if resolved, err := filepath.EvalSymlinks(binPath); err == nil {
		binPath = resolved
	}
	if err := validateDaemonExecutable(binPath); err != nil {
		return err
	}

	if runtime.GOOS == "darwin" {
		return installLaunchdDaemon(binPath)
	}
	return installSystemdDaemon(binPath)
}

func validateDaemonExecutable(binPath string) error {
	identity, err := currentDaemonExecutableIdentity()
	if err != nil {
		return fmt.Errorf("refusing to install worker daemon: %w", err)
	}
	return validateDaemonExecutableIdentity(binPath, identity)
}

func currentDaemonExecutableIdentity() (daemonExecutableIdentity, error) {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return daemonExecutableIdentity{}, fmt.Errorf("current executable build identity is unavailable")
	}
	return daemonExecutableIdentity{
		PackagePath:   info.Path,
		ModulePath:    info.Main.Path,
		ModuleVersion: info.Main.Version,
		BuildVersion:  version.Version(),
		BuildCommit:   version.Commit(),
	}, nil
}

func validateDaemonExecutableIdentity(binPath string, identity daemonExecutableIdentity) error {
	if filepath.Base(filepath.Clean(binPath)) != "auto" {
		return fmt.Errorf("refusing to install worker daemon from non-auto executable %q", binPath)
	}
	if identity.PackagePath != autoMainPackagePath ||
		identity.ModulePath != autoModulePath ||
		!hasProductionDaemonBuildMetadata(identity) {
		return fmt.Errorf(
			"refusing to install worker daemon from untrusted auto build identity %q",
			identity.PackagePath,
		)
	}
	return nil
}

func hasProductionDaemonBuildMetadata(identity daemonExecutableIdentity) bool {
	buildVersion := strings.TrimPrefix(strings.TrimSpace(identity.BuildVersion), "v")
	if buildVersion == "" ||
		buildVersion == "dev" ||
		strings.HasPrefix(buildVersion, "dev-") ||
		buildVersion == "unknown" ||
		strings.Contains(buildVersion, "dirty") {
		return false
	}

	buildCommit := strings.TrimSpace(identity.BuildCommit)
	if buildCommit != "" && buildCommit != "none" {
		return true
	}
	moduleVersion := strings.TrimSpace(identity.ModuleVersion)
	return moduleVersion != "" && moduleVersion != "(devel)"
}

// installLaunchdDaemon writes a plist and loads it via launchctl (macOS).
func installLaunchdDaemon(binPath string) error {
	// Already running — no-op.
	if exec.Command("launchctl", "list", "co.autopus.worker").Run() == nil {
		return nil
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	logDir := filepath.Join(home, ".config", "autopus", "logs")
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("create log dir: %w", err)
	}

	plistPath := filepath.Join(home, "Library", "LaunchAgents", "co.autopus.worker.plist")
	if err := os.MkdirAll(filepath.Dir(plistPath), 0755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	plistContent := buildPlistContent(binPath, logDir)
	if err := os.WriteFile(plistPath, []byte(plistContent), 0644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}

	if out, err := exec.Command("launchctl", "load", plistPath).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load: %s: %w", string(out), err)
	}
	return nil
}

// buildPlistContent generates the launchd plist XML.
func buildPlistContent(binPath, logDir string) string {
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>co.autopus.worker</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>worker</string>
        <string>start</string>
    </array>
    <key>KeepAlive</key>
    <true/>
    <key>RunAtLoad</key>
    <true/>
    <key>StandardOutPath</key>
    <string>%s/autopus-worker.out.log</string>
    <key>StandardErrorPath</key>
    <string>%s/autopus-worker.err.log</string>
</dict>
</plist>`, binPath, logDir, logDir)
}

// installSystemdDaemon writes a unit file and enables it via systemctl (Linux).
func installSystemdDaemon(binPath string) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home dir: %w", err)
	}

	unitDir := filepath.Join(home, ".config", "systemd", "user")
	if err := os.MkdirAll(unitDir, 0755); err != nil {
		return fmt.Errorf("create systemd user dir: %w", err)
	}

	unitPath := filepath.Join(unitDir, "autopus-worker.service")
	unitContent := buildSystemdUnit(binPath)
	if err := os.WriteFile(unitPath, []byte(unitContent), 0644); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}

	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %s: %w", string(out), err)
	}
	if out, err := exec.Command("systemctl", "--user", "enable", "--now", "autopus-worker.service").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl enable: %s: %w", string(out), err)
	}
	return nil
}

// buildSystemdUnit generates the systemd unit file content.
func buildSystemdUnit(binPath string) string {
	return fmt.Sprintf(`[Unit]
Description=Autopus Worker Daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart=%s worker start
Restart=always
RestartSec=5
Environment=HOME=%%h

[Install]
WantedBy=default.target
`, binPath)
}
