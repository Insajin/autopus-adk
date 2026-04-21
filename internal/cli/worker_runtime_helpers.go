package cli

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/worker/pidlock"
)

// printDaemonStatus prints OS-specific daemon status information.
func printDaemonStatus(cmd *cobra.Command) {
	out := cmd.OutOrStdout()
	if runtime.GOOS == "darwin" {
		result, err := exec.Command("launchctl", "list", "co.autopus.worker").CombinedOutput()
		if err == nil {
			fmt.Fprintf(out, "Launchd status:\n%s", string(result))
		}
		return
	}
	result, err := exec.Command("systemctl", "--user", "status", "autopus-worker.service").CombinedOutput()
	if err == nil {
		fmt.Fprintf(out, "Systemd status:\n%s", string(result))
	}
}

// workerLogPath returns the path to the worker log file.
func workerLogPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "autopus", "logs", "autopus-worker.out.log")
}

// workerDataPath returns a path under the autopus config directory.
func workerDataPath(name string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "autopus", name)
}

// printPIDStatus writes PID information to out using the default PID lock file.
func printPIDStatus(out io.Writer) {
	pidPath := pidlock.DefaultPath()
	lk := pidlock.New(pidPath)
	pid, err := lk.ReadPID()
	if err != nil {
		fmt.Fprintf(out, "PID: not running\n")
		return
	}
	fmt.Fprintf(out, "PID: %d\n", pid)
}
