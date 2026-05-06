package orchestra

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"text/template"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/insajin/autopus-adk/templates"
)

// orchestraTemplates lists all orchestra template files that must be parsed together
// (the role templates reference the context partial via {{template ...}}).
var orchestraTemplates = []string{
	"shared/orchestra-context.md.tmpl",
	"shared/orchestra-debater-r1.md.tmpl",
	"shared/orchestra-debater-r2.md.tmpl",
	"shared/orchestra-judge.md.tmpl",
	"shared/orchestra-reviewer.md.tmpl",
	"shared/orchestra-consensus.md.tmpl",
}

// PromptBuilder renders orchestra prompts from embedded Go templates.
type PromptBuilder struct {
	tmpl *template.Template
}

// NewPromptBuilder parses all orchestra templates and returns a ready builder.
func NewPromptBuilder() (*PromptBuilder, error) {
	tmpl := template.New("")
	for _, name := range orchestraTemplates {
		data, err := templates.FS.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("prompt_builder: read %s: %w", name, err)
		}
		// Register under full path (shared/orchestra-*.md.tmpl).
		if _, err := tmpl.New(name).Parse(string(data)); err != nil {
			return nil, fmt.Errorf("prompt_builder: parse %s: %w", name, err)
		}
		// Also register under basename so {{template "orchestra-context.md.tmpl" .}} resolves.
		base := path.Base(name)
		if base != name {
			if _, err := tmpl.New(base).Parse(string(data)); err != nil {
				return nil, fmt.Errorf("prompt_builder: parse alias %s: %w", base, err)
			}
		}
	}
	return &PromptBuilder{tmpl: tmpl}, nil
}

// BuildDebaterR1 renders the Round 1 independent analysis prompt.
func (pb *PromptBuilder) BuildDebaterR1(data PromptData) (string, error) {
	return pb.render("shared/orchestra-debater-r1.md.tmpl", data)
}

// BuildDebaterR1WithManifest renders Round 1 and returns a diagnostic prompt layer manifest.
func (pb *PromptBuilder) BuildDebaterR1WithManifest(data PromptData) (string, PromptManifest, error) {
	return pb.renderWithManifest("debater_r1", "shared/orchestra-debater-r1.md.tmpl", data)
}

// BuildDebaterR2 renders the Round 2 cross-pollination prompt.
func (pb *PromptBuilder) BuildDebaterR2(data PromptData) (string, error) {
	return pb.render("shared/orchestra-debater-r2.md.tmpl", data)
}

// BuildJudge renders the final judge synthesis prompt.
func (pb *PromptBuilder) BuildJudge(data PromptData) (string, error) {
	return pb.render("shared/orchestra-judge.md.tmpl", data)
}

// BuildReviewer renders the SPEC reviewer prompt.
func (pb *PromptBuilder) BuildReviewer(data PromptData) (string, error) {
	return pb.render("shared/orchestra-reviewer.md.tmpl", data)
}

// BuildReviewerWithManifest renders a reviewer prompt and returns its layer manifest.
func (pb *PromptBuilder) BuildReviewerWithManifest(data PromptData) (string, PromptManifest, error) {
	return pb.renderWithManifest("reviewer", "shared/orchestra-reviewer.md.tmpl", data)
}

// render executes the named template with the given data.
func (pb *PromptBuilder) render(name string, data PromptData) (string, error) {
	var buf bytes.Buffer
	if err := pb.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("prompt_builder: render %s: %w", name, err)
	}
	return buf.String(), nil
}

func (pb *PromptBuilder) renderWithManifest(role, name string, data PromptData) (string, PromptManifest, error) {
	prompt, err := pb.render(name, data)
	if err != nil {
		return "", PromptManifest{}, err
	}
	manifest, err := buildPromptManifest(role, name, data)
	if err != nil {
		return "", PromptManifest{}, err
	}
	return prompt, manifest, nil
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-AGENT-PROMPT-001: orchestra:* layer IDs and groups are diagnostic manifest contracts.
func buildPromptManifest(role, templateName string, data PromptData) (PromptManifest, error) {
	layers := []promptlayer.Layer{
		{
			ID:            "orchestra:" + role + ":identity",
			Kind:          promptlayer.KindStable,
			Group:         promptlayer.GroupIdentityRules,
			SourceRef:     templateName,
			Content:       stableTemplateLayerContent(templateName),
			CacheEligible: true,
		},
		{
			ID:            "orchestra:project-context",
			Kind:          promptlayer.KindStable,
			Group:         promptlayer.GroupProjectContext,
			SourceRef:     "orchestra.PromptData project fields",
			Content:       projectContextLayerContent(data),
			CacheEligible: true,
		},
		{
			ID:        "orchestra:" + role + ":task",
			Kind:      promptlayer.KindEphemeral,
			Group:     promptlayer.GroupUserRequest,
			SourceRef: "orchestra.PromptData task fields",
			Content:   taskLayerContent(data),
		},
	}
	if data.SnapshotID != "" {
		sourceRef := safeSnapshotSourceRef(data.SnapshotSourceRefs)
		if sourceRef == "" {
			sourceRef = "snapshot"
		}
		layers = append(layers, promptlayer.SnapshotLayer(safeSnapshotID(data.SnapshotID), sourceRef, data.SnapshotContent))
	}
	rendered, err := promptlayer.Render(layers)
	if err != nil {
		return PromptManifest{}, err
	}
	return rendered.Manifest, nil
}

func projectContextLayerContent(data PromptData) string {
	var sb strings.Builder
	sb.WriteString(data.ProjectName)
	sb.WriteString("\n")
	sb.WriteString(data.ProjectSummary)
	sb.WriteString("\n")
	sb.WriteString(data.TechStack)
	sb.WriteString("\n")
	sb.WriteString(strings.Join(data.Components, "\n"))
	sb.WriteString("\n")
	sb.WriteString(strings.Join(data.MustReadFiles, "\n"))
	for _, path := range data.RelevantPaths {
		sb.WriteString("\n")
		sb.WriteString(path.Path)
		sb.WriteString(": ")
		sb.WriteString(path.Description)
	}
	sb.WriteString("\n")
	sb.WriteString(data.TargetModule)
	return sb.String()
}

func stableTemplateLayerContent(templateName string) string {
	names := []string{"shared/orchestra-context.md.tmpl", templateName}
	var sb strings.Builder
	for _, name := range names {
		data, err := templates.FS.ReadFile(name)
		if err != nil {
			sb.WriteString("missing template: ")
			sb.WriteString(name)
			sb.WriteString("\n")
			continue
		}
		sb.WriteString("template: ")
		sb.WriteString(name)
		sb.WriteString("\n")
		sb.Write(data)
		sb.WriteString("\n")
	}
	return sb.String()
}

func taskLayerContent(data PromptData) string {
	var sb strings.Builder
	sb.WriteString(data.Topic)
	sb.WriteString("\n")
	sb.WriteString(data.SpecContent)
	sb.WriteString("\n")
	sb.WriteString(data.CodeContext)
	sb.WriteString("\n")
	sb.WriteString(data.SchemaMethod)
	sb.WriteString("\n")
	sb.WriteString(data.SchemaJSON)
	return sb.String()
}

var (
	safeSnapshotIDPattern        = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$`)
	safeSnapshotSourceRefPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_./:-]{0,255}$`)
	windowsAbsPathPattern        = regexp.MustCompile(`^[A-Za-z]:/`)
)

func safeSnapshotID(id string) string {
	id = strings.TrimSpace(id)
	if safeSnapshotIDPattern.MatchString(id) && !strings.ContainsAny(id, `/\`) && !looksSensitiveRef(id) {
		return id
	}
	return "snapshot:" + shortDigest(id)
}

func safeSnapshotSourceRef(refs []string) string {
	safe := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		if isSafeSnapshotSourceRef(ref) {
			safe = append(safe, filepath.ToSlash(filepath.Clean(ref)))
			continue
		}
		safe = append(safe, "snapshot-ref:"+shortDigest(ref))
	}
	return strings.Join(safe, ",")
}

func isSafeSnapshotSourceRef(ref string) bool {
	if filepath.IsAbs(ref) || windowsAbsPathPattern.MatchString(ref) || strings.HasPrefix(ref, "~") || strings.ContainsAny(ref, `\`) || looksSensitiveRef(ref) {
		return false
	}
	clean := filepath.Clean(ref)
	return clean != "." && clean != ".." && !strings.HasPrefix(clean, "../") && safeSnapshotSourceRefPattern.MatchString(ref)
}

func looksSensitiveRef(ref string) bool {
	lower := strings.ToLower(ref)
	for _, marker := range []string{"api_key", "apikey", "access_token", "auth_token", "secret", "password", "token", "sk-", "ghp_", "github_pat_"} {
		if strings.Contains(lower, marker) {
			return true
		}
	}
	return false
}

func shortDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])[:12]
}
