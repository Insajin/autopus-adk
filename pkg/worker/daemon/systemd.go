package daemon

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

// unitTemplate is the systemd user unit file template.
var unitTemplate = template.Must(template.New("unit").Parse(`[Unit]
Description=Autopus Worker Daemon
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
ExecStart={{ .ExecStart }}
Restart=always
RestartSec=5
Environment=HOME=%h
StandardOutput=append:{{ .LogDir }}/autopus-worker.out.log
StandardError=append:{{ .LogDir }}/autopus-worker.err.log

[Install]
WantedBy=default.target
`))

// unitData holds template rendering data for systemd unit.
type unitData struct {
	ExecStart string
	LogDir    string
}

// @AX:NOTE[AUTO]: magic constant — systemdUnitName is the systemd service identifier; changing this orphans existing installed units
const systemdUnitName = "autopus-worker.service"

// systemdUnitPath returns the user unit file path.
func systemdUnitPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", systemdUnitName)
	}
	return filepath.Join(home, ".config", "systemd", "user", systemdUnitName)
}

// GenerateUnit returns the systemd unit file content for the worker daemon.
func GenerateUnit(cfg LaunchdConfig) (string, error) {
	parts := []string{cfg.BinaryPath}
	parts = append(parts, cfg.Args...)
	execStart := strings.Join(parts, " ")

	logDir := cfg.LogDir
	if logDir == "" {
		home, _ := os.UserHomeDir()
		logDir = filepath.Join(home, ".config", "autopus", "logs")
	}

	data := unitData{ExecStart: execStart, LogDir: logDir}

	var buf []byte
	w := &byteWriter{buf: &buf}
	if err := unitTemplate.Execute(w, data); err != nil {
		return "", fmt.Errorf("render unit template: %w", err)
	}
	return string(buf), nil
}

// InstallSystemd writes the unit file and enables it via systemctl.
func InstallSystemd(cfg LaunchdConfig) error {
	content, err := GenerateUnit(cfg)
	if err != nil {
		return err
	}

	path := systemdUnitPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create systemd user dir: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write unit file: %w", err)
	}
	log.Printf("[daemon] unit file written to %s", path)

	// Reload and enable the service.
	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl daemon-reload: %s: %w", string(out), err)
	}
	if out, err := exec.Command("systemctl", "--user", "enable", "--now", systemdUnitName).CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl enable: %s: %w", string(out), err)
	}

	log.Printf("[daemon] systemd service enabled: %s", systemdUnitName)
	return nil
}

// UninstallSystemd disables the service and removes the unit file.
func UninstallSystemd() error {
	// Disable and stop the service.
	cmd := exec.Command("systemctl", "--user", "disable", "--now", systemdUnitName)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[daemon] systemctl disable warning: %s: %v", string(out), err)
	}

	path := systemdUnitPath()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove unit file: %w", err)
	}

	// Reload after removal.
	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		log.Printf("[daemon] systemctl daemon-reload warning: %s: %v", string(out), err)
	}

	log.Printf("[daemon] systemd service uninstalled")
	return nil
}

// IsSystemdInstalled checks whether the unit file exists.
func IsSystemdInstalled() bool {
	_, err := os.Stat(systemdUnitPath())
	return err == nil
}
