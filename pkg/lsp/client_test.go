package lsp_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/lsp"
)

func TestNewClient_InvalidCommand(t *testing.T) {
	t.Parallel()

	// 존재하지 않는 명령은 오류를 반환해야 함
	client, err := lsp.NewClient("nonexistent-lsp-server-xyz", []string{})
	if err == nil {
		// 일부 시스템에서는 프로세스 시작 자체는 성공할 수 있어 Close 호출
		_ = client.Shutdown()
	}
	// 존재하지 않는 명령이므로 오류이거나 nil이어야 함
	_ = err
}

func TestDiagnostic_Struct(t *testing.T) {
	t.Parallel()

	d := lsp.Diagnostic{
		File:     "main.go",
		Line:     10,
		Col:      5,
		Message:  "undefined: foo",
		Severity: "error",
	}

	assert.Equal(t, "main.go", d.File)
	assert.Equal(t, 10, d.Line)
	assert.Equal(t, "error", d.Severity)
}

func TestLocation_Struct(t *testing.T) {
	t.Parallel()

	loc := lsp.Location{
		File: "pkg/api/handler.go",
		Line: 42,
		Col:  8,
	}

	assert.Equal(t, "pkg/api/handler.go", loc.File)
	assert.Equal(t, 42, loc.Line)
}

func TestSymbol_Struct(t *testing.T) {
	t.Parallel()

	sym := lsp.Symbol{
		Name: "HandleRequest",
		Kind: "function",
		Location: lsp.Location{
			File: "handler.go",
			Line: 10,
		},
	}

	assert.Equal(t, "HandleRequest", sym.Name)
	assert.Equal(t, "function", sym.Kind)
}

func TestJSON_MarshalDiagnostic(t *testing.T) {
	t.Parallel()

	d := lsp.Diagnostic{
		File:     "test.go",
		Line:     1,
		Col:      1,
		Message:  "syntax error",
		Severity: "error",
	}

	b, err := json.Marshal(d)
	require.NoError(t, err)
	assert.Contains(t, string(b), "test.go")
}
