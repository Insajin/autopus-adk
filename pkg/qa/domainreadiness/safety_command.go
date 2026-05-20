package domainreadiness

import (
	"path/filepath"
	"strings"
)

func unsafeEnvAllowlist(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" || strings.ContainsAny(value, "=$ \t\r\n") {
		return true
	}
	lower := strings.ToLower(value)
	for _, fragment := range []string{"secret", "token", "password", "credential", "cookie", "authorization", "api_key", "apikey"} {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return false
}

func containsUnsafeArgv(argv []string) bool {
	for _, arg := range argv {
		if strings.TrimSpace(arg) == "" || shellMeta.MatchString(arg) {
			return true
		}
		switch filepath.Base(arg) {
		case "sh", "bash", "zsh", "fish", "cmd", "powershell", "pwsh", "env":
			return true
		}
	}
	return false
}

func knownAdapterCommand(adapter string, argv []string) bool {
	if len(argv) == 0 {
		return true
	}
	switch strings.TrimSpace(adapter) {
	case "go-test":
		return len(argv) >= 2 && argv[0] == "go" && argv[1] == "test"
	case "node-script":
		if argv[0] != "npm" && argv[0] != "pnpm" && argv[0] != "yarn" {
			return false
		}
		return len(argv) >= 2 && argv[1] == "test" || len(argv) >= 3 && argv[1] == "run"
	case "playwright", "gui-explore":
		return jsRunner(argv, "playwright")
	case "vitest":
		return jsRunner(argv, "vitest")
	case "jest":
		return jsRunner(argv, "jest")
	case "pytest":
		return argv[0] == "pytest" || len(argv) >= 3 && argv[0] == "python" && argv[1] == "-m" && argv[2] == "pytest"
	case "cargo-test":
		return len(argv) >= 2 && argv[0] == "cargo" && argv[1] == "test"
	case "auto-test-run":
		return len(argv) >= 3 && argv[0] == "auto" && argv[1] == "test" && argv[2] == "run"
	case "auto-verify":
		return len(argv) >= 2 && argv[0] == "auto" && argv[1] == "verify"
	case "canary-template":
		return len(argv) >= 2 && argv[0] == "auto" && argv[1] == "canary"
	case "custom-command":
		return knownExecutable(argv[0])
	default:
		return false
	}
}

func jsRunner(argv []string, runner string) bool {
	if len(argv) == 0 {
		return false
	}
	if argv[0] == runner {
		return true
	}
	if len(argv) >= 2 && (argv[0] == "npx" || argv[0] == "pnpm" || argv[0] == "yarn") && argv[1] == runner {
		return true
	}
	return len(argv) >= 3 && argv[0] == "npm" && argv[1] == "exec" && argv[2] == runner
}

func knownExecutable(value string) bool {
	switch value {
	case "go", "npm", "pnpm", "yarn", "npx", "pytest", "python", "cargo", "auto", "maestro", "appium":
		return true
	default:
		return false
	}
}
