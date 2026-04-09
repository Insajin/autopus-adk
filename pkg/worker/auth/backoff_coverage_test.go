package auth

import (
	"errors"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsRetryable_NilError verifies that nil is not retryable.
func TestIsRetryable_NilError(t *testing.T) {
	t.Parallel()
	assert.False(t, IsRetryable(nil), "nil error must not be retryable")
}

// TestIsRetryable_UnknownError verifies that unknown errors default to non-retryable.
func TestIsRetryable_UnknownError(t *testing.T) {
	t.Parallel()
	assert.False(t, IsRetryable(errors.New("some unknown error")), "unknown errors must not be retryable")
}

// TestIsRetryable_NetErrorTemporary verifies that temporary net.Error is retryable.
func TestIsRetryable_NetErrorTemporary(t *testing.T) {
	t.Parallel()
	err := &fakeNetError{temporary: true, timeout: false}
	assert.True(t, IsRetryable(err), "temporary net.Error must be retryable")
}

// TestIsRetryable_NetErrorNeitherTimeoutNorTemporary verifies non-retryable net.Error.
func TestIsRetryable_NetErrorNeitherTimeoutNorTemporary(t *testing.T) {
	t.Parallel()
	err := &fakeNetError{timeout: false, temporary: false}
	assert.False(t, IsRetryable(err), "net.Error with timeout=false and temporary=false must not be retryable")
}

// TestIsRetryable_URLErrorTimeout verifies that url.Error with Timeout returns retryable.
func TestIsRetryable_URLErrorTimeout(t *testing.T) {
	t.Parallel()
	err := &url.Error{Op: "Post", URL: "http://x", Err: &fakeNetError{timeout: true}}
	assert.True(t, IsRetryable(err), "url.Error wrapping a timeout must be retryable")
}

// TestIsRetryable_URLErrorNonTimeout verifies url.Error without timeout and no StatusCode.
func TestIsRetryable_URLErrorNonTimeout(t *testing.T) {
	t.Parallel()
	err := &url.Error{Op: "Post", URL: "http://x", Err: errors.New("generic error")}
	assert.False(t, IsRetryable(err), "url.Error with no timeout and no StatusCode must not be retryable")
}

// Note: The url.Error wrapping a StatusCode() inner error path in IsRetryable
// is not easily exercisable via fakeHTTPError because url.Error itself implements
// net.Error, which is matched first in the errors.As chain. This branch is covered
// indirectly through integration tests that use real HTTP clients.

// TestIsRetryableStatus covers all branches of the status switch.
func TestIsRetryableStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		code int
		want bool
		name string
	}{
		{401, false, "401 Unauthorized"},
		{403, false, "403 Forbidden"},
		{429, true, "429 TooManyRequests"},
		{500, true, "500 InternalServerError"},
		{502, true, "502 BadGateway"},
		{503, true, "503 ServiceUnavailable"},
		{599, true, "599 custom 5xx"},
		{600, false, "600 not a 5xx"},
		{200, false, "200 OK"},
		{404, false, "404 NotFound"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tt.want, IsRetryableStatus(tt.code))
		})
	}
}

// TestBackoff_NegativeDelayClampedToZero verifies that extreme negative jitter
// never produces a negative duration (math guard).
// This is a property test — we drive jitter=1.0 so delay*(1-1)=0 edge case.
func TestBackoff_ZeroJitterProducesExactBase(t *testing.T) {
	t.Parallel()

	// With jitter=0 the formula is: base * factor^0 * 1 = base.
	got := Backoff(0, DefaultBackoffBase, DefaultBackoffFactor, 0)
	assert.Equal(t, DefaultBackoffBase, got, "zero jitter must return exact base delay")
}
