package lsp_test

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/lsp"
)

// TestMockClient_DiagnosticsEmpty는 빈 진단 목록을 테스트한다.
func TestMockClient_DiagnosticsEmpty(t *testing.T) {
	t.Parallel()

	client := lsp.NewMockClient(nil)
	diags, err := client.Diagnostics("main.go")
	require.NoError(t, err)
	assert.Empty(t, diags)
}

// TestMockClient_DiagnosticsMultipleFiles는 여러 파일의 진단 메시지를 테스트한다.
func TestMockClient_DiagnosticsMultipleFiles(t *testing.T) {
	t.Parallel()

	client := lsp.NewMockClient([]lsp.Diagnostic{
		{File: "main.go", Line: 1, Col: 1, Message: "error in main", Severity: "error"},
		{File: "handler.go", Line: 5, Col: 3, Message: "warning in handler", Severity: "warning"},
		{File: "main.go", Line: 10, Col: 2, Message: "another error", Severity: "error"},
	})

	mainDiags, err := client.Diagnostics("main.go")
	require.NoError(t, err)
	assert.Len(t, mainDiags, 2)

	handlerDiags, err := client.Diagnostics("handler.go")
	require.NoError(t, err)
	assert.Len(t, handlerDiags, 1)
	assert.Equal(t, "warning", handlerDiags[0].Severity)
}

// TestMockClient_ReferencesEmpty는 빈 참조 목록을 테스트한다.
func TestMockClient_ReferencesEmpty(t *testing.T) {
	t.Parallel()

	client := lsp.NewMockClient(nil)
	refs, err := client.References("UnknownSymbol")
	require.NoError(t, err)
	assert.Empty(t, refs)
}

// TestMockClient_SymbolsEmpty는 빈 심볼 목록을 테스트한다.
func TestMockClient_SymbolsEmpty(t *testing.T) {
	t.Parallel()

	client := lsp.NewMockClient(nil)
	syms, err := client.Symbols("nonexistent.go")
	require.NoError(t, err)
	assert.Empty(t, syms)
}

// TestMockClient_SetAndGetDefinition는 정의 설정 및 조회를 테스트한다.
func TestMockClient_SetAndGetDefinition(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		symbol string
		loc    *lsp.Location
	}{
		{
			name:   "정의 있음",
			symbol: "MyFunction",
			loc:    &lsp.Location{File: "pkg/api/handler.go", Line: 42, Col: 1},
		},
		{
			name:   "nil 정의",
			symbol: "NilFunc",
			loc:    nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			client := lsp.NewMockClient(nil)
			client.SetDefinition(tt.symbol, tt.loc)

			got, err := client.Definition(tt.symbol)
			require.NoError(t, err)

			if tt.loc == nil {
				assert.Nil(t, got)
				return
			}

			require.NotNil(t, got)
			assert.Equal(t, tt.loc.File, got.File)
			assert.Equal(t, tt.loc.Line, got.Line)
			assert.Equal(t, tt.loc.Col, got.Col)
		})
	}
}

// TestMockClient_RenameAlwaysSucceeds는 Rename이 항상 성공하는지 테스트한다.
func TestMockClient_RenameAlwaysSucceeds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		oldName string
		newName string
	}{
		{"handleRequest", "HandleRequest"},
		{"foo", "bar"},
		{"oldName", ""},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%s->%s", tt.oldName, tt.newName), func(t *testing.T) {
			t.Parallel()

			client := lsp.NewMockClient(nil)
			err := client.Rename(tt.oldName, tt.newName)
			assert.NoError(t, err)
		})
	}
}

// TestDiagnostic_Severities는 다양한 심각도 수준을 테스트한다.
func TestDiagnostic_Severities(t *testing.T) {
	t.Parallel()

	severities := []string{"error", "warning", "info", "hint"}

	for _, severity := range severities {
		t.Run(severity, func(t *testing.T) {
			t.Parallel()

			client := lsp.NewMockClient([]lsp.Diagnostic{
				{File: "test.go", Line: 1, Col: 1, Message: "test message", Severity: severity},
			})

			diags, err := client.Diagnostics("test.go")
			require.NoError(t, err)
			require.Len(t, diags, 1)
			assert.Equal(t, severity, diags[0].Severity)
		})
	}
}

// TestMockClient_MultipleRefs는 여러 참조를 설정하고 조회하는 테스트이다.
func TestMockClient_MultipleRefs(t *testing.T) {
	t.Parallel()

	client := lsp.NewMockClient(nil)

	locs := []lsp.Location{
		{File: "main.go", Line: 5, Col: 1},
		{File: "handler.go", Line: 12, Col: 3},
		{File: "api_test.go", Line: 25, Col: 2},
	}
	client.SetRefs("ProcessRequest", locs)

	refs, err := client.References("ProcessRequest")
	require.NoError(t, err)
	assert.Len(t, refs, 3)
	assert.Equal(t, "main.go", refs[0].File)
	assert.Equal(t, "handler.go", refs[1].File)
	assert.Equal(t, "api_test.go", refs[2].File)
}

// TestMockClient_ImplementsCommander는 MockClient가 Commander 인터페이스를 구현하는지 확인한다.
func TestMockClient_ImplementsCommander(t *testing.T) {
	t.Parallel()

	var _ lsp.Commander = lsp.NewMockClient(nil)
}
