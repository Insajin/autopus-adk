package setup

import (
	"fmt"
	"strings"
)

func renderConventions(info *ProjectInfo) string {
	var b strings.Builder
	b.WriteString("# Code Conventions\n\n")

	for _, language := range info.Languages {
		fmt.Fprintf(&b, "## %s\n\n", language.Name)
		sample, hasSample := info.Conventions[language.Name]

		if hasSample && sample.FileNaming != "" {
			b.WriteString("### File Naming\n\n")
			fmt.Fprintf(&b, "- Detected pattern: **%s**\n", sample.FileNaming)
			if len(sample.ExampleFiles) > 0 {
				b.WriteString("- Examples: ")
				for i, file := range sample.ExampleFiles {
					if i > 0 {
						b.WriteString(", ")
					}
					fmt.Fprintf(&b, "`%s`", file)
				}
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}

		switch language.Name {
		case "Go":
			b.WriteString("### Naming\n\n")
			b.WriteString("- Packages: `lowercase` (single word preferred)\n")
			b.WriteString("- Exported: `PascalCase`\n")
			b.WriteString("- Unexported: `camelCase`\n\n")
			b.WriteString("### Error Handling\n\n")
			if hasSample && len(sample.ErrorPatterns) > 0 {
				b.WriteString("Detected patterns in this project:\n\n")
				for _, pattern := range sample.ErrorPatterns {
					fmt.Fprintf(&b, "- %s\n", pattern)
				}
				b.WriteString("\n")
			} else {
				b.WriteString("```go\nif err != nil {\n    return fmt.Errorf(\"context: %w\", err)\n}\n```\n\n")
			}
			if hasSample && sample.ImportStyle != "" && sample.ImportStyle != "unknown" {
				fmt.Fprintf(&b, "### Import Style\n\n- %s\n\n", sample.ImportStyle)
			}
			b.WriteString("### Project Layout\n\n")
			b.WriteString("- `cmd/` — CLI entry points\n")
			b.WriteString("- `pkg/` — Public reusable libraries\n")
			b.WriteString("- `internal/` — Private implementation\n\n")
		case "TypeScript":
			b.WriteString("### Naming\n\n")
			b.WriteString("- Types/Interfaces: `PascalCase`\n")
			b.WriteString("- Functions/Variables: `camelCase`\n")
			b.WriteString("- Constants: `UPPER_SNAKE_CASE`\n\n")
		case "Python":
			b.WriteString("### Naming\n\n")
			b.WriteString("- Classes: `PascalCase`\n")
			b.WriteString("- Functions/Variables: `snake_case`\n")
			b.WriteString("- Constants: `UPPER_SNAKE_CASE`\n\n")
		case "Rust":
			b.WriteString("### Naming\n\n")
			b.WriteString("- Types/Traits: `PascalCase`\n")
			b.WriteString("- Functions/Variables: `snake_case`\n")
			b.WriteString("- Constants: `UPPER_SNAKE_CASE`\n\n")
		}

		if hasSample && (sample.HasLinter || sample.HasFormatter) {
			b.WriteString("### Tooling\n\n")
			if sample.HasLinter {
				fmt.Fprintf(&b, "- **Linter:** %s\n", sample.LinterName)
			}
			if sample.HasFormatter {
				fmt.Fprintf(&b, "- **Formatter:** %s\n", sample.FormatterName)
			}
			b.WriteString("\n")
		}
	}

	return truncateLines(b.String(), maxDocLines)
}

func renderBoundaries(info *ProjectInfo) string {
	var b strings.Builder

	b.WriteString("# Boundaries\n\n")
	b.WriteString("Constraints categorized by autonomy level.\n\n")
	b.WriteString("## Always Do (Autonomous)\n\n")
	b.WriteString("Actions the agent can take without asking.\n\n")
	b.WriteString("- Run tests before committing\n")
	b.WriteString("- Format code according to project standards\n")
	b.WriteString("- Fix lint warnings\n")

	for _, language := range info.Languages {
		switch language.Name {
		case "Go":
			b.WriteString("- Run `go vet` and `go test -race` before commits\n")
		case "TypeScript", "JavaScript":
			b.WriteString("- Run `npm test` and `npm run lint` before commits\n")
		case "Python":
			b.WriteString("- Run `pytest` and `ruff check` before commits\n")
		}
	}

	b.WriteString("\n## Ask First (Requires Confirmation)\n\n")
	b.WriteString("Actions that need user approval.\n\n")
	b.WriteString("- Adding new dependencies\n")
	b.WriteString("- Changing public API signatures\n")
	b.WriteString("- Modifying CI/CD configuration\n")
	b.WriteString("- Database schema changes\n")
	b.WriteString("- Deleting files or directories\n")

	b.WriteString("\n## Never Do (Hard Stops)\n\n")
	b.WriteString("Actions that are always prohibited.\n\n")
	b.WriteString("- Commit secrets, API keys, or credentials\n")
	b.WriteString("- Force push to main/master branch\n")
	b.WriteString("- Skip tests (--no-verify)\n")
	b.WriteString("- Disable security checks\n")
	b.WriteString("- Remove error handling\n")

	return truncateLines(b.String(), maxDocLines)
}
