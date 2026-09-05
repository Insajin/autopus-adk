package orchestra

import (
	"os"
	"strings"
	"time"
)

// Sandbox mode vocabulary recorded on provider execution evidence.
const (
	SandboxModeReadOnly       = "read-only"
	SandboxModeWorkspaceWrite = "workspace-write"
	SandboxModeUnrestricted   = "unrestricted"
)

// ProviderExecution records how one provider process was launched. It is the
// provenance carrier behind the command/cwd/pid/sandbox fields of
// ProviderRunReceipt so a receipt can prove where and how a provider ran.
type ProviderExecution struct {
	Command     []string  `json:"command"`                // argv including the binary
	Cwd         string    `json:"cwd,omitempty"`          // effective process working directory
	PID         int       `json:"pid,omitempty"`          // started process ID; 0 when the process never started
	SandboxMode string    `json:"sandbox_mode,omitempty"` // read-only, workspace-write, or unrestricted
	StartedAt   time.Time `json:"started_at"`
	EndedAt     time.Time `json:"ended_at"`
}

// newProviderExecution captures launch provenance before the process starts.
// The cwd is the explicit provider WorkDir or, when empty, the inherited
// orchestrator cwd so the receipt always names the directory that ran.
func newProviderExecution(provider ProviderConfig, args []string, start time.Time) *ProviderExecution {
	command := make([]string, 0, len(args)+1)
	command = append(command, provider.Binary)
	command = append(command, args...)
	cwd := strings.TrimSpace(provider.WorkDir)
	if cwd == "" {
		cwd, _ = os.Getwd()
	}
	return &ProviderExecution{
		Command:     command,
		Cwd:         cwd,
		SandboxMode: providerSandboxMode(provider, args),
		StartedAt:   start,
	}
}

// finish stamps the process end time derived from the measured duration so the
// receipt window matches the response Duration exactly.
func (e *ProviderExecution) finish(duration time.Duration) {
	if e == nil {
		return
	}
	e.EndedAt = e.StartedAt.Add(duration)
}

// providerSandboxMode reports the sandbox mode a provider runs under: the
// policy-stamped mode wins; otherwise the native argv is inspected. Codex
// declares its sandbox explicitly, a Claude plan permission mode is read-only,
// and a permission/sandbox bypass flag means unrestricted. Without any
// restriction the orchestrator imposes nothing, which is recorded as
// unrestricted rather than guessed.
func providerSandboxMode(provider ProviderConfig, args []string) string {
	if mode := strings.TrimSpace(provider.SandboxMode); mode != "" {
		return mode
	}
	for index, arg := range args {
		switch {
		case arg == "--sandbox" && index+1 < len(args) && !strings.HasPrefix(args[index+1], "-"):
			return args[index+1]
		case strings.HasPrefix(arg, "--sandbox=") && len(arg) > len("--sandbox="):
			return strings.TrimPrefix(arg, "--sandbox=")
		case arg == "--permission-mode" && index+1 < len(args) && args[index+1] == "plan",
			arg == "--permission-mode=plan":
			return SandboxModeReadOnly
		case arg == "--dangerously-skip-permissions", arg == "--dangerously-bypass-approvals-and-sandbox", arg == "--yolo":
			return SandboxModeUnrestricted
		}
	}
	return SandboxModeUnrestricted
}

// resolveProviderWorkDir applies the run-level provider working directory to
// a provider that does not pin its own, so every subprocess dispatch site
// shares one cwd policy.
func resolveProviderWorkDir(cfg OrchestraConfig, provider ProviderConfig) ProviderConfig {
	if provider.WorkDir == "" {
		provider.WorkDir = cfg.ProviderWorkDir
	}
	return provider
}

// providerLaunchDir resolves where pane shells run: the isolated provider
// directory when set, otherwise the artifact working directory.
func (cfg OrchestraConfig) providerLaunchDir() string {
	if cfg.ProviderWorkDir != "" {
		return cfg.ProviderWorkDir
	}
	return cfg.WorkingDir
}
