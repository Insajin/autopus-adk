package setup

import (
	"fmt"
	"strings"
)

func renderIndex(info *ProjectInfo) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# %s\n\n", info.Name))

	b.WriteString("## Tech Stack\n\n")
	for _, language := range info.Languages {
		if language.Version != "" {
			b.WriteString(fmt.Sprintf("- **%s** %s\n", language.Name, language.Version))
		} else {
			b.WriteString(fmt.Sprintf("- **%s**\n", language.Name))
		}
	}
	for _, framework := range info.Frameworks {
		b.WriteString(fmt.Sprintf("- **%s** %s\n", framework.Name, framework.Version))
	}
	b.WriteString("\n")

	b.WriteString("## Directory Overview\n\n```\n")
	for _, entry := range info.Structure {
		writeTreeEntry(&b, entry, 0)
	}
	b.WriteString("```\n\n")

	if len(info.EntryPoints) > 0 {
		b.WriteString("## Key Entry Points\n\n")
		for _, entryPoint := range info.EntryPoints {
			b.WriteString(fmt.Sprintf("- `%s` — %s\n", entryPoint.Path, entryPoint.Description))
		}
		b.WriteString("\n")
	}

	if len(info.Workspaces) > 0 {
		b.WriteString("## Workspaces\n\n")
		fmt.Fprintf(&b, "This is a **monorepo** with %d workspaces (%s):\n\n", len(info.Workspaces), info.Workspaces[0].Type)
		for _, workspace := range info.Workspaces {
			fmt.Fprintf(&b, "- `%s/` — %s\n", workspace.Path, workspace.Name)
		}
		b.WriteString("\n")
	}

	if info.MultiRepo != nil && info.MultiRepo.IsMultiRepo {
		b.WriteString("## Repositories\n\n")
		fmt.Fprintf(&b, "This is a **multi-repo** workspace with %d repositories:\n\n", len(info.MultiRepo.Components))
		for _, component := range info.MultiRepo.Components {
			fmt.Fprintf(&b, "- `%s/` — %s (%s)\n", component.Path, component.Name, component.Role)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Documentation\n\n")
	b.WriteString("- [Commands](commands.md) — Build, test, lint commands\n")
	b.WriteString("- [Structure](structure.md) — Directory structure and roles\n")
	b.WriteString("- [Conventions](conventions.md) — Code conventions with examples\n")
	b.WriteString("- [Boundaries](boundaries.md) — Constraints (Always / Ask / Never)\n")
	b.WriteString("- [Architecture](architecture.md) — Architecture decisions and rationale\n")
	b.WriteString("- [Testing](testing.md) — Test patterns and coverage\n")

	return truncateLines(b.String(), maxIndexLines)
}

func renderCommands(info *ProjectInfo) string {
	var b strings.Builder
	b.WriteString("# Commands\n\n")

	if len(info.BuildFiles) == 0 {
		b.WriteString("No build files detected.\n\n")
		b.WriteString("## Manual Setup\n\n")
		b.WriteString("Add your build commands here:\n\n")
		b.WriteString("```bash\n# Build\n# Test\n# Lint\n# Format\n```\n")
		return b.String()
	}

	for _, buildFile := range info.BuildFiles {
		b.WriteString(fmt.Sprintf("## %s (`%s`)\n\n", buildFile.Type, buildFile.Path))
		if len(buildFile.Commands) == 0 {
			b.WriteString("No commands extracted.\n\n")
			continue
		}

		categories := []struct {
			name string
			keys []string
		}{
			{"Build", []string{"build", "compile", "install"}},
			{"Test", []string{"test", "coverage", "e2e"}},
			{"Lint / Format", []string{"lint", "format", "fmt", "check", "vet", "clippy"}},
			{"Run", []string{"run", "dev", "start", "serve", "up"}},
			{"Clean / Deploy", []string{"clean", "deploy", "down", "docker"}},
		}

		used := make(map[string]bool)
		for _, category := range categories {
			var commands []string
			for _, key := range category.keys {
				if command, ok := buildFile.Commands[key]; ok {
					commands = append(commands, fmt.Sprintf("```bash\n%s\n```\n", command))
					used[key] = true
				}
			}
			if len(commands) == 0 {
				continue
			}
			b.WriteString(fmt.Sprintf("### %s\n\n", category.name))
			for _, command := range commands {
				b.WriteString(command)
				b.WriteString("\n")
			}
		}

		var remaining []string
		for key, command := range buildFile.Commands {
			if !used[key] {
				remaining = append(remaining, fmt.Sprintf("- `%s`: `%s`", key, command))
			}
		}
		if len(remaining) == 0 {
			continue
		}
		b.WriteString("### Other\n\n")
		for _, entry := range remaining {
			b.WriteString(entry + "\n")
		}
		b.WriteString("\n")
	}

	return truncateLines(b.String(), maxDocLines)
}

func renderStructure(info *ProjectInfo) string {
	var b strings.Builder

	b.WriteString("# Project Structure\n\n```\n")
	b.WriteString(info.Name + "/\n")
	for _, entry := range info.Structure {
		writeDetailedTreeEntry(&b, entry, 1)
	}
	b.WriteString("```\n\n")

	b.WriteString("## Directory Roles\n\n")
	for _, entry := range info.Structure {
		if entry.Description != "" {
			b.WriteString(fmt.Sprintf("- **%s/** — %s\n", entry.Name, entry.Description))
		}
		for _, child := range entry.Children {
			if child.Description != "" {
				b.WriteString(fmt.Sprintf("  - **%s/** — %s\n", child.Name, child.Description))
			}
		}
	}

	if info.MultiRepo != nil && info.MultiRepo.IsMultiRepo {
		b.WriteString("\n")
		b.WriteString(renderRepoBoundaries(info.MultiRepo))
	}

	return truncateLines(b.String(), maxDocLines)
}
