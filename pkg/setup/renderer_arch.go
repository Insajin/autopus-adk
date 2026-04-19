package setup

import (
	"fmt"
	"strings"
)

func renderArchitecture(info *ProjectInfo, opts *RenderOptions) string {
	var b strings.Builder
	b.WriteString("# Architecture\n\n")

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
			for _, domain := range opts.ArchMap.Domains {
				b.WriteString(fmt.Sprintf("- **%s** (`%s`) — %s\n", domain.Name, domain.Path, domain.Description))
			}
			b.WriteString("\n")
		}
	} else {
		b.WriteString("## Overview\n\n")
		b.WriteString("Architecture analysis not available. Run `auto arch generate` to create a detailed analysis.\n\n")
		for _, language := range info.Languages {
			if language.Name != "Go" {
				continue
			}
			b.WriteString("### Go Standard Layout\n\n")
			b.WriteString("- `cmd/` → Application entry points (highest level)\n")
			b.WriteString("- `internal/` → Private packages (not importable by external code)\n")
			b.WriteString("- `pkg/` → Public reusable libraries\n\n")
		}
	}

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

	testConfig := info.TestConfig
	if testConfig.Framework == "" {
		b.WriteString("No test framework detected.\n\n")
		b.WriteString("## Setup\n\n")
		b.WriteString("Add test framework configuration here.\n")
		return b.String()
	}

	b.WriteString("## Framework\n\n")
	b.WriteString(fmt.Sprintf("- **Framework:** %s\n", testConfig.Framework))
	b.WriteString(fmt.Sprintf("- **Command:** `%s`\n", testConfig.Command))
	if testConfig.CoverageOn {
		b.WriteString("- **Coverage:** Enabled\n")
	}
	b.WriteString("\n")

	if len(testConfig.Dirs) > 0 {
		b.WriteString("## Test Locations\n\n")
		for _, dirName := range testConfig.Dirs {
			b.WriteString(fmt.Sprintf("- `%s/`\n", dirName))
		}
		b.WriteString("\n")
	}

	for _, language := range info.Languages {
		if language.Name != "Go" {
			continue
		}
		b.WriteString("## Patterns\n\n")
		b.WriteString("### Table-Driven Tests\n\n")
		b.WriteString("```go\nfunc TestExample(t *testing.T) {\n    tests := []struct {\n        name string\n        input string\n        want  string\n    }{\n        {\"basic\", \"input\", \"expected\"},\n    }\n    for _, tt := range tests {\n        t.Run(tt.name, func(t *testing.T) {\n            got := DoSomething(tt.input)\n            assert.Equal(t, tt.want, got)\n        })\n    }\n}\n```\n\n")
		b.WriteString("### Conventions\n\n")
		b.WriteString("- Test files: `*_test.go` (same package)\n")
		b.WriteString("- Use `t.Parallel()` for independent tests\n")
		b.WriteString("- Race detection: `go test -race ./...`\n")
	}

	return truncateLines(b.String(), maxDocLines)
}
