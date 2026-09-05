package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/insajin/autopus-adk/internal/cli/tui"
)

const doctorDesktopShimCheckID = "doctor.launcher.desktop_shim"

const (
	// desktopShimBrokerFlag and desktopShimBanner are the markers Autopus
	// Desktop writes into the POSIX launcher it installs over the `auto` entry
	// on PATH. Either one identifies the launcher.
	desktopShimBrokerFlag = "--autopus-managed-adk-cli-broker"
	desktopShimBanner     = "Autopus Desktop managed auto launcher"

	// desktopShimBrokerRejection is the broker error users hit (exit 126) when
	// the managed slot no longer matches the Desktop manifest.
	desktopShimBrokerRejection = "managed_adk_broker_current_slot_rejected"

	// desktopManagedSlotRelPath is the home-relative macOS location of the
	// managed CLI slot the launcher brokers into.
	desktopManagedSlotRelPath = "Library/Application Support/co.autopus.desktop/managed-adk/current/auto"

	// desktopShimHeadBytes bounds the launcher read. The shim is a ~237 byte
	// shell script, so a real Go binary must never be slurped into memory.
	desktopShimHeadBytes = 4 << 10
)

// desktopShimUserHomeDir is a package-level seam, mirroring binary_path.go, so
// launcher tests can resolve the managed slot inside a temp home instead of the
// developer's real machine.
var desktopShimUserHomeDir = os.UserHomeDir

type desktopShimDiagnosis struct {
	// Found reports that the PATH entry is the Desktop launcher shim.
	Found bool
	// ShimPath is the PATH entry for `auto`, empty when `auto` is not on PATH.
	// It is populated even for a genuine CLI binary so the projections can tell
	// "not a launcher" apart from "nothing on PATH".
	ShimPath string
	// ManagedPath is the managed slot binary, empty where its location is not
	// known (non-macOS, or an unresolvable home directory).
	ManagedPath   string
	ManagedExists bool
	// SelfPath is the currently running binary, so the report can say whether
	// this process already is the managed slot.
	SelfPath string
}

func diagnoseDesktopShim() desktopShimDiagnosis {
	diagnosis := desktopShimDiagnosis{ManagedPath: desktopManagedSlotPath()}
	if diagnosis.ManagedPath != "" {
		if info, err := os.Stat(diagnosis.ManagedPath); err == nil && info.Mode().IsRegular() {
			diagnosis.ManagedExists = true
		}
	}
	if info, err := resolveCurrentBinaryPath(); err == nil {
		diagnosis.SelfPath = info.ManagedPath()
	}

	pathEntry, err := exec.LookPath("auto")
	if err != nil {
		return diagnosis
	}
	diagnosis.ShimPath = pathEntry
	diagnosis.Found = isDesktopShimFile(pathEntry)
	return diagnosis
}

func desktopManagedSlotPath() string {
	if runtime.GOOS != "darwin" {
		return ""
	}
	home, err := desktopShimUserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	return filepath.Join(home, filepath.FromSlash(desktopManagedSlotRelPath))
}

// isDesktopShimFile reports whether path is the Desktop launcher: a regular
// shell script carrying one of the launcher markers. The real CLI binary, and a
// symlink pointing at it, both fail the `#!` prefix test.
func isDesktopShimFile(path string) bool {
	head, ok := readDesktopShimHead(path)
	if !ok || !bytes.HasPrefix(head, []byte("#!")) {
		return false
	}
	return bytes.Contains(head, []byte(desktopShimBrokerFlag)) ||
		bytes.Contains(head, []byte(desktopShimBanner))
}

func readDesktopShimHead(path string) ([]byte, bool) {
	target := path
	if info, err := os.Lstat(path); err == nil && info.Mode()&os.ModeSymlink != 0 {
		resolved, err := filepath.EvalSymlinks(path)
		if err != nil {
			return nil, false
		}
		target = resolved
	}
	if info, err := os.Stat(target); err != nil || !info.Mode().IsRegular() {
		return nil, false
	}

	file, err := os.Open(target)
	if err != nil {
		return nil, false
	}
	defer file.Close()

	head := make([]byte, desktopShimHeadBytes)
	read, err := file.Read(head)
	if read == 0 || (err != nil && !errors.Is(err, io.EOF)) {
		return nil, false
	}
	return head[:read], true
}

// checkDesktopShimText projects the launcher diagnosis into the text report. It
// is advisory: a Desktop-managed installation is legitimate, so the warning
// never flips the doctor verdict, matching the Installed CLIs neighbour.
func checkDesktopShimText(w io.Writer, diagnosis desktopShimDiagnosis) {
	tui.SectionHeader(w, "Auto Launcher")
	switch {
	case diagnosis.ShimPath == "":
		tui.Info(w, "auto is not on PATH")
	case !diagnosis.Found:
		tui.OK(w, fmt.Sprintf("PATH auto is not a Desktop launcher (%s)", diagnosis.ShimPath))
	default:
		tui.SKIP(w, desktopShimSummaryLine(diagnosis))
		tui.Bullet(w, desktopShimManagedLine(diagnosis))
		if line := desktopShimWorkaroundLine(diagnosis); line != "" {
			tui.Bullet(w, line)
		}
		tui.Bullet(w, desktopShimRecoveryLine())
	}
}

// collectDesktopShimCheck mirrors checkDesktopShimText into the JSON report. A
// launcher on PATH shadows the CLI for every future invocation, so it raises an
// envelope warning the same way collectCLIChecks does for a PATH gap.
func (r *doctorJSONReport) collectDesktopShimCheck(diagnosis desktopShimDiagnosis) {
	switch {
	case diagnosis.ShimPath == "":
		r.checks = append(r.checks, jsonCheck{
			ID:       doctorDesktopShimCheckID,
			Severity: "info",
			Status:   "skip",
			Detail:   "auto is not on PATH",
		})
	case !diagnosis.Found:
		r.checks = append(r.checks, jsonCheck{
			ID:       doctorDesktopShimCheckID,
			Severity: "info",
			Status:   "pass",
			Detail:   fmt.Sprintf("PATH auto is not a Desktop launcher (%s)", diagnosis.ShimPath),
		})
	default:
		detail := desktopShimJSONDetail(diagnosis)
		r.status = jsonStatusWarn
		r.warnings = append(r.warnings, jsonMessage{
			Code:    "desktop_managed_launcher",
			Message: detail,
		})
		r.checks = append(r.checks, jsonCheck{
			ID:       doctorDesktopShimCheckID,
			Severity: "warning",
			Status:   "warn",
			Detail:   detail,
		})
	}
}

// desktopShimJSONDetail joins the same lines the text surface prints, so both
// projections always name the launcher, the managed slot and the recovery path.
func desktopShimJSONDetail(diagnosis desktopShimDiagnosis) string {
	lines := []string{
		desktopShimSummaryLine(diagnosis),
		desktopShimManagedLine(diagnosis),
		desktopShimRecoveryLine(),
	}
	return strings.Join(lines, "; ")
}

func desktopShimSummaryLine(diagnosis desktopShimDiagnosis) string {
	return fmt.Sprintf("PATH auto is the Autopus Desktop launcher at %s", diagnosis.ShimPath)
}

func desktopShimManagedLine(diagnosis desktopShimDiagnosis) string {
	if diagnosis.ManagedPath == "" {
		return "managed CLI location is unknown on this platform"
	}
	state := "missing"
	if diagnosis.ManagedExists {
		state = "exists"
	}
	if diagnosis.isRunningManagedSlot() {
		state += ", currently running"
	}
	return fmt.Sprintf("managed CLI at %s (%s)", diagnosis.ManagedPath, state)
}

func desktopShimWorkaroundLine(diagnosis desktopShimDiagnosis) string {
	if diagnosis.ManagedPath == "" {
		return ""
	}
	return fmt.Sprintf("workaround: run %q directly, or alias auto=%q",
		diagnosis.ManagedPath, diagnosis.ManagedPath)
}

func desktopShimRecoveryLine() string {
	return fmt.Sprintf(
		"if the launcher rejects with %s, re-run 'auto update' from the managed binary or reinstall Autopus Desktop",
		desktopShimBrokerRejection,
	)
}

func (diagnosis desktopShimDiagnosis) isRunningManagedSlot() bool {
	if diagnosis.SelfPath == "" || diagnosis.ManagedPath == "" {
		return false
	}
	return filepath.Clean(diagnosis.SelfPath) == filepath.Clean(diagnosis.ManagedPath)
}
