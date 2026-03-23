package cli

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/insajin/autopus-adk/pkg/detect"
)

// execFunc is the signature for running a shell command. Injected for testing.
type execFunc func(name string, args ...string) ([]byte, error)

// defaultExec runs the command and returns combined output.
// @AX:WARN [AUTO] executes install commands derived from dependency config without validation
// @AX:REASON: install command strings (e.g. InstallCmd) are user-visible but hardcoded in detect.go; still, any future dynamic source would introduce command injection risk
func defaultExec(name string, args ...string) ([]byte, error) {
	return exec.Command(name, args...).CombinedOutput() //nolint:gosec
}

// filterMissing returns only the statuses where the dependency is not installed.
func filterMissing(statuses []detect.DependencyStatus) []detect.DependencyStatus {
	var missing []detect.DependencyStatus
	for _, s := range statuses {
		if !s.Installed {
			missing = append(missing, s)
		}
	}
	return missing
}

// runDoctorFix installs each missing dependency in order, respecting DependsOn.
// If autoYes is false, the user is prompted before each install.
func runDoctorFix(w io.Writer, deps []detect.DependencyStatus, autoYes bool) error {
	return runDoctorFixWith(w, deps, autoYes, defaultExec, bufio.NewReader(os.Stdin))
}

// runDoctorFixWith is the injectable version used in tests.
// @AX:ANCHOR [AUTO] central install orchestrator — called by runDoctorFix and directly in tests
// @AX:REASON: fan_in from doctor.go (via runDoctorFix) and doctor_fix_test.go; logic changes here affect all fix flows
func runDoctorFixWith(
	w io.Writer,
	deps []detect.DependencyStatus,
	autoYes bool,
	run execFunc,
	reader *bufio.Reader,
) error {
	ordered := orderByDependency(deps)

	// Track which deps were successfully installed in this run.
	installed := map[string]bool{}

	for _, s := range ordered {
		dep := s.Dependency

		// Skip deps whose prerequisite is still missing.
		if dep.DependsOn != "" && !detect.IsInstalled(dep.DependsOn) && !installed[dep.DependsOn] {
			tui.SKIP(w, fmt.Sprintf("%s: skipped (requires %s)", dep.Name, dep.DependsOn))
			continue
		}

		// npm-based deps require node.
		if dep.IsNpmBased() && !detect.IsInstalled("node") && !installed["node"] {
			tui.SKIP(w, fmt.Sprintf("%s: skipped (node not installed)", dep.Name))
			continue
		}

		if !autoYes {
			fmt.Fprintf(w, "  Install %s? [y/N] ", dep.Name)
			line, err := reader.ReadString('\n')
			if err != nil {
				return fmt.Errorf("reading input: %w", err)
			}
			if !strings.HasPrefix(strings.TrimSpace(strings.ToLower(line)), "y") {
				tui.SKIP(w, fmt.Sprintf("%s: skipped by user", dep.Name))
				continue
			}
		}

		tui.Info(w, fmt.Sprintf("Installing %s: %s", dep.Name, dep.InstallCmd))
		parts := strings.Fields(dep.InstallCmd)
		if len(parts) == 0 {
			tui.SKIP(w, fmt.Sprintf("%s: no install command defined", dep.Name))
			continue
		}

		out, err := run(parts[0], parts[1:]...)
		if err != nil {
			// Suggest npm prefix workaround on permission errors.
			if strings.Contains(string(out), "EACCES") {
				tui.FAIL(w, fmt.Sprintf("%s install failed (permission denied)", dep.Name))
				tui.Bullet(w, "Try: npm config set prefix ~/.npm-global && export PATH=$HOME/.npm-global/bin:$PATH")
			} else {
				tui.FAIL(w, fmt.Sprintf("%s install failed: %v", dep.Name, err))
				tui.Bullet(w, fmt.Sprintf("Output: %s", strings.TrimSpace(string(out))))
			}
			continue
		}

		tui.OK(w, fmt.Sprintf("%s installed", dep.Name))
		installed[dep.Name] = true

		// Run post-install command if defined (e.g., playwright browser download).
		if dep.PostInstallCmd != "" {
			tui.Info(w, fmt.Sprintf("Post-install: %s", dep.PostInstallCmd))
			pparts := strings.Fields(dep.PostInstallCmd)
			if pout, perr := run(pparts[0], pparts[1:]...); perr != nil {
				tui.FAIL(w, fmt.Sprintf("%s post-install failed: %v", dep.Name, perr))
				tui.Bullet(w, fmt.Sprintf("Output: %s", strings.TrimSpace(string(pout))))
			} else {
				tui.OK(w, fmt.Sprintf("%s post-install complete", dep.Name))
			}
		}
	}

	// Re-check and display updated dependency status.
	tui.SectionHeader(w, "Updated Status")
	updated := detect.CheckDependencies(detect.FullModeDeps)
	for _, s := range updated {
		if s.Installed {
			tui.OK(w, s.Name)
		} else if s.Required {
			tui.FAIL(w, fmt.Sprintf("%s not installed", s.Name))
		} else {
			tui.SKIP(w, fmt.Sprintf("%s not installed (optional)", s.Name))
		}
	}

	return nil
}

// orderByDependency sorts deps so that prerequisites come before dependants.
// It uses a simple two-pass approach since dependency chains are shallow.
func orderByDependency(deps []detect.DependencyStatus) []detect.DependencyStatus {
	var first, rest []detect.DependencyStatus
	depNames := make(map[string]bool, len(deps))
	for _, s := range deps {
		depNames[s.Name] = true
	}

	for _, s := range deps {
		if s.DependsOn != "" && depNames[s.DependsOn] {
			rest = append(rest, s)
		} else {
			first = append(first, s)
		}
	}
	return append(first, rest...)
}
