package journey

import "fmt"

func validateJSRunnerArgv(argv []string, runner string, requiredTail ...string) error {
	if len(argv) == 0 {
		return fmt.Errorf("%s command is empty", runner)
	}
	position := 0
	switch argv[0] {
	case runner:
	case "npx", "yarn":
		if len(argv) < 2 || argv[1] != runner {
			return fmt.Errorf("%s command must invoke %s", runner, runner)
		}
		position = 1
	case "pnpm":
		if len(argv) >= 3 && argv[1] == "exec" && argv[2] == runner {
			position = 2
		} else if len(argv) >= 2 && argv[1] == runner {
			position = 1
		} else {
			return fmt.Errorf("%s command must invoke %s", runner, runner)
		}
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
