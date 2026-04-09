package auth

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBackoff_BaseDelay verifies that attempt=0 returns the base delay of 3s (±20%).
func TestBackoff_BaseDelay(t *testing.T) {
	t.Parallel()

	delay := Backoff(0, DefaultBackoffBase, DefaultBackoffFactor, DefaultBackoffJitter)

	// Base=3s with ±20% jitter → [2.4s, 3.6s].
	assert.GreaterOrEqual(t, delay, 2400*time.Millisecond)
	assert.LessOrEqual(t, delay, 3600*time.Millisecond)
}

// TestBackoff_ExponentialGrowth verifies that delays double on each attempt.
func TestBackoff_ExponentialGrowth(t *testing.T) {
	t.Parallel()

	tests := []struct {
		attempt int
		wantMin time.Duration
		wantMax time.Duration
	}{
		// attempt=1: base*factor^1=6s ±20% → [4.8s, 7.2s]
		{attempt: 1, wantMin: 4800 * time.Millisecond, wantMax: 7200 * time.Millisecond},
		// attempt=2: base*factor^2=12s ±20% → [9.6s, 14.4s]
		{attempt: 2, wantMin: 9600 * time.Millisecond, wantMax: 14400 * time.Millisecond},
	}

	for _, tt := range tests {
		tt := tt
		t.Run("attempt_"+itoa(tt.attempt), func(t *testing.T) {
			t.Parallel()
			delay := Backoff(tt.attempt, DefaultBackoffBase, DefaultBackoffFactor, DefaultBackoffJitter)
			assert.GreaterOrEqual(t, delay, tt.wantMin)
			assert.LessOrEqual(t, delay, tt.wantMax)
		})
	}
}

// TestBackoff_JitterRange calls Backoff many times and verifies all results
// stay within ±20% of the base delay.
func TestBackoff_JitterRange(t *testing.T) {
	t.Parallel()

	const samples = 200
	min := time.Duration(float64(DefaultBackoffBase) * 0.8)
	max := time.Duration(float64(DefaultBackoffBase) * 1.2)

	for i := 0; i < samples; i++ {
		delay := Backoff(0, DefaultBackoffBase, DefaultBackoffFactor, DefaultBackoffJitter)
		require.GreaterOrEqual(t, delay, min, "jitter below -20%% on sample %d", i)
		require.LessOrEqual(t, delay, max, "jitter above +20%% on sample %d", i)
	}
}

// TestIsRetryable_NetworkError verifies that network timeout errors are retryable.
func TestIsRetryable_NetworkError(t *testing.T) {
	t.Parallel()

	// net.Error with Timeout()=true → retryable.
	err := &fakeNetError{timeout: true}
	assert.True(t, IsRetryable(err), "network timeout errors must be retryable")
}

// TestIsRetryable_401Unauthorized verifies that 401 responses are NOT retryable.
func TestIsRetryable_401Unauthorized(t *testing.T) {
	t.Parallel()

	// 401 means wrong credentials — retrying won't help.
	assert.False(t, IsRetryableStatus(401), "401 must not be retryable")
}

// itoa converts small ints to strings for table-driven sub-test names.
func itoa(n int) string {
	switch n {
	case 0:
		return "0"
	case 1:
		return "1"
	case 2:
		return "2"
	default:
		return "n"
	}
}

// fakeNetError implements net.Error for testing network timeout scenarios.
type fakeNetError struct {
	timeout   bool
	temporary bool
}

func (e *fakeNetError) Error() string   { return "fake net error" }
func (e *fakeNetError) Timeout() bool   { return e.timeout }
func (e *fakeNetError) Temporary() bool { return e.temporary } //nolint:staticcheck
