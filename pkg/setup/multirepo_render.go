package setup

import (
	"fmt"
	"strings"
)

func renderWorkspaceSection(info *MultiRepoInfo) string {
	var b strings.Builder
	b.WriteString("## Workspace\n\n")
	b.WriteString("- **Type:** multi-repo\n")
	fmt.Fprintf(&b, "- **Root:** `%s`\n", info.WorkspaceRoot)
	fmt.Fprintf(&b, "- **Repositories:** %d\n\n", len(info.Components))

	b.WriteString("### Repository List\n\n")
	for _, component := range info.Components {
		fmt.Fprintf(
			&b,
			"- `%s/` — %s (%s, %s, %s)\n",
			component.Path,
			component.Name,
			component.Role,
			component.PrimaryLanguage,
			defaultText(component.RemoteURL, "no remote"),
		)
	}
	b.WriteString("\n")

	b.WriteString("### Dependency Graph\n\n")
	if len(info.Dependencies) == 0 {
		b.WriteString("- No cross-repo dependencies detected.\n\n")
	} else {
		for _, dep := range info.Dependencies {
			fmt.Fprintf(&b, "- %s -> %s [%s]\n", dep.Source, dep.Target, dep.Type)
		}
		b.WriteString("\n")
	}

	b.WriteString("### Deploy Targets\n\n")
	for _, component := range info.Components {
		fmt.Fprintf(&b, "- `%s` -> %s\n", component.Name, inferDeployTarget(component))
	}
	b.WriteString("\n")
	return b.String()
}

func renderDevWorkflow(info *MultiRepoInfo) string {
	var b strings.Builder
	b.WriteString("## Development Workflow\n\n")
	b.WriteString("- Make code changes in the owning repository first; keep the workspace root for shared docs and coordination.\n")
	b.WriteString("- Validate downstream repositories in dependency order whenever a shared module changes.\n")
	b.WriteString("- Prefer local `replace` or `file:` links during development until dependent repositories are updated and released.\n")
	if len(info.Dependencies) > 0 {
		b.WriteString("- Current cross-repo coordination paths:\n")
		for _, dep := range info.Dependencies {
			fmt.Fprintf(&b, "  - `%s` depends on `%s` via `%s`\n", dep.Source, dep.Target, dep.Type)
		}
	}
	b.WriteString("\n")
	return b.String()
}

func renderRepoBoundaries(info *MultiRepoInfo) string {
	var b strings.Builder
	b.WriteString("## Repository Boundaries\n\n")
	for _, component := range info.Components {
		fmt.Fprintf(&b, "- [git repo] %s/ — %s — %s\n", component.Path, component.Role, defaultText(component.RemoteURL, "no remote"))
	}
	return b.String()
}

func inferDeployTarget(component RepoComponent) string {
	lower := strings.ToLower(component.Name + " " + component.Role)
	switch {
	case strings.Contains(lower, "desktop"):
		return "desktop distribution"
	case strings.Contains(lower, "tap"):
		return "homebrew distribution"
	case strings.Contains(lower, "web") || strings.Contains(lower, "frontend"):
		return "web application hosting"
	case strings.Contains(lower, "backend") || strings.Contains(lower, "api") || strings.Contains(lower, "server"):
		return "backend service runtime"
	case component.Path == ".":
		return "workspace coordination and docs"
	default:
		return "local development"
	}
}

func defaultText(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}
