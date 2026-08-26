package content

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransformAgentForOMP_RemovesParentTodoAndAddsOnlyRoleNativeTools(t *testing.T) {
	for _, test := range []struct {
		name      string
		source    string
		required  []string
		forbidden []string
	}{
		{
			name:     "executor receives lsp without todo",
			source:   "Read, Write, Edit, Grep, Glob, Bash, TodoWrite",
			required: []string{"lsp"}, forbidden: []string{"todo", "browser", "inspect_image"},
		},
		{
			name:     "debugger receives lsp",
			source:   "Read, Write, Edit, Grep, Glob, Bash",
			required: []string{"lsp"}, forbidden: []string{"todo", "browser", "inspect_image"},
		},
		{
			name:     "reviewer receives lsp",
			source:   "Read, Grep, Glob, Bash",
			required: []string{"lsp"}, forbidden: []string{"todo", "browser", "inspect_image"},
		},
		{
			name:     "validator receives lsp",
			source:   "Read, Grep, Glob, Bash",
			required: []string{"lsp"}, forbidden: []string{"todo", "browser", "inspect_image"},
		},
		{
			name:     "frontend specialist receives visual tools and lsp",
			source:   "Read, Write, Edit, Grep, Glob, Bash",
			required: []string{"browser", "inspect_image", "lsp"}, forbidden: []string{"todo"},
		},
		{
			name:     "ux validator receives visual tools",
			source:   "Read, Grep, Glob, Bash",
			required: []string{"browser", "inspect_image"}, forbidden: []string{"todo", "lsp"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			name := strings.ReplaceAll(strings.Split(test.name, " receives")[0], " ", "-")
			rendered := TransformAgentForOMP(AgentSource{
				Meta: AgentSourceMeta{Name: name, Tools: test.source},
			})
			frontmatter, _, ok := strings.Cut(strings.TrimPrefix(rendered, "---\n"), "---\n")
			require.True(t, ok)
			for _, tool := range test.required {
				assert.Contains(t, frontmatter, "  - "+tool+"\n")
			}
			for _, tool := range test.forbidden {
				assert.NotContains(t, frontmatter, "  - "+tool+"\n")
			}
		})
	}
}
