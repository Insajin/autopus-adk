package journey

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var unsafeShellTokens = regexp.MustCompile(`[;&|<>$` + "`" + `]|\|\||&&|\r|\n`)

func Validate(pack Pack, projectDir string) error {
	if strings.TrimSpace(pack.ID) == "" {
		return validationError("qa_journey_invalid", "missing journey id")
	}
	if strings.TrimSpace(pack.Adapter.ID) == "" {
		return validationError("qa_journey_invalid", "missing adapter id")
	}
	if len(pack.Lanes) == 0 {
		return validationError("qa_journey_invalid", "missing lanes")
	}
	if len(pack.Checks) == 0 {
		return validationError("qa_journey_invalid", "missing checks")
	}
	return ValidateCommand(pack.Adapter.ID, pack.Command, pack.Artifacts, projectDir, "qa_journey")
}

func ValidateCompiledCommand(adapterID string, command Command, artifacts []Artifact, projectDir string) error {
	return ValidateCommand(adapterID, command, artifacts, projectDir, "qa_compiler")
}

func ValidateCommand(adapterID string, command Command, artifacts []Artifact, projectDir, prefix string) error {
	if strings.TrimSpace(command.CWD) == "" {
		command.CWD = "."
	}
	if err := validateCWD(command.CWD, projectDir); err != nil {
		return validationError(prefix+"_cwd_outside_project", err.Error())
	}
	if err := validateTimeout(command.Timeout); err != nil {
		return validationError(prefix+"_timeout_invalid", err.Error())
	}
	if err := validateArtifacts(artifacts); err != nil {
		return validationError(prefix+"_artifact_path_invalid", err.Error())
	}
	if err := validateEnvAllowlist(command.EnvAllowlist); err != nil {
		return validationError(prefix+"_env_not_allowlisted", err.Error())
	}
	if err := validateCommandShape(adapterID, command); err != nil {
		return validationError(prefix+"_command_unsafe", err.Error())
	}
	return nil
}

func validateCommandShape(adapterID string, command Command) error {
	adapterID = strings.TrimSpace(adapterID)
	if adapterID == "custom-command" {
		if len(command.Argv) == 0 {
			return fmt.Errorf("custom-command requires command.argv")
		}
		return validateArgv(command.Argv)
	}
	argv := command.Argv
	if len(argv) == 0 && strings.TrimSpace(command.Run) != "" {
		if unsafeShellTokens.MatchString(command.Run) {
			return fmt.Errorf("command.run contains shell metacharacters")
		}
		argv = strings.Fields(command.Run)
	}
	if len(argv) == 0 {
		return nil
	}
	if err := validateArgv(argv); err != nil {
		return err
	}
	return validateAdapterArgv(adapterID, argv)
}

func validateAdapterArgv(adapterID string, argv []string) error {
	switch adapterID {
	case "go-test":
		if len(argv) < 2 || !executableIs(argv[0], "go") || argv[1] != "test" {
			return fmt.Errorf("go-test command must start with go test")
		}
	case "node-script":
		return validateNodeScriptArgv(argv)
	case "vitest":
		return validateJSRunnerArgv(argv, "vitest")
	case "jest":
		return validateJSRunnerArgv(argv, "jest")
	case "playwright":
		return validateJSRunnerArgv(argv, "playwright", "test")
	case "pytest":
		if executableIs(argv[0], "pytest") {
			return nil
		}
		if len(argv) >= 3 && executableIs(argv[0], "python") && argv[1] == "-m" && argv[2] == "pytest" {
			return nil
		}
		return fmt.Errorf("pytest command must use pytest or python -m pytest")
	case "cargo-test":
		if len(argv) < 2 || !executableIs(argv[0], "cargo") || argv[1] != "test" {
			return fmt.Errorf("cargo-test command must start with cargo test")
		}
	case "auto-test-run":
		if len(argv) < 3 || !executableIs(argv[0], "auto") || argv[1] != "test" || argv[2] != "run" {
			return fmt.Errorf("auto-test-run command must start with auto test run")
		}
	case "auto-verify":
		if len(argv) < 2 || !executableIs(argv[0], "auto") || argv[1] != "verify" {
			return fmt.Errorf("auto-verify command must start with auto verify")
		}
	case "canary-template":
		if len(argv) < 2 || !executableIs(argv[0], "auto") || argv[1] != "canary" {
			return fmt.Errorf("canary-template executable command must start with auto canary")
		}
	default:
		return fmt.Errorf("unknown adapter %q", adapterID)
	}
	return nil
}

func validateArgv(argv []string) error {
	if len(argv) > 0 {
		if containsShellInvocation(argv) {
			return fmt.Errorf("command.argv may not invoke a shell")
		}
	}
	for _, arg := range argv {
		if strings.TrimSpace(arg) == "" {
			return fmt.Errorf("command.argv contains empty value")
		}
		if unsafeShellTokens.MatchString(arg) {
			return fmt.Errorf("command.argv contains shell metacharacters")
		}
	}
	return nil
}

func validateNodeScriptArgv(argv []string) error {
	if !executableIs(argv[0], "npm", "pnpm", "yarn") {
		return fmt.Errorf("node-script command must use npm, pnpm, or yarn")
	}
	if len(argv) >= 2 && argv[1] == "test" {
		return nil
	}
	if len(argv) >= 3 && argv[1] == "run" && strings.TrimSpace(argv[2]) != "" {
		return nil
	}
	return fmt.Errorf("node-script command must run an explicit package script")
}

func validateJSRunnerArgv(argv []string, runner string, requiredTail ...string) error {
	if len(argv) == 0 {
		return fmt.Errorf("%s command is empty", runner)
	}
	binary := argv[0]
	position := 0
	switch binary {
	case runner:
	case "npx", "pnpm", "yarn":
		if len(argv) < 2 || argv[1] != runner {
			return fmt.Errorf("%s command must invoke %s", runner, runner)
		}
		position = 1
	case "npm":
		if len(argv) < 3 || argv[1] != "exec" || argv[2] != runner {
			return fmt.Errorf("%s command must use npm exec %s", runner, runner)
		}
		position = 2
	default:
		return fmt.Errorf("%s command must invoke %s", runner, runner)
	}
	for offset, expected := range requiredTail {
		index := position + 1 + offset
		if len(argv) <= index || argv[index] != expected {
			return fmt.Errorf("%s command must include %s", runner, expected)
		}
	}
	return nil
}

func containsShellInvocation(argv []string) bool {
	for _, arg := range argv {
		base := filepath.Base(arg)
		switch base {
		case "sh", "bash", "zsh", "fish", "cmd", "powershell", "pwsh", "env":
			return true
		}
	}
	return false
}

func executableIs(value string, allowed ...string) bool {
	for _, name := range allowed {
		if value == name {
			return true
		}
	}
	return false
}

func validateCWD(cwd, projectDir string) error {
	if filepath.IsAbs(cwd) {
		return fmt.Errorf("cwd must be relative to project root")
	}
	clean := filepath.Clean(cwd)
	if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return fmt.Errorf("cwd outside project root")
	}
	root, err := filepath.Abs(projectDir)
	if err != nil {
		return err
	}
	target := filepath.Join(root, clean)
	rootReal, err := filepath.EvalSymlinks(root)
	if err != nil {
		return err
	}
	targetReal, err := filepath.EvalSymlinks(target)
	if err != nil {
		if os.IsNotExist(err) && clean == "." {
			return nil
		}
		return err
	}
	if !isWithin(rootReal, targetReal) {
		return fmt.Errorf("cwd outside project root")
	}
	return nil
}

func isWithin(root, target string) bool {
	rel, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

func validateTimeout(value string) error {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil || duration <= 0 || duration > 30*time.Minute {
		return fmt.Errorf("timeout must be between 1ns and 30m")
	}
	return nil
}

func validateArtifacts(artifacts []Artifact) error {
	for _, artifact := range artifacts {
		for _, value := range []string{artifact.Path, artifact.Root} {
			if value == "" {
				continue
			}
			if filepath.IsAbs(value) {
				return fmt.Errorf("artifact path must be relative")
			}
			clean := filepath.Clean(value)
			if clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return fmt.Errorf("artifact path outside allowed roots")
			}
		}
	}
	return nil
}

func validateEnvAllowlist(values []string) error {
	for _, value := range values {
		if strings.ContainsAny(value, "=$ \t\r\n") {
			return fmt.Errorf("env allowlist entries must be variable names")
		}
	}
	return nil
}

func validationError(code, message string) error {
	return &ValidationError{Code: code, Message: message}
}
