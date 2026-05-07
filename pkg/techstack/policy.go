package techstack

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ProjectMode identifies whether stack choices should preserve existing manifests
// or resolve current stable versions for a new project.
type ProjectMode string

const (
	ModeGreenfield ProjectMode = "greenfield"
	ModeBrownfield ProjectMode = "brownfield"
)

// SourceRef records the source used to justify a version choice.
type SourceRef struct {
	Name      string
	URL       string
	Version   string
	CheckedAt time.Time
}

// Candidate is a selected or rejected technology/library version.
type Candidate struct {
	Name       string
	Version    string
	Stability  string
	SourceRefs []SourceRef
	Notes      []string
}

// Decision is the structured technology stack decision that PRD/SPEC research
// should preserve before implementation starts.
type Decision struct {
	Mode            ProjectMode
	Selected        []Candidate
	Rejected        []Candidate
	Constraints     []string
	AllowPrerelease bool
	GeneratedAt     time.Time
}

var manifestNames = []string{
	"go.mod",
	"package.json",
	"pyproject.toml",
	"requirements.txt",
	"Cargo.toml",
	"pnpm-workspace.yaml",
	"yarn.lock",
	"package-lock.json",
	"bun.lockb",
}

// InferMode classifies the request as greenfield or brownfield. Explicit request
// wording wins over local manifests because users often ask for a new project from
// inside a meta workspace.
func InferMode(projectDir, description string) ProjectMode {
	lower := strings.ToLower(description)
	for _, keyword := range []string{
		"greenfield",
		"new project",
		"from scratch",
		"scaffold",
		"starter",
		"신규 프로젝트",
		"새 프로젝트",
		"처음부터",
		"스캐폴드",
		"스타터",
	} {
		if strings.Contains(lower, keyword) {
			return ModeGreenfield
		}
	}
	if HasManifest(projectDir) {
		return ModeBrownfield
	}
	return ModeGreenfield
}

// HasManifest reports whether projectDir already contains a dependency/build
// manifest that should be treated as a brownfield compatibility constraint.
func HasManifest(projectDir string) bool {
	for _, name := range manifestNames {
		if _, err := os.Stat(filepath.Join(projectDir, name)); err == nil {
			return true
		}
	}
	return false
}

// ValidateDecision enforces the freshness evidence required for stack decisions.
func ValidateDecision(decision Decision) error {
	switch decision.Mode {
	case ModeGreenfield:
		return validateGreenfield(decision)
	case ModeBrownfield:
		return validateBrownfield(decision)
	default:
		return fmt.Errorf("unknown project mode %q", decision.Mode)
	}
}

func validateGreenfield(decision Decision) error {
	if len(decision.Selected) == 0 {
		return errors.New("greenfield stack decision requires at least one selected candidate")
	}
	for _, candidate := range decision.Selected {
		if strings.TrimSpace(candidate.Name) == "" {
			return errors.New("greenfield stack candidate missing name")
		}
		if strings.TrimSpace(candidate.Version) == "" {
			return fmt.Errorf("%s: greenfield stack candidate missing resolved version", candidate.Name)
		}
		if !decision.AllowPrerelease && looksPrerelease(candidate.Version) {
			return fmt.Errorf("%s: prerelease version %q requires explicit allow_prerelease", candidate.Name, candidate.Version)
		}
		if len(candidate.SourceRefs) == 0 {
			return fmt.Errorf("%s: greenfield stack candidate requires at least one source ref", candidate.Name)
		}
		for _, ref := range candidate.SourceRefs {
			if strings.TrimSpace(ref.Name) == "" {
				return fmt.Errorf("%s: source ref missing name", candidate.Name)
			}
			if strings.TrimSpace(ref.Version) == "" {
				return fmt.Errorf("%s: source ref %q missing version", candidate.Name, ref.Name)
			}
			if ref.CheckedAt.IsZero() {
				return fmt.Errorf("%s: source ref %q missing checked_at", candidate.Name, ref.Name)
			}
		}
	}
	return nil
}

func validateBrownfield(decision Decision) error {
	if len(decision.Constraints) == 0 && len(decision.Selected) == 0 {
		return errors.New("brownfield stack decision requires existing constraints or selected candidates")
	}
	for _, candidate := range decision.Selected {
		if strings.TrimSpace(candidate.Name) == "" {
			return errors.New("brownfield stack candidate missing name")
		}
	}
	return nil
}

func looksPrerelease(version string) bool {
	lower := strings.ToLower(version)
	if strings.Contains(lower, "-") {
		return true
	}
	for _, marker := range []string{"alpha", "beta", "rc", "next", "canary", "preview", "snapshot"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

// RenderMarkdown returns a compact research.md section for the stack decision.
func RenderMarkdown(decision Decision) string {
	var b strings.Builder
	b.WriteString("## Technology Stack Decision\n\n")
	fmt.Fprintf(&b, "- mode: %s\n", decision.Mode)
	if !decision.GeneratedAt.IsZero() {
		fmt.Fprintf(&b, "- generated_at: %s\n", decision.GeneratedAt.UTC().Format(time.RFC3339))
	}
	fmt.Fprintf(&b, "- allow_prerelease: %t\n\n", decision.AllowPrerelease)

	if len(decision.Constraints) > 0 {
		b.WriteString("### Constraints\n\n")
		for _, c := range decision.Constraints {
			fmt.Fprintf(&b, "- %s\n", c)
		}
		b.WriteString("\n")
	}

	b.WriteString("### Selected\n\n")
	b.WriteString("| Technology | Version | Stability | Source refs |\n")
	b.WriteString("|------------|---------|-----------|-------------|\n")
	for _, candidate := range decision.Selected {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			candidate.Name,
			candidate.Version,
			candidate.Stability,
			formatSourceRefs(candidate.SourceRefs),
		)
	}

	if len(decision.Rejected) > 0 {
		b.WriteString("\n### Rejected\n\n")
		for _, candidate := range decision.Rejected {
			fmt.Fprintf(&b, "- %s %s", candidate.Name, candidate.Version)
			if len(candidate.Notes) > 0 {
				fmt.Fprintf(&b, " - %s", strings.Join(candidate.Notes, "; "))
			}
			b.WriteString("\n")
		}
	}

	return b.String()
}

func formatSourceRefs(refs []SourceRef) string {
	if len(refs) == 0 {
		return "-"
	}
	parts := make([]string, 0, len(refs))
	for _, ref := range refs {
		label := ref.Name
		if ref.Version != "" {
			label += "@" + ref.Version
		}
		if !ref.CheckedAt.IsZero() {
			label += " checked " + ref.CheckedAt.UTC().Format("2006-01-02")
		}
		parts = append(parts, label)
	}
	return strings.Join(parts, "<br>")
}
