package template

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderString(t *testing.T) {
	t.Parallel()
	e := New()
	result, err := e.RenderString("Hello {{.Name}}", map[string]string{"Name": "World"})
	require.NoError(t, err)
	assert.Equal(t, "Hello World", result)
}

func TestRenderString_FuncMap(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		tmpl string
		data any
		want string
	}{
		{
			name: "upper",
			tmpl: `{{upper .Text}}`,
			data: map[string]string{"Text": "hello"},
			want: "HELLO",
		},
		{
			name: "lower",
			tmpl: `{{lower .Text}}`,
			data: map[string]string{"Text": "HELLO"},
			want: "hello",
		},
		{
			name: "trim",
			tmpl: `{{trim .Text}}`,
			data: map[string]string{"Text": "  hello  "},
			want: "hello",
		},
		{
			name: "join",
			tmpl: `{{join ", " .Items}}`,
			data: map[string][]string{"Items": {"a", "b", "c"}},
			want: "a, b, c",
		},
		{
			name: "indent",
			tmpl: `{{indent 4 .Text}}`,
			data: map[string]string{"Text": "line1\nline2"},
			want: "    line1\n    line2",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			e := New()
			result, err := e.RenderString(tt.tmpl, tt.data)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

func TestRenderString_InvalidTemplate(t *testing.T) {
	t.Parallel()
	e := New()
	_, err := e.RenderString("{{.Undefined", nil)
	require.Error(t, err)
}
