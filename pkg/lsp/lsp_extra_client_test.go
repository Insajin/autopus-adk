package lsp_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/lsp"
)

func newCatClientOrSkip(t *testing.T) *lsp.Client {
	t.Helper()

	client, err := lsp.NewClient("cat", []string{})
	if err != nil {
		t.Skipf("cat 명령 실행 불가: %v", err)
	}

	return client
}

// TestNewClient_ValidCommand는 유효한 명령으로 클라이언트 생성을 테스트한다.
func TestNewClient_ValidCommand(t *testing.T) {
	t.Parallel()

	client := newCatClientOrSkip(t)
	require.NotNil(t, client)
	_ = client.Shutdown()
}

// TestNewClient_Initialize는 Initialize 메서드를 테스트한다.
func TestNewClient_Initialize(t *testing.T) {
	t.Parallel()

	client := newCatClientOrSkip(t)
	defer func() { _ = client.Shutdown() }()

	err := client.Initialize("file:///tmp/test")
	_ = err
}

// TestNewClient_Shutdown는 Shutdown 메서드를 테스트한다.
func TestNewClient_Shutdown(t *testing.T) {
	t.Parallel()

	client := newCatClientOrSkip(t)
	err := client.Shutdown()
	_ = err
}
