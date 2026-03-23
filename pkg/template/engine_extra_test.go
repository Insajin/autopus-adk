package template

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestRenderFile은 파일 기반 템플릿 렌더링을 테스트한다.
func TestRenderFile(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "hello.tmpl")
	require.NoError(t, os.WriteFile(tmplPath, []byte("Hello {{.Name}}!"), 0o644))

	e := New()
	result, err := e.RenderFile(tmplPath, map[string]string{"Name": "World"})
	require.NoError(t, err)
	assert.Equal(t, "Hello World!", result)
}

// TestRenderFile_WithFuncMap은 FuncMap 함수를 사용하는 파일 템플릿을 테스트한다.
func TestRenderFile_WithFuncMap(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		content string
		data    any
		want    string
	}{
		{
			name:    "upper 함수",
			content: `{{upper .Text}}`,
			data:    map[string]string{"Text": "hello world"},
			want:    "HELLO WORLD",
		},
		{
			name:    "lower 함수",
			content: `{{lower .Text}}`,
			data:    map[string]string{"Text": "HELLO WORLD"},
			want:    "hello world",
		},
		{
			name:    "trim 함수",
			content: `{{trim .Text}}`,
			data:    map[string]string{"Text": "  trimmed  "},
			want:    "trimmed",
		},
		{
			name:    "join 함수",
			content: `{{join "-" .Items}}`,
			data:    map[string][]string{"Items": {"a", "b", "c"}},
			want:    "a-b-c",
		},
		{
			name:    "indent 함수",
			content: `{{indent 2 .Text}}`,
			data:    map[string]string{"Text": "line1\nline2"},
			want:    "  line1\n  line2",
		},
		{
			name:    "contains 함수",
			content: `{{contains .Text "hello"}}`,
			data:    map[string]string{"Text": "hello world"},
			want:    "true",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			dir := t.TempDir()
			tmplPath := filepath.Join(dir, "test.tmpl")
			require.NoError(t, os.WriteFile(tmplPath, []byte(tt.content), 0o644))

			e := New()
			result, err := e.RenderFile(tmplPath, tt.data)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestRenderFile_NonExistent는 존재하지 않는 파일 렌더링 오류를 테스트한다.
func TestRenderFile_NonExistent(t *testing.T) {
	t.Parallel()

	e := New()
	_, err := e.RenderFile("/nonexistent/path/template.tmpl", nil)
	require.Error(t, err)
}

// TestRenderFile_InvalidTemplate은 잘못된 파일 템플릿 오류를 테스트한다.
func TestRenderFile_InvalidTemplate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "invalid.tmpl")
	require.NoError(t, os.WriteFile(tmplPath, []byte("{{.Unclosed"), 0o644))

	e := New()
	_, err := e.RenderFile(tmplPath, nil)
	require.Error(t, err)
}

// TestRenderFile_WithStruct는 구조체 데이터로 파일 템플릿을 테스트한다.
func TestRenderFile_WithStruct(t *testing.T) {
	t.Parallel()

	type Config struct {
		Name    string
		Version string
		Enabled bool
	}

	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "config.tmpl")
	content := `name: {{.Name}}
version: {{.Version}}
enabled: {{.Enabled}}`
	require.NoError(t, os.WriteFile(tmplPath, []byte(content), 0o644))

	e := New()
	result, err := e.RenderFile(tmplPath, Config{
		Name:    "autopus",
		Version: "1.0.0",
		Enabled: true,
	})
	require.NoError(t, err)
	assert.Contains(t, result, "autopus")
	assert.Contains(t, result, "1.0.0")
	assert.Contains(t, result, "true")
}

// TestRenderString_ExecuteError는 실행 중 오류가 발생하는 템플릿을 테스트한다.
func TestRenderString_ExecuteError(t *testing.T) {
	t.Parallel()

	e := New()
	// 파이프라인 오류 - 존재하지 않는 함수 호출
	_, err := e.RenderString("{{nonexistent .}}", nil)
	// 존재하지 않는 함수이므로 파싱 오류 발생
	require.Error(t, err)
}

// TestRenderString_ComplexTemplate은 복잡한 템플릿을 테스트한다.
func TestRenderString_ComplexTemplate(t *testing.T) {
	t.Parallel()

	type Item struct {
		Name  string
		Value int
	}

	type Data struct {
		Title string
		Items []Item
	}

	tmpl := `# {{upper .Title}}
{{range .Items}}- {{.Name}}: {{.Value}}
{{end}}`

	e := New()
	result, err := e.RenderString(tmpl, Data{
		Title: "my list",
		Items: []Item{
			{Name: "alpha", Value: 1},
			{Name: "beta", Value: 2},
		},
	})
	require.NoError(t, err)
	assert.Contains(t, result, "MY LIST")
	assert.Contains(t, result, "alpha")
	assert.Contains(t, result, "beta")
}

// TestRenderString_IndentEmpty는 빈 줄에 indent를 적용하는 테스트이다.
func TestRenderString_IndentEmpty(t *testing.T) {
	t.Parallel()

	e := New()
	// 빈 줄이 있는 텍스트에 indent 적용 - 빈 줄은 그대로 빈 줄이어야 함
	result, err := e.RenderString(`{{indent 4 .Text}}`, map[string]string{"Text": "line1\n\nline3"})
	require.NoError(t, err)
	assert.Contains(t, result, "    line1")
	assert.Contains(t, result, "    line3")
}

// TestRenderString_LangName tests the langName funcmap function.
func TestRenderString_LangName(t *testing.T) {
	t.Parallel()

	e := New()
	tests := []struct {
		name string
		code string
		want string
	}{
		{"english", "en", "English"},
		{"korean", "ko", "Korean"},
		{"japanese", "ja", "Japanese"},
		{"chinese", "zh", "Chinese"},
		{"unknown code", "fr", "fr"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result, err := e.RenderString(`{{langName .Code}}`, map[string]string{"Code": tt.code})
			require.NoError(t, err)
			assert.Equal(t, tt.want, result)
		})
	}
}

// TestRenderFile_EmptyTemplate은 빈 파일 템플릿을 테스트한다.
func TestRenderFile_EmptyTemplate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "empty.tmpl")
	require.NoError(t, os.WriteFile(tmplPath, []byte(""), 0o644))

	e := New()
	result, err := e.RenderFile(tmplPath, nil)
	require.NoError(t, err)
	assert.Equal(t, "", result)
}

// TestRenderFile_MultilineTemplate은 여러 줄 파일 템플릿을 테스트한다.
func TestRenderFile_MultilineTemplate(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tmplPath := filepath.Join(dir, "multi.tmpl")
	content := `Line 1: {{.Value1}}
Line 2: {{.Value2}}
Line 3: {{upper .Value3}}`
	require.NoError(t, os.WriteFile(tmplPath, []byte(content), 0o644))

	e := New()
	result, err := e.RenderFile(tmplPath, map[string]string{
		"Value1": "first",
		"Value2": "second",
		"Value3": "third",
	})
	require.NoError(t, err)
	assert.Equal(t, "Line 1: first\nLine 2: second\nLine 3: THIRD", result)
}
