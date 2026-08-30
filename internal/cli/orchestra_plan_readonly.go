package cli

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

// applyPlanReadOnlyProviderPolicy returns provider configs whose native argv
// enforce planning-only execution. Caller-owned configs and slices are not mutated.
func applyPlanReadOnlyProviderPolicy(providers []orchestra.ProviderConfig) ([]orchestra.ProviderConfig, error) {
	projected := make([]orchestra.ProviderConfig, len(providers))
	for index, provider := range providers {
		if err := validatePlanProviderArgs(provider); err != nil {
			return nil, err
		}

		provider.Args = append([]string(nil), provider.Args...)
		provider.PaneArgs = append([]string(nil), provider.PaneArgs...)
		provider.WorkingPatterns = append([]string(nil), provider.WorkingPatterns...)
		provider.ResultReadyPatterns = append([]string(nil), provider.ResultReadyPatterns...)
		provider.FastFailPatterns = append([]orchestra.FastFailRule(nil), provider.FastFailPatterns...)

		switch provider.Name {
		case "claude":
			provider.Args = projectClaudePlanArgs(provider.Args)
			provider.PaneArgs = projectClaudePlanArgs(provider.PaneArgs)
		case "codex":
			provider.Args = projectCodexPlanArgs(provider.Args)
			provider.PaneArgs = projectCodexPlanArgs(provider.PaneArgs)
		case "gemini":
			provider.Args = projectGeminiPlanArgs(provider.Args)
			provider.PaneArgs = projectGeminiPlanArgs(provider.PaneArgs)
		default:
			return nil, fmt.Errorf("plan read-only policy: unsupported provider %q", provider.Name)
		}
		projected[index] = provider
	}
	return projected, nil
}

func validatePlanProviderArgs(provider orchestra.ProviderConfig) error {
	expectedBinary := map[string]string{"claude": "claude", "codex": "codex", "gemini": "agy"}
	binary, known := expectedBinary[provider.Name]
	if !known {
		return fmt.Errorf("plan read-only policy: unsupported provider %q", provider.Name)
	}
	if provider.Binary != binary {
		return fmt.Errorf("plan read-only policy: provider %q requires native binary %q", provider.Name, binary)
	}
	for _, args := range [][]string{provider.Args, provider.PaneArgs} {
		if err := validatePlanProviderArgv(provider.Name, args); err != nil {
			return err
		}
	}
	return nil
}

func validatePlanProviderArgv(provider string, args []string) error {
	for index := 0; index < len(args); index++ {
		arg := args[index]
		if dangerousPlanArg(arg) {
			return fmt.Errorf("plan read-only policy: provider %q contains unsafe argv %q", provider, arg)
		}
		if planProviderBoolArg(provider, arg) {
			continue
		}
		flag, inlineValue, inline := strings.Cut(arg, "=")
		if !planProviderValueArg(provider, flag) {
			return fmt.Errorf("plan read-only policy: provider %q contains unsupported argv %q", provider, arg)
		}
		value := inlineValue
		if !inline {
			if index+1 >= len(args) {
				return fmt.Errorf("plan read-only policy: provider %q has incomplete argv %q", provider, arg)
			}
			index++
			value = args[index]
		}
		if !validPlanProviderArgValue(provider, flag, value) {
			return fmt.Errorf("plan read-only policy: provider %q contains unsafe value for %q", provider, flag)
		}
	}
	return nil
}

func planProviderBoolArg(provider, arg string) bool {
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

func planProviderValueArg(provider, flag string) bool {
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

func validPlanProviderArgValue(provider, flag, value string) bool {
	if strings.ContainsAny(value, "\x00\r\n") || dangerousPlanArg(value) {
		return false
	}
	if provider == "codex" && (flag == "-c" || flag == "--config") {
		return strings.HasPrefix(value, "model_reasoning_effort=")
	}
	return true
}

func projectClaudePlanArgs(args []string) []string {
	args = upsertPlanArgValue(args, "--permission-mode", "plan")
	for _, flag := range []string{"--safe-mode", "--no-session-persistence", "--disable-slash-commands"} {
		args = ensurePlanBoolArg(args, flag)
	}
	return args
}

func projectCodexPlanArgs(args []string) []string {
	args = upsertPlanArgValue(args, "--sandbox", "read-only")
	for _, flag := range []string{"--ephemeral", "--ignore-user-config", "--ignore-rules"} {
		args = ensurePlanBoolArg(args, flag)
	}
	return args
}

func projectGeminiPlanArgs(args []string) []string {
	args = upsertPlanArgValue(args, "--mode", "plan")
	args = ensurePlanBoolArg(args, "--sandbox")
	return ensurePlanBoolArg(args, "--disable-slash-commands")
}

func dangerousPlanArg(arg string) bool {
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

func upsertPlanArgValue(args []string, flag, value string) []string {
	result := make([]string, 0, len(args)+2)
	found := false
	for index := 0; index < len(args); index++ {
		if args[index] == "--" {
			if !found {
				result = append(result, flag, value)
			}
			return append(result, args[index:]...)
		}
		switch {
		case args[index] == flag:
			if !found {
				result = append(result, flag, value)
				found = true
			}
			if index+1 < len(args) && !strings.HasPrefix(args[index+1], "-") {
				index++
			}
		case strings.HasPrefix(args[index], flag+"="):
			if !found {
				result = append(result, flag+"="+value)
				found = true
			}
		default:
			result = append(result, args[index])
		}
	}
	if !found {
		result = append(result, flag, value)
	}
	return result
}

func ensurePlanBoolArg(args []string, flag string) []string {
	result := make([]string, 0, len(args)+1)
	found := false
	for index, arg := range args {
		if arg == "--" {
			if !found {
				result = append(result, flag)
			}
			return append(result, args[index:]...)
		}
		if arg == flag || strings.HasPrefix(arg, flag+"=") {
			if !found {
				result = append(result, flag)
				found = true
			}
			continue
		}
		result = append(result, arg)
	}
	if !found {
		result = append(result, flag)
	}
	return result
}
