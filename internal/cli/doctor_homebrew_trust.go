package cli

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/internal/cli/tui"
)

type homebrewTrustDiagnosis struct{ state, detail string }
type homebrewTrustRunner func(context.Context, ...string) ([]byte, error)

func diagnoseHomebrewTrust(ctx context.Context) homebrewTrustDiagnosis {
	brew, err := exec.LookPath("brew")
	if err != nil {
		return homebrewTrustDiagnosis{"skip", "Homebrew is not on PATH"}
	}
	return diagnoseHomebrewTrustWith(ctx, func(ctx context.Context, args ...string) ([]byte, error) {
		binary := brew
		if len(args) > 0 && args[0] == "git" {
			binary = "git"
			args = args[1:]
		}
		cmd := exec.CommandContext(ctx, binary, args...)
		cmd.WaitDelay = time.Second
		cmd.Env = append(os.Environ(), "HOMEBREW_NO_AUTO_UPDATE=1", "HOMEBREW_NO_ANALYTICS=1")
		return cmd.Output()
	})
}

// Inspect the tap definition even when the running CLI is not a Brew install:
// cleanup can load it to process old cached ADK archives. Never evaluate Ruby.
func diagnoseHomebrewTrustWith(ctx context.Context, run homebrewTrustRunner) homebrewTrustDiagnosis {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	unknown := homebrewTrustDiagnosis{"unknown", "Homebrew tap trust could not be verified; inspect 'brew trust --json=v1'"}
	version, err := run(ctx, "--version")
	if err != nil {
		return unknown
	}
	fields := strings.Fields(string(version))
	if len(fields) < 2 || fields[0] != "Homebrew" {
		return unknown
	}
	major, err := strconv.Atoi(strings.Split(fields[1], ".")[0])
	if err != nil {
		return unknown
	}
	if major < 6 {
		return homebrewTrustDiagnosis{"skip", "Homebrew before 6: default tap trust requirement does not apply"}
	}
	repo, err := run(ctx, "--repository")
	if err != nil || !filepath.IsAbs(strings.TrimSpace(string(repo))) {
		return unknown
	}
	tapDir := filepath.Join(strings.TrimSpace(string(repo)), "Library/Taps/insajin/homebrew-autopus")
	found := false
	for _, path := range []string{"Casks/auto.rb", "Casks/a/auto.rb"} {
		info, err := os.Stat(filepath.Join(tapDir, path))
		if err != nil && !os.IsNotExist(err) {
			return unknown
		}
		if err == nil && info.Mode().IsRegular() {
			found = true
		}
	}
	if !found {
		return homebrewTrustDiagnosis{"skip", "Homebrew ADK tap cask is not present"}
	}
	// Check the configured origin without invoking tap-info's private-repo API probe.
	origin, err := run(ctx, "git", "-C", tapDir, "config", "--get", "remote.origin.url")
	if err != nil {
		return unknown
	}
	remote := strings.TrimSuffix(strings.ToLower(strings.TrimSpace(string(origin))), ".git")
	if remote != "https://github.com/insajin/homebrew-autopus" && remote != "git@github.com:insajin/homebrew-autopus" && remote != "ssh://git@github.com/insajin/homebrew-autopus" {
		return unknown
	}
	raw, err := run(ctx, "trust", "--json=v1")
	if err != nil {
		return unknown
	}
	var trusted struct {
		Taps  []string `json:"taps"`
		Casks []string `json:"casks"`
	}
	if json.Unmarshal(raw, &trusted) != nil || trusted.Taps == nil || trusted.Casks == nil {
		return unknown
	}
	for _, entry := range trusted.Casks {
		if strings.EqualFold(entry, "insajin/autopus/auto") {
			return homebrewTrustDiagnosis{"pass", "Homebrew ADK cask is explicitly trusted"}
		}
	}
	for _, entry := range trusted.Taps {
		if strings.EqualFold(entry, "insajin/autopus") {
			return homebrewTrustDiagnosis{"pass", "Homebrew ADK tap is explicitly trusted"}
		}
	}
	// Remote-based trust depends on the tap's configured origin. Avoid declaring
	// it untrusted without resolving Homebrew's remote equivalence semantics.
	for _, entry := range append(trusted.Taps, trusted.Casks...) {
		if strings.Contains(entry, ":") {
			return unknown
		}
	}
	return homebrewTrustDiagnosis{"warn", "Homebrew ADK cask lacks explicit trust; cached ADK archives may cause cleanup and unrelated upgrades to fail. Run 'brew trust --cask insajin/autopus/auto', then retry the failed command"}
}

func checkHomebrewTrustText(w io.Writer, d homebrewTrustDiagnosis) bool {
	tui.SectionHeader(w, "Homebrew Tap Trust")
	if d.state == "pass" {
		tui.OK(w, d.detail)
	} else {
		tui.SKIP(w, d.detail)
	}
	return d.state != "warn" && d.state != "unknown"
}

func (r *doctorJSONReport) collectHomebrewTrustCheck(d homebrewTrustDiagnosis) {
	status, severity := d.state, "info"
	if d.state == "warn" || d.state == "unknown" {
		status, severity = "warn", "warning"
		r.status = jsonStatusWarn
		r.warnings = append(r.warnings, jsonMessage{Code: "homebrew_tap_trust_" + d.state, Message: d.detail})
	}
	r.checks = append(r.checks, jsonCheck{ID: "doctor.homebrew.tap_trust", Status: status, Severity: severity, Detail: d.detail})
}
