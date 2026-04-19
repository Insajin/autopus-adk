package setup

import (
	"bufio"
	"os"
	"strings"
)

// parseMakefileTargets extracts targets and their first meaningful recipe.
func parseMakefileTargets(path string) map[string]string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	targets := make(map[string]string)
	phonyTargets := make(map[string]bool)

	scanner := bufio.NewScanner(file)
	var currentTarget string
	var currentRecipe []string

	flushTarget := func() {
		if currentTarget != "" && len(currentRecipe) > 0 {
			recipe := strings.Join(currentRecipe, " && ")
			targets[currentTarget] = strings.TrimPrefix(recipe, "@")
		} else if currentTarget != "" {
			targets[currentTarget] = "make " + currentTarget
		}
		currentTarget = ""
		currentRecipe = nil
	}

	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "\t") {
			recipe := strings.TrimSpace(line)
			if currentTarget != "" && recipe != "" && !strings.HasPrefix(recipe, "#") {
				if !strings.HasPrefix(recipe, "@echo") && !strings.HasPrefix(recipe, "echo ") {
					currentRecipe = append(currentRecipe, strings.TrimPrefix(recipe, "@"))
				}
			}
			continue
		}

		flushTarget()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if strings.Contains(trimmed, "=") && !strings.Contains(trimmed, ":") {
			continue
		}
		if strings.HasPrefix(trimmed, ".PHONY:") {
			phonies := strings.TrimPrefix(trimmed, ".PHONY:")
			for _, phony := range strings.Fields(phonies) {
				phonyTargets[phony] = true
			}
			continue
		}
		if strings.HasPrefix(trimmed, ".") {
			continue
		}
		if strings.Contains(trimmed, ":") {
			parts := strings.SplitN(trimmed, ":", 2)
			name := strings.TrimSpace(parts[0])
			if name != "" && !strings.Contains(name, " ") && !strings.Contains(name, "/") && !strings.Contains(name, "$") {
				currentTarget = name
			}
		}
	}

	flushTarget()
	return targets
}

func parsePyprojectScripts(path string) map[string]string {
	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	commands := make(map[string]string)
	scanner := bufio.NewScanner(file)
	inScripts := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "[tool.poetry.scripts]" || line == "[project.scripts]" {
			inScripts = true
			continue
		}
		if strings.HasPrefix(line, "[") {
			inScripts = false
			continue
		}
		if inScripts && strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			name := strings.TrimSpace(parts[0])
			value := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			commands[name] = value
		}
	}

	return commands
}
