package orchestra

import (
	"bytes"
	"fmt"
	"path"
	"text/template"

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

// render executes the named template with the given data.
func (pb *PromptBuilder) render(name string, data PromptData) (string, error) {
	var buf bytes.Buffer
	if err := pb.tmpl.ExecuteTemplate(&buf, name, data); err != nil {
		return "", fmt.Errorf("prompt_builder: render %s: %w", name, err)
	}
	return buf.String(), nil
}
