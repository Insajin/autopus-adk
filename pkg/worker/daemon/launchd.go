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

// @AX:NOTE[AUTO]: magic constant — launchdLabel is the macOS launchd service identifier; changing this orphans existing installed agents
const (
	launchdLabel = "co.autopus.worker"
)

// LaunchdConfig holds configuration for macOS launchd plist generation.
type LaunchdConfig struct {
	BinaryPath string
	Args       []string
	LogDir     string
}

// xmlEscape replaces XML-special characters in dynamic values to produce valid plist XML.
func xmlEscape(s string) string {
	r := strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", "'", "&apos;", "\"", "&quot;")
	return r.Replace(s)
}

// plistTemplate is the launchd plist XML template.
// Dynamic values are escaped via the xmlEsc function to prevent invalid XML.
var plistTemplate = template.Must(template.New("plist").Funcs(template.FuncMap{
	"xmlEsc": xmlEscape,
}).Parse(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>{{ .Label | xmlEsc }}</string>
    <key>ProgramArguments</key>
    <array>
        <string>{{ .BinaryPath | xmlEsc }}</string>
{{- range .Args }}
        <string>{{ . | xmlEsc }}</string>
{{- end }}
    </array>
    <key>KeepAlive</key>
    <true/>
    <key>RunAtLoad</key>
    <true/>
    <key>EnvironmentVariables</key>
    <dict>
        <key>PATH</key>
        {{/* @AX:WARN[AUTO]: hardcoded PATH includes cmux-specific path — breaks on machines without cmux installed; should be configurable */}}
        <string>/Applications/cmux.app/Contents/Resources/bin:/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin</string>
    </dict>
    <key>ProcessType</key>
    <string>Background</string>
    <key>ThrottleInterval</key>
    <integer>10</integer>
    <key>StandardOutPath</key>
    <string>{{ .LogDir | xmlEsc }}/autopus-worker.out.log</string>
    <key>StandardErrorPath</key>
    <string>{{ .LogDir | xmlEsc }}/autopus-worker.err.log</string>
</dict>
</plist>
`))

// plistData holds template rendering data.
type plistData struct {
	Label      string
	BinaryPath string
	Args       []string
	LogDir     string
}

// launchdPlistPath returns the plist file path.
func launchdPlistPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", launchdLabel+".plist")
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist")
}

// GeneratePlist returns the XML plist content for the worker daemon.
func GeneratePlist(cfg LaunchdConfig) (string, error) {
	logDir := cfg.LogDir
	if logDir == "" {
		home, _ := os.UserHomeDir()
		logDir = filepath.Join(home, ".config", "autopus", "logs")
	}

	data := plistData{
		Label:      launchdLabel,
		BinaryPath: cfg.BinaryPath,
		Args:       cfg.Args,
		LogDir:     logDir,
	}

	var buf []byte
	w := &byteWriter{buf: &buf}
	if err := plistTemplate.Execute(w, data); err != nil {
		return "", fmt.Errorf("render plist template: %w", err)
	}
	return string(buf), nil
}

// byteWriter collects template output into a byte slice.
type byteWriter struct {
	buf *[]byte
}

func (bw *byteWriter) Write(p []byte) (int, error) {
	*bw.buf = append(*bw.buf, p...)
	return len(p), nil
}

// InstallLaunchd writes the plist file and loads it via launchctl.
func InstallLaunchd(cfg LaunchdConfig) error {
	content, err := GeneratePlist(cfg)
	if err != nil {
		return err
	}

	path := launchdPlistPath()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("create LaunchAgents dir: %w", err)
	}

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		return fmt.Errorf("write plist: %w", err)
	}
	log.Printf("[daemon] plist written to %s", path)

	// Load the agent via launchctl.
	cmd := exec.Command("launchctl", "load", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl load: %s: %w", string(out), err)
	}

	log.Printf("[daemon] launchd agent loaded: %s", launchdLabel)
	return nil
}

// UninstallLaunchd unloads the agent and removes the plist file.
func UninstallLaunchd() error {
	path := launchdPlistPath()

	cmd := exec.Command("launchctl", "unload", path)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("[daemon] launchctl unload warning: %s: %v", string(out), err)
	}

	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove plist: %w", err)
	}

	log.Printf("[daemon] launchd agent uninstalled")
	return nil
}

// IsLaunchdInstalled checks whether the plist file exists.
func IsLaunchdInstalled() bool {
	_, err := os.Stat(launchdPlistPath())
	return err == nil
}
