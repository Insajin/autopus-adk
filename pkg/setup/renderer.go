package setup

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/arch"
	"github.com/insajin/autopus-adk/pkg/lore"
)

const (
	maxIndexLines = 200
	maxDocLines   = 500
)

// RenderOptions holds optional data for rendering.
type RenderOptions struct {
	ArchMap   *arch.ArchitectureMap
	LoreItems []lore.LoreEntry
}

// Render generates all documentation files from ProjectInfo.
func Render(info *ProjectInfo, opts *RenderOptions) *DocSet {
	if opts == nil {
		opts = &RenderOptions{}
	}

	ds := &DocSet{
		Index:        renderIndex(info),
		Commands:     renderCommands(info),
		Structure:    renderStructure(info),
		Conventions:  renderConventions(info),
		Boundaries:   renderBoundaries(info),
		Architecture: renderArchitecture(info, opts),
		Testing:      renderTesting(info),
	}
	return ds
}

func renderIndex(info *ProjectInfo) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("# %s\n\n", info.Name))

	// Tech stack
	b.WriteString("## Tech Stack\n\n")
	if len(info.Languages) > 0 {
		for _, l := range info.Languages {
			if l.Version != "" {
				b.WriteString(fmt.Sprintf("- **%s** %s\n", l.Name, l.Version))
			} else {
				b.WriteString(fmt.Sprintf("- **%s**\n", l.Name))
			}
		}
	}
	if len(info.Frameworks) > 0 {
		for _, f := range info.Frameworks {
			b.WriteString(fmt.Sprintf("- **%s** %s\n", f.Name, f.Version))
		}
	}
	b.WriteString("\n")

	// Directory overview
	b.WriteString("## Directory Overview\n\n")
	b.WriteString("```\n")
	for _, d := range info.Structure {
		writeTreeEntry(&b, d, 0)
	}
	b.WriteString("```\n\n")

	// Entry points
	if len(info.EntryPoints) > 0 {
		b.WriteString("## Key Entry Points\n\n")
		for _, ep := range info.EntryPoints {
			b.WriteString(fmt.Sprintf("- `%s` — %s\n", ep.Path, ep.Description))
		}
		b.WriteString("\n")
	}

	// Workspaces (monorepo)
	if len(info.Workspaces) > 0 {
		b.WriteString("## Workspaces\n\n")
		fmt.Fprintf(&b, "This is a **monorepo** with %d workspaces (%s):\n\n",
			len(info.Workspaces), info.Workspaces[0].Type)
		for _, ws := range info.Workspaces {
			fmt.Fprintf(&b, "- `%s/` — %s\n", ws.Path, ws.Name)
		}
		b.WriteString("\n")
	}

	// Document links
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

	for _, bf := range info.BuildFiles {
		b.WriteString(fmt.Sprintf("## %s (`%s`)\n\n", bf.Type, bf.Path))

		if len(bf.Commands) == 0 {
			b.WriteString("No commands extracted.\n\n")
			continue
		}

		// Group by category
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
		for _, cat := range categories {
			var cmds []string
			for _, key := range cat.keys {
				if cmd, ok := bf.Commands[key]; ok {
					cmds = append(cmds, fmt.Sprintf("```bash\n%s\n```\n", cmd))
					used[key] = true
				}
			}
			if len(cmds) > 0 {
				b.WriteString(fmt.Sprintf("### %s\n\n", cat.name))
				for _, c := range cmds {
					b.WriteString(c)
					b.WriteString("\n")
				}
			}
		}

		// Remaining commands
		var remaining []string
		for key, cmd := range bf.Commands {
			if !used[key] {
				remaining = append(remaining, fmt.Sprintf("- `%s`: `%s`", key, cmd))
			}
		}
		if len(remaining) > 0 {
			b.WriteString("### Other\n\n")
			for _, r := range remaining {
				b.WriteString(r + "\n")
			}
			b.WriteString("\n")
		}
	}

	return truncateLines(b.String(), maxDocLines)
}

func renderStructure(info *ProjectInfo) string {
	var b strings.Builder

	b.WriteString("# Project Structure\n\n")
	b.WriteString("```\n")
	b.WriteString(info.Name + "/\n")
	for _, d := range info.Structure {
		writeDetailedTreeEntry(&b, d, 1)
	}
	b.WriteString("```\n\n")

	// Role descriptions
	b.WriteString("## Directory Roles\n\n")
	for _, d := range info.Structure {
		if d.Description != "" {
			b.WriteString(fmt.Sprintf("- **%s/** — %s\n", d.Name, d.Description))
		}
		for _, child := range d.Children {
			if child.Description != "" {
				b.WriteString(fmt.Sprintf("  - **%s/** — %s\n", child.Name, child.Description))
			}
		}
	}

	return truncateLines(b.String(), maxDocLines)
}

func renderConventions(info *ProjectInfo) string {
	var b strings.Builder

	b.WriteString("# Code Conventions\n\n")

	for _, lang := range info.Languages {
		fmt.Fprintf(&b, "## %s\n\n", lang.Name)

		// Use project-specific conventions if available
		sample, hasSample := info.Conventions[lang.Name]

		// File naming — prefer detected pattern over generic
		if hasSample && sample.FileNaming != "" {
			fmt.Fprintf(&b, "### File Naming\n\n")
			fmt.Fprintf(&b, "- Detected pattern: **%s**\n", sample.FileNaming)
			if len(sample.ExampleFiles) > 0 {
				b.WriteString("- Examples: ")
				for i, f := range sample.ExampleFiles {
					if i > 0 {
						b.WriteString(", ")
					}
					fmt.Fprintf(&b, "`%s`", f)
				}
				b.WriteString("\n")
			}
			b.WriteString("\n")
		}

		// Language-specific type/variable naming (these are language standard, not project-specific)
		switch lang.Name {
		case "Go":
			b.WriteString("### Naming\n\n")
			b.WriteString("- Packages: `lowercase` (single word preferred)\n")
			b.WriteString("- Exported: `PascalCase`\n")
			b.WriteString("- Unexported: `camelCase`\n\n")

			// Error handling — show detected patterns if available
			b.WriteString("### Error Handling\n\n")
			if hasSample && len(sample.ErrorPatterns) > 0 {
				b.WriteString("Detected patterns in this project:\n\n")
				for _, p := range sample.ErrorPatterns {
					fmt.Fprintf(&b, "- %s\n", p)
				}
				b.WriteString("\n")
			} else {
				b.WriteString("```go\nif err != nil {\n    return fmt.Errorf(\"context: %w\", err)\n}\n```\n\n")
			}

			// Import style
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

		// Tooling section — linter and formatter
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

	for _, lang := range info.Languages {
		switch lang.Name {
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

func renderArchitecture(info *ProjectInfo, opts *RenderOptions) string {
	var b strings.Builder

	b.WriteString("# Architecture\n\n")

	// Include arch analysis if available
	if opts.ArchMap != nil && len(opts.ArchMap.Layers) > 0 {
		b.WriteString("## Layers\n\n")
		for _, layer := range opts.ArchMap.Layers {
			deps := "none"
			if len(layer.AllowedDeps) > 0 {
				deps = strings.Join(layer.AllowedDeps, ", ")
			}
			b.WriteString(fmt.Sprintf("- **%s** (level %d) — depends on: %s\n", layer.Name, layer.Level, deps))
		}
		b.WriteString("\n")

		if len(opts.ArchMap.Domains) > 0 {
			b.WriteString("## Domains\n\n")
			for _, d := range opts.ArchMap.Domains {
				b.WriteString(fmt.Sprintf("- **%s** (`%s`) — %s\n", d.Name, d.Path, d.Description))
			}
			b.WriteString("\n")
		}
	} else {
		b.WriteString("## Overview\n\n")
		b.WriteString("Architecture analysis not available. Run `auto arch generate` to create a detailed analysis.\n\n")

		// Infer basic architecture from directory structure
		for _, lang := range info.Languages {
			switch lang.Name {
			case "Go":
				b.WriteString("### Go Standard Layout\n\n")
				b.WriteString("- `cmd/` → Application entry points (highest level)\n")
				b.WriteString("- `internal/` → Private packages (not importable by external code)\n")
				b.WriteString("- `pkg/` → Public reusable libraries\n\n")
			}
		}
	}

	// Include recent lore decisions
	if len(opts.LoreItems) > 0 {
		b.WriteString("## Recent Decisions\n\n")
		for _, entry := range opts.LoreItems {
			b.WriteString(fmt.Sprintf("### %s\n\n", entry.CommitMsg))
			if entry.Constraint != "" {
				b.WriteString(fmt.Sprintf("**Constraint:** %s\n\n", entry.Constraint))
			}
			if entry.Rejected != "" {
				b.WriteString(fmt.Sprintf("**Rejected:** %s\n\n", entry.Rejected))
			}
			if entry.Directive != "" {
				b.WriteString(fmt.Sprintf("**Directive:** %s\n\n", entry.Directive))
			}
		}
	}

	return truncateLines(b.String(), maxDocLines)
}

func renderTesting(info *ProjectInfo) string {
	var b strings.Builder

	b.WriteString("# Testing\n\n")

	tc := info.TestConfig
	if tc.Framework == "" {
		b.WriteString("No test framework detected.\n\n")
		b.WriteString("## Setup\n\n")
		b.WriteString("Add test framework configuration here.\n")
		return b.String()
	}

	b.WriteString("## Framework\n\n")
	b.WriteString(fmt.Sprintf("- **Framework:** %s\n", tc.Framework))
	b.WriteString(fmt.Sprintf("- **Command:** `%s`\n", tc.Command))
	if tc.CoverageOn {
		b.WriteString("- **Coverage:** Enabled\n")
	}
	b.WriteString("\n")

	if len(tc.Dirs) > 0 {
		b.WriteString("## Test Locations\n\n")
		for _, d := range tc.Dirs {
			b.WriteString(fmt.Sprintf("- `%s/`\n", d))
		}
		b.WriteString("\n")
	}

	// Language-specific patterns
	for _, lang := range info.Languages {
		switch lang.Name {
		case "Go":
			b.WriteString("## Patterns\n\n")
			b.WriteString("### Table-Driven Tests\n\n")
			b.WriteString("```go\nfunc TestExample(t *testing.T) {\n    tests := []struct {\n        name string\n        input string\n        want  string\n    }{\n        {\"basic\", \"input\", \"expected\"},\n    }\n    for _, tt := range tests {\n        t.Run(tt.name, func(t *testing.T) {\n            got := DoSomething(tt.input)\n            assert.Equal(t, tt.want, got)\n        })\n    }\n}\n```\n\n")
			b.WriteString("### Conventions\n\n")
			b.WriteString("- Test files: `*_test.go` (same package)\n")
			b.WriteString("- Use `t.Parallel()` for independent tests\n")
			b.WriteString("- Race detection: `go test -race ./...`\n")
		}
	}

	return truncateLines(b.String(), maxDocLines)
}

func writeTreeEntry(b *strings.Builder, d DirEntry, depth int) {
	prefix := strings.Repeat("    ", depth)
	suffix := "/"
	if d.Description != "" {
		suffix += "  # " + d.Description
	}
	b.WriteString(fmt.Sprintf("%s%s%s\n", prefix, d.Name, suffix))
}

func writeDetailedTreeEntry(b *strings.Builder, d DirEntry, depth int) {
	prefix := strings.Repeat("    ", depth)
	b.WriteString(fmt.Sprintf("%s%s/\n", prefix, d.Name))
	for _, child := range d.Children {
		writeDetailedTreeEntry(b, child, depth+1)
	}
}

func truncateLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	truncated := strings.Join(lines[:maxLines-2], "\n")
	truncated += "\n\n<!-- Truncated: exceeded line limit -->\n"
	return truncated
}
