// Package content_test는 방법론 콘텐츠 트리 관문의 테스트이다.
package content_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/content"
)

// TestValidateMethodologyContent는 생성 파이프라인의 콘텐츠 관문을 고정한다.
func TestValidateMethodologyContent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		file    string
		body    string
		wantErr []string
	}{
		{
			name: "valid chain",
			file: "tdd.yaml",
			body: "name: tdd\nstages:\n  - name: red\n    required_before: \"\"\n  - name: green\n    required_before: red\n",
		},
		{
			name:    "cycle is rejected",
			file:    "loop.yaml",
			body:    "name: loop\nstages:\n  - name: red\n    required_before: green\n  - name: green\n    required_before: red\n",
			wantErr: []string{"loop.yaml", "cycle"},
		},
		{
			name:    "name must match the file name",
			file:    "tdd.yaml",
			body:    "name: kanban\nstages:\n  - name: red\n",
			wantErr: []string{"tdd.yaml", "kanban"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			root := t.TempDir()
			dir := filepath.Join(root, "methodology")
			require.NoError(t, os.MkdirAll(dir, 0o755))
			require.NoError(t, os.WriteFile(filepath.Join(dir, tc.file), []byte(tc.body), 0o644))

			err := content.ValidateMethodologyContent(root)
			if len(tc.wantErr) == 0 {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			for _, fragment := range tc.wantErr {
				assert.Contains(t, err.Error(), fragment)
			}
		})
	}
}

// TestValidateMethodologyContent_MissingDirectory는 정의 디렉터리가 없는
// 콘텐츠 트리에서 관문이 통과함을 고정한다.
func TestValidateMethodologyContent_MissingDirectory(t *testing.T) {
	t.Parallel()

	require.NoError(t, content.ValidateMethodologyContent(t.TempDir()))
}

// TestValidateMethodologyContent_ShippedTree는 배포되는 콘텐츠 트리가 관문을
// 통과함을 고정한다.
func TestValidateMethodologyContent_ShippedTree(t *testing.T) {
	t.Parallel()

	require.NoError(t, content.ValidateMethodologyContent(filepath.Join("..", "..", "content")))
}
