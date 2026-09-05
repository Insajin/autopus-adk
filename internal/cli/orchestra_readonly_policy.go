package cli

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/orchestra"
)

// readOnlyPolicyOptions tunes the read-only projection for the execution environment.
type readOnlyPolicyOptions struct {
	// OutsideRepo marks that providers start from an isolated non-repository
	// directory; Codex exec refuses such a cwd without --skip-git-repo-check.
	OutsideRepo bool
}

// readOnlyOrchestraCommand reports whether a command must run its providers
// read-only: planning and brainstorming produce advisory text only, so no
// provider may touch the caller's worktree (issue #108).
func readOnlyOrchestraCommand(commandName string) bool {
	return commandName == "plan" || commandName == "brainstorm"
}

// applyCommandReadOnlyPolicy projects provider argv for read-only commands and
// returns other commands' providers untouched.
func applyCommandReadOnlyPolicy(commandName string, providers []orchestra.ProviderConfig, opts readOnlyPolicyOptions) ([]orchestra.ProviderConfig, error) {
	if !readOnlyOrchestraCommand(commandName) {
		return providers, nil
	}
	return applyReadOnlyProviderPolicy(providers, opts)
}

// applyReadOnlyProviderPolicy returns provider configs whose native argv
// enforce read-only execution and whose SandboxMode records that evidence.
// OMP-backed providers are accepted as-is: that backend only exposes the
// read/grep/glob tool allowlist and fails closed at session start otherwise.
// Caller-owned configs and slices are not mutated.
func applyReadOnlyProviderPolicy(providers []orchestra.ProviderConfig, opts readOnlyPolicyOptions) ([]orchestra.ProviderConfig, error) {
	projected := make([]orchestra.ProviderConfig, len(providers))
	for index, provider := range providers {
		if provider.Backend == config.ProviderBackendOMP {
			provider.SandboxMode = orchestra.SandboxModeReadOnly
			projected[index] = provider
			continue
		}
		if err := validateReadOnlyProviderArgs(provider); err != nil {
			return nil, err
		}

		provider.Args = append([]string(nil), provider.Args...)
		provider.PaneArgs = append([]string(nil), provider.PaneArgs...)
		provider.WorkingPatterns = append([]string(nil), provider.WorkingPatterns...)
		provider.ResultReadyPatterns = append([]string(nil), provider.ResultReadyPatterns...)
		provider.FastFailPatterns = append([]orchestra.FastFailRule(nil), provider.FastFailPatterns...)

		switch provider.Name {
		case "claude":
			provider.Args = projectClaudeReadOnlyArgs(provider.Args)
			provider.PaneArgs = projectClaudeReadOnlyArgs(provider.PaneArgs)
		case "codex":
			provider.Args = projectCodexReadOnlyArgs(provider.Args, opts.OutsideRepo)
			provider.PaneArgs = projectCodexReadOnlyArgs(provider.PaneArgs, false)
		case "gemini":
			provider.Args = projectGeminiReadOnlyArgs(provider.Args)
			provider.PaneArgs = projectGeminiReadOnlyArgs(provider.PaneArgs)
		default:
			return nil, fmt.Errorf("read-only provider policy: unsupported provider %q", provider.Name)
		}
		provider.SandboxMode = orchestra.SandboxModeReadOnly
		projected[index] = provider
	}
	return projected, nil
}

func validateReadOnlyProviderArgs(provider orchestra.ProviderConfig) error {
	expectedBinary := map[string]string{"claude": "claude", "codex": "codex", "gemini": "agy"}
	binary, known := expectedBinary[provider.Name]
	if !known {
		return fmt.Errorf("read-only provider policy: unsupported provider %q", provider.Name)
	}
	if provider.Binary != binary {
		return fmt.Errorf("read-only provider policy: provider %q requires native binary %q", provider.Name, binary)
	}
	for _, args := range [][]string{provider.Args, provider.PaneArgs} {
		if err := validateReadOnlyProviderArgv(provider.Name, args); err != nil {
			return err
		}
	}
	return nil
}

func validateReadOnlyProviderArgv(provider string, args []string) error {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if dangerousProviderArg(arg) {
			return fmt.Errorf("read-only provider policy: provider %q contains unsafe argv %q", provider, arg)
		}
		if readOnlyProviderBoolArg(provider, arg) {
			continue
		}
		flag, inlineValue, inline := strings.Cut(arg, "=")
		if !readOnlyProviderValueArg(provider, flag) {
			return fmt.Errorf("read-only provider policy: provider %q contains unsupported argv %q", provider, arg)
		}
		value := inlineValue
		if !inline {
			if index+1 >= len(args) {
				return fmt.Errorf("read-only provider policy: provider %q has incomplete argv %q", provider, arg)
			}
			index++
			value = args[index]
		}
		if !validReadOnlyProviderArgValue(provider, flag, value) {
			return fmt.Errorf("read-only provider policy: provider %q contains unsafe value for %q", provider, flag)
		}
	}
	return nil
}

func readOnlyProviderBoolArg(provider, arg string) bool {
	switch provider {
	case "claude":
		switch arg {
		case "--print", "-p", "--no-session-persistence", "--disable-slash-commands", "--safe-mode":
			return true
		}
	case "codex":
		switch arg {
		case "exec", "--json", "--ephemeral", "--ignore-user-config", "--ignore-rules", "--skip-git-repo-check":
			return true
		}
	case "gemini":
		return arg == "--sandbox" || arg == "--disable-slash-commands"
	}
	return false
}

func readOnlyProviderValueArg(provider, flag string) bool {
	switch provider {
	case "claude":
		switch flag {
		case "--model", "--effort", "--output-format", "--max-budget-usd", "--permission-mode":
			return true
		}
	case "codex":
		switch flag {
		case "--sandbox", "-s", "--model", "-m", "--color", "--config", "-c":
			return true
		}
	case "gemini":
		switch flag {
		case "--print", "-p", "--model", "--effort", "--mode", "--output-format", "--json-schema", "--print-timeout":
			return true
		}
	}
	return false
}

func validReadOnlyProviderArgValue(provider, flag, value string) bool {
	if strings.ContainsAny(value, "\x00\r\n") || dangerousProviderArg(value) {
		return false
	}
	if provider == "codex" && (flag == "-c" || flag == "--config") {
		return strings.HasPrefix(value, "model_reasoning_effort=")
	}
	return true
}

func projectClaudeReadOnlyArgs(args []string) []string {
	args = upsertArgValue(args, "--permission-mode", "plan")
	for _, flag := range []string{"--safe-mode", "--no-session-persistence", "--disable-slash-commands"} {
		args = ensureBoolArg(args, flag)
	}
	return args
}

func projectCodexReadOnlyArgs(args []string, outsideRepo bool) []string {
	args = upsertArgValue(args, "--sandbox", "read-only")
	for _, flag := range []string{"--ephemeral", "--ignore-user-config", "--ignore-rules"} {
		args = ensureBoolArg(args, flag)
	}
	if outsideRepo {
		args = ensureBoolArg(args, "--skip-git-repo-check")
	}
	return args
}

func projectGeminiReadOnlyArgs(args []string) []string {
	args = upsertArgValue(args, "--mode", "plan")
	args = ensureBoolArg(args, "--sandbox")
	return ensureBoolArg(args, "--disable-slash-commands")
}

func dangerousProviderArg(arg string) bool {
	arg = strings.ToLower(strings.TrimSpace(arg))
	if strings.Contains(arg, "danger") || strings.Contains(arg, "bypass") || strings.Contains(arg, "yolo") {
		return true
	}
	switch arg {
	case "-y", "--full-auto", "--no-sandbox", "--sandbox=false", "--sandbox=0":
		return true
	default:
		return false
	}
}
