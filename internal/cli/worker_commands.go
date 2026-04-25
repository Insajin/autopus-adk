package cli

import (
	"fmt"
	"os"
	"runtime"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/worker/daemon"
	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

// addWorkerSubcommands registers all worker subcommands on the parent command.
// @AX:NOTE[AUTO]: [downgraded from ANCHOR — fan_in < 3] CLI command registration boundary — worker_validate.go is the sole production caller; update this function when worker commands change
func addWorkerSubcommands(parent *cobra.Command) {
	startCmd := newWorkerStartCmd()
	sidecarCmd := newWorkerSidecarCmd()
	stopCmd := newWorkerStopCmd()
	statusCmd := newWorkerStatusCmd()
	sessionCmd := newWorkerSessionCmd()
	logsCmd := newWorkerLogsCmd()
	restartCmd := newWorkerRestartCmd()
	historyCmd := newWorkerHistoryCmd()
	costCmd := newWorkerCostCmd()
	setupCmd := newWorkerSetupCmd()
	ensureCmd := newWorkerEnsureCmd()
	markLegacyLocalHostWorker(startCmd)
	markLegacyLocalHostWorker(stopCmd)
	markLegacyLocalHostWorker(statusCmd)
	markLegacyLocalHostWorker(logsCmd)
	markLegacyLocalHostWorker(restartCmd)
	markLegacyLocalHostWorker(historyCmd)
	markLegacyLocalHostWorker(costCmd)
	markLegacyLocalHostWorker(setupCmd)
	markCompatibilityShim(sidecarCmd)
	markCompatibilityShim(sessionCmd)
	markCompatibilityShim(ensureCmd)

	parent.AddCommand(
		startCmd,
		sidecarCmd,
		stopCmd,
		statusCmd,
		sessionCmd,
		logsCmd,
		newWorkerMCPServerCmd(),
		restartCmd,
		historyCmd,
		costCmd,
		setupCmd,
		ensureCmd,
	)
}

func markCompatibilityShim(cmd *cobra.Command) {
	cmd.Hidden = true
}

func newWorkerStartCmd() *cobra.Command {
	var daemonFlag bool
	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the legacy local-host worker",
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
		Short: "Stop the legacy local-host worker daemon",
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
	var format string
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show legacy local-host worker diagnostics",
		Long:  "Compatibility surface for legacy local-host worker diagnostics. For installed desktop operation, use the desktop app status action or `autopus-desktop-runtime desktop status --json` for the canonical desktop runtime readiness contract.",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, err := resolveJSONMode(jsonOutput, format)
			if err != nil {
				return err
			}

			if jsonMode {
				statusPayload := setup.CollectStatus()
				warnings := buildWorkerStatusWarnings(statusPayload)
				status := jsonStatusOK
				if len(warnings) > 0 {
					status = jsonStatusWarn
				}
				return writeJSONResult(cmd, status, statusPayload, warnings, nil)
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
	addJSONFlags(cmd, &jsonOutput, &format)
	return cmd
}

func newWorkerLogsCmd() *cobra.Command {
	var taskFilter string
	cmd := &cobra.Command{
		Use:   "logs",
		Short: "Tail legacy local-host worker logs",
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
		Short: "Restart the legacy local-host worker daemon",
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
		Short: "Show legacy local-host worker task history",
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
		Short: "Show legacy local-host worker cost summary",
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
		Short: "Run legacy local-host worker setup",
		Long: `Legacy local-host worker setup for compatibility/dev-only use.

Prefer the canonical desktop/runtime flow:
  1. Desktop app Connect action or autopus-desktop-runtime connect
  2. Desktop app status action or autopus-desktop-runtime desktop status --json
  3. Desktop app session/bootstrap action or autopus-desktop-runtime desktop session

Use this command only when you explicitly need the legacy local-host worker path.

Compatibility mode still guides the 3-step worker setup process:
  1. Autopus 서버 인증 (브라우저에서 로그인)
  2. 워크스페이스 선택
  3. AI 프로바이더 확인 (Claude, Codex, Gemini)

비대화형 모드 (에이전트/CI 환경):
  auto worker setup --token <jwt> --workspace <workspace-id>
  auto worker setup --api-key <acos_worker_...> --workspace <workspace-id>  # legacy`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runWorkerSetup(cmd, backendURL, token, workspaceID, apiKey)
		},
	}
	// @AX:NOTE[AUTO]: magic constant — default backend URL "https://api.autopus.co" is hardcoded; must match production endpoint
	cmd.Flags().StringVar(&backendURL, "backend", "https://api.autopus.co", "Backend API URL")
	cmd.Flags().StringVar(&token, "token", "", "Pre-obtained JWT — preferred non-interactive auth")
	cmd.Flags().StringVar(&workspaceID, "workspace", "", "Workspace ID — skips interactive selection")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "Legacy Worker API Key (acos_worker_...) — backward compatibility only")
	return cmd
}

// installDaemon installs the worker as a system daemon based on OS.
func installDaemon() error {
	pathInfo, err := resolveCurrentBinaryPath()
	if err != nil {
		return err
	}

	cfg := daemon.LaunchdConfig{
		BinaryPath: pathInfo.ManagedPath(),
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
