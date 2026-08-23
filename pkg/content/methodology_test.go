// Package content_test는 방법론 콘텐츠 패키지의 테스트이다.
package content_test

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/content"
)

func TestLoadMethodology_TDD(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	tddYAML := `name: tdd
review_gate: true
enforce_rules:
  - "테스트 없이 코드 작성 금지"
stages:
  - name: red
    description: 실패하는 테스트 작성
    rules:
      - "테스트 전 코드 작성 시 거부"
    required_before: ""
  - name: green
    description: 최소 구현으로 테스트 통과
    required_before: red
  - name: refactor
    description: 코드 정리
    required_before: green
`
	err := os.WriteFile(filepath.Join(dir, "tdd.yaml"), []byte(tddYAML), 0644)
	require.NoError(t, err)

	def, err := content.LoadMethodology(filepath.Join(dir, "tdd.yaml"))
	require.NoError(t, err)
	assert.Equal(t, "tdd", def.Name)
	assert.True(t, def.ReviewGate)
	assert.Len(t, def.Stages, 3)
	assert.Equal(t, "red", def.Stages[0].Name)
}

func TestLoadMethodology_NotFound(t *testing.T) {
	t.Parallel()

	_, err := content.LoadMethodology("/nonexistent/path.yaml")
	assert.Error(t, err)
}

func TestLoadMethodologyFromFS(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{
		"methodology/tdd.yaml": &fstest.MapFile{
			Data: []byte(`name: tdd
review_gate: true
enforce_rules:
  - "테스트 없이 코드 작성 금지"
stages:
  - name: red
    description: 실패하는 테스트 작성
  - name: green
    description: 최소 구현으로 테스트 통과
  - name: refactor
    description: 코드 정리
`),
		},
	}

	def, err := content.LoadMethodologyFromFS(fsys, "methodology/tdd.yaml")
	require.NoError(t, err)
	assert.Equal(t, "tdd", def.Name)
	assert.True(t, def.ReviewGate)
	assert.Len(t, def.Stages, 3)
	assert.Equal(t, "red", def.Stages[0].Name)
}

func TestLoadMethodologyFromFS_NotFound(t *testing.T) {
	t.Parallel()

	fsys := fstest.MapFS{}
	_, err := content.LoadMethodologyFromFS(fsys, "methodology/nonexistent.yaml")
	assert.Error(t, err)
}
