package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/worker/daemon"
	"github.com/insajin/autopus-adk/pkg/worker/pidlock"
	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

// addWorkerSubcommands registers all worker subcommands on the parent command.
// @AX:ANCHOR[AUTO]: CLI command registration boundary — all worker subcommands are wired here; adding/removing commands requires updating this function
func addWorkerSubcommands(parent *cobra.Command) {
	parent.AddCommand(
		newWorkerStartCmd(),
		newWorkerStopCmd(),
		newWorkerStatusCmd(),
		newWorkerLogsCmd(),
		newWorkerRestartCmd(),
		newWorkerHistoryCmd(),
		newWorkerCostCmd(),
		newWorkerSetupCmd(),
		newWorkerEnsureCmd(),
	)
}

func newWorkerStartCmd() *cobra.Command {
	var daemonFlag bool
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the worker (foreground or daemon)",
		RunE: func(cmd *cobra.Command, args []string) error {
			if daemonFlag {
				return installDaemon()
			}
			return runWorkerForeground()
		},
	}
	cmd.Flags().BoolVar(&daemonFlag, "daemon", false, "Install and start as system daemon")
	return cmd
}

func newWorkerStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Stop the worker daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			if runtime.GOOS == "darwin" {
				if err := daemon.UninstallLaunchd(); err != nil {
					return fmt.Errorf("stop launchd daemon: %w", err)
				}
				fmt.Fprintln(cmd.OutOrStdout(), "Worker daemon stopped (launchd).")
				return nil
			}
			if err := daemon.UninstallSystemd(); err != nil {
				return fmt.Errorf("stop systemd daemon: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Worker daemon stopped (systemd).")
			return nil
		},
	}
}

func newWorkerStatusCmd() *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show worker daemon status",
		RunE: func(cmd *cobra.Command, args []string) error {
			if jsonOutput {
				// Machine-readable JSON output — no other output.
				s := setup.CollectStatus()
				enc := json.NewEncoder(cmd.OutOrStdout())
				enc.SetIndent("", "  ")
				return enc.Encode(s)
			}
			// Human-readable output (existing behavior).
			out := cmd.OutOrStdout()
			installed := isDaemonInstalled()
			fmt.Fprintf(out, "Daemon installed: %v\n", installed)
			fmt.Fprintf(out, "Platform: %s\n", runtime.GOOS)
			printPIDStatus(out)
			if installed {
				printDaemonStatus(cmd)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Output status as JSON")
	return cmd
}

func newWorkerLogsCmd() *cobra.Command {
	var taskFilter string
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Tail worker log file",
		RunE: func(cmd *cobra.Command, args []string) error {
			logPath := workerLogPath()
			if _, err := os.Stat(logPath); os.IsNotExist(err) {
				return fmt.Errorf("log file not found: %s", logPath)
			}
			data, err := os.ReadFile(logPath)
			if err != nil {
				return fmt.Errorf("read log: %w", err)
			}
			lines := strings.Split(string(data), "\n")
			for _, line := range lines {
				if taskFilter == "" || strings.Contains(line, taskFilter) {
					fmt.Fprintln(cmd.OutOrStdout(), line)
				}
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&taskFilter, "task", "", "Filter logs by task ID")
	return cmd
}

func newWorkerRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Restart the worker daemon",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Stop ignoring errors (may not be running).
			if runtime.GOOS == "darwin" {
				_ = daemon.UninstallLaunchd()
			} else {
				_ = daemon.UninstallSystemd()
			}
			if err := installDaemon(); err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), "Worker daemon restarted.")
			return nil
		},
	}
}

func newWorkerHistoryCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "Show recent task history",
		RunE: func(cmd *cobra.Command, args []string) error {
			histPath := workerDataPath("task-history.log")
			if _, err := os.Stat(histPath); os.IsNotExist(err) {
				fmt.Fprintln(cmd.OutOrStdout(), "No task history found.")
				return nil
			}
			data, err := os.ReadFile(histPath)
			if err != nil {
				return fmt.Errorf("read history: %w", err)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
}

func newWorkerCostCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cost",
		Short: "Show cost summary",
		RunE: func(cmd *cobra.Command, args []string) error {
			costPath := workerDataPath("cost.log")
			if _, err := os.Stat(costPath); os.IsNotExist(err) {
				fmt.Fprintln(cmd.OutOrStdout(), "No cost data found.")
				return nil
			}
			data, err := os.ReadFile(costPath)
			if err != nil {
				return fmt.Errorf("read cost log: %w", err)
			}
			fmt.Fprint(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
}

func newWorkerSetupCmd() *cobra.Command {
	var backendURL string
	var token string
	var workspaceID string
	var apiKey string
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Run worker setup wizard",
		Long: `Worker는 Autopus 서버에서 작업을 받아 자동으로 실행하는 백그라운드 서비스입니다.

이 명령어는 3단계 설정 과정을 안내합니다:
  1. Autopus 서버 인증 (브라우저에서 로그인)
  2. 워크스페이스 선택
  3. AI 프로바이더 확인 (Claude, Codex, Gemini)

비대화형 모드 (에이전트/CI 환경):
  auto worker setup --token <jwt> --workspace <workspace-id>
  auto worker setup --api-key <acos_worker_...> --workspace <workspace-id>`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkerSetup(cmd, backendURL, token, workspaceID, apiKey)
		},
	}
	// @AX:NOTE[AUTO]: magic constant — default backend URL "https://api.autopus.co" is hardcoded; must match production endpoint
	cmd.Flags().StringVar(&backendURL, "backend", "https://api.autopus.co", "Backend API URL")
	cmd.Flags().StringVar(&token, "token", "", "Pre-obtained JWT — skips browser OAuth (for agents/CI)")
	cmd.Flags().StringVar(&workspaceID, "workspace", "", "Workspace ID — skips interactive selection")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "Worker API Key (acos_worker_...) — skips browser OAuth")
	return cmd
}

// installDaemon installs the worker as a system daemon based on OS.
func installDaemon() error {
	binPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve binary path: %w", err)
	}

	cfg := daemon.LaunchdConfig{
		BinaryPath: binPath,
		Args:       []string{"worker", "start"},
	}

	if runtime.GOOS == "darwin" {
		return daemon.InstallLaunchd(cfg)
	}
	return daemon.InstallSystemd(cfg)
}

// isDaemonInstalled checks if the daemon is installed on the current OS.
func isDaemonInstalled() bool {
	if runtime.GOOS == "darwin" {
		return daemon.IsLaunchdInstalled()
	}
	return daemon.IsSystemdInstalled()
}

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
