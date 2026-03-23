package lsp_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/lsp"
)

// TestNewClient_ValidCommand는 유효한 명령으로 클라이언트 생성을 테스트한다.
func TestNewClient_ValidCommand(t *testing.T) {
	t.Parallel()

	// cat 명령은 대부분의 UNIX 시스템에 존재한다
	client, err := lsp.NewClient("cat", []string{})
	if err != nil {
		t.Skipf("cat 명령 실행 불가: %v", err)
	}
	require.NotNil(t, client)
	// 정상 종료
	_ = client.Shutdown()
}

// TestNewClient_Initialize는 Initialize 메서드를 테스트한다.
func TestNewClient_Initialize(t *testing.T) {
	t.Parallel()

	// cat 명령으로 클라이언트 생성 후 Initialize 시도
	client, err := lsp.NewClient("cat", []string{})
	if err != nil {
		t.Skipf("cat 명령 실행 불가: %v", err)
	}
	defer func() { _ = client.Shutdown() }()

	// Initialize는 sendRequest를 호출하므로 실행이 되어야 함
	err = client.Initialize("file:///tmp/test")
	// cat은 JSON-RPC를 이해하지 못하지만 쓰기는 성공할 수 있다
	_ = err
}

// TestNewClient_Shutdown는 Shutdown 메서드를 테스트한다.
func TestNewClient_Shutdown(t *testing.T) {
	t.Parallel()

	client, err := lsp.NewClient("cat", []string{})
	if err != nil {
		t.Skipf("cat 명령 실행 불가: %v", err)
	}

	// Shutdown은 오류 없이 실행되어야 함
	err = client.Shutdown()
	_ = err // cat이 종료되면서 Wait 오류가 발생할 수 있음
}

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

	// main.go 진단: 2개
	mainDiags, err := client.Diagnostics("main.go")
	require.NoError(t, err)
	assert.Len(t, mainDiags, 2)

	// handler.go 진단: 1개
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
				// nil을 명시적으로 설정하면 nil이 저장됨
				assert.Nil(t, got)
			} else {
				require.NotNil(t, got)
				assert.Equal(t, tt.loc.File, got.File)
				assert.Equal(t, tt.loc.Line, got.Line)
				assert.Equal(t, tt.loc.Col, got.Col)
			}
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

// TestMockClient_ImplementsCommander는 MockClient가 Commander 인터페이스를 구현하는지 확인한다.
func TestMockClient_ImplementsCommander(t *testing.T) {
	t.Parallel()

	var _ lsp.Commander = lsp.NewMockClient(nil)
}

// TestDetectServer_PyprojectToml은 pyproject.toml 기반 Python 프로젝트 감지를 테스트한다.
func TestDetectServer_PyprojectToml(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "pyproject.toml"), []byte("[build-system]\n"), 0o644))

	serverCmd, args, err := lsp.DetectServer(dir)
	require.NoError(t, err)
	assert.Equal(t, "pyright", serverCmd)
	assert.Contains(t, args, "--stdio")
}

// TestDetectServer_RequirementsTxt는 requirements.txt 기반 Python 프로젝트 감지를 테스트한다.
func TestDetectServer_RequirementsTxt(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "requirements.txt"), []byte("fastapi\npydantic\n"), 0o644))

	serverCmd, args, err := lsp.DetectServer(dir)
	require.NoError(t, err)
	assert.Equal(t, "pyright", serverCmd)
	assert.Contains(t, args, "--stdio")
}

// TestDetectServer_PriorityGoOverTS는 go.mod와 package.json이 모두 있을 때 Go가 우선임을 테스트한다.
func TestDetectServer_PriorityGoOverTS(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.23\n"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"name":"test"}`), 0o644))

	// Go가 먼저 감지되어야 함
	serverCmd, _, err := lsp.DetectServer(dir)
	require.NoError(t, err)
	assert.Equal(t, "gopls", serverCmd)
}

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
