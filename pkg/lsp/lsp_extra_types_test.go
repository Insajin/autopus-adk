package lsp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/insajin/autopus-adk/pkg/lsp"
)

// TestSymbol_AllFields는 Symbol 구조체의 모든 필드를 테스트한다.
func TestSymbol_AllFields(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		sym  lsp.Symbol
	}{
		{
			name: "함수",
			sym: lsp.Symbol{
				Name: "HandleRequest",
				Kind: "function",
				Location: lsp.Location{
					File: "handler.go",
					Line: 10,
					Col:  1,
				},
			},
		},
		{
			name: "구조체",
			sym: lsp.Symbol{
				Name: "Config",
				Kind: "struct",
				Location: lsp.Location{
					File: "config.go",
					Line: 5,
					Col:  1,
				},
			},
		},
		{
			name: "변수",
			sym: lsp.Symbol{
				Name: "defaultTimeout",
				Kind: "variable",
				Location: lsp.Location{
					File: "const.go",
					Line: 1,
					Col:  1,
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			assert.NotEmpty(t, tt.sym.Name)
			assert.NotEmpty(t, tt.sym.Kind)
			assert.NotEmpty(t, tt.sym.Location.File)
			assert.Positive(t, tt.sym.Location.Line)
		})
	}
}
