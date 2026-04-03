package setup

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHumanError_ServerErrors(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"request failed: HTTP 500 Internal Server Error"},
		{"backend returned HTTP 502"},
		{"HTTP 503 service unavailable"},
	}
	for _, tt := range tests {
		got := HumanError(errors.New(tt.input))
		assert.Equal(t, "Autopus 서버에 연결할 수 없습니다. 인터넷 연결을 확인하고 다시 시도해주세요.", got)
	}
}

func TestHumanError_AuthErrors(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"PKCE verification failed"},
		{"invalid code_verifier"},
		{"device_code expired"},
	}
	for _, tt := range tests {
		got := HumanError(errors.New(tt.input))
		assert.Equal(t, "서버 인증 중 오류가 발생했습니다. 잠시 후 다시 시도해주세요.", got)
	}
}

func TestHumanError_NetworkErrors(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"dial tcp: connection refused"},
		{"lookup api.autopus.co: no such host"},
		{"context deadline exceeded: timeout"},
	}
	for _, tt := range tests {
		got := HumanError(errors.New(tt.input))
		assert.Equal(t, "네트워크 연결에 실패했습니다. 인터넷 연결을 확인해주세요.", got)
	}
}

func TestHumanError_TokenErrors(t *testing.T) {
	tests := []struct {
		input string
	}{
		{"token expired at 2024-01-01"},
		{"unauthorized access"},
		{"server returned 401"},
	}
	for _, tt := range tests {
		got := HumanError(errors.New(tt.input))
		assert.Equal(t, "인증이 만료되었습니다. 다시 로그인해주세요.", got)
	}
}

func TestHumanError_Fallback(t *testing.T) {
	err := errors.New("some unknown error")
	got := HumanError(err)
	assert.Equal(t, "some unknown error", got)
}

func TestHumanError_Nil(t *testing.T) {
	got := HumanError(nil)
	assert.Equal(t, "", got)
}

func TestHumanErrorf(t *testing.T) {
	err := errors.New("HTTP 500 boom")
	got := HumanErrorf("setup failed: %s", err)
	require.NotNil(t, got)
	assert.Equal(t, "setup failed: Autopus 서버에 연결할 수 없습니다. 인터넷 연결을 확인하고 다시 시도해주세요.", got.Error())
}

func TestHumanErrorf_Nil(t *testing.T) {
	got := HumanErrorf("should not happen: %s", nil)
	assert.Nil(t, got)
}

func TestHumanErrorf_Fallback(t *testing.T) {
	err := errors.New("weird error")
	got := HumanErrorf("context: %s", err)
	require.NotNil(t, got)
	assert.Equal(t, "context: weird error", got.Error())
}
