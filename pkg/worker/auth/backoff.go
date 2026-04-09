// Package auth provides token lifecycle management for autopus workers.
package auth

import (
	"errors"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"time"
)

const (
	// DefaultBackoffBase is the initial delay for the first retry attempt.
	DefaultBackoffBase = 3 * time.Second

	// DefaultBackoffFactor is the exponential growth factor applied per attempt.
	DefaultBackoffFactor = 2.0

	// DefaultBackoffJitter is the ±fraction applied to each computed delay.
	DefaultBackoffJitter = 0.20

	// DefaultMaxRetries is the maximum number of retry attempts before giving up.
	DefaultMaxRetries = 3
)

// Backoff calculates the delay for the given attempt using exponential backoff with jitter.
//
// attempt=0 → base, attempt=1 → base*factor, attempt=2 → base*factor²
// Jitter applies ±jitter fraction to the computed delay.
// Formula: base * factor^attempt * (1 + rand(-jitter, +jitter))
func Backoff(attempt int, base time.Duration, factor, jitter float64) time.Duration {
	delay := float64(base) * math.Pow(factor, float64(attempt))

	// Apply ±jitter: random value in [-jitter, +jitter] fraction of delay.
	jitterFraction := jitter * (2*rand.Float64() - 1)
	delay *= 1 + jitterFraction

	if delay < 0 {
		delay = 0
	}
	return time.Duration(delay)
}

// IsRetryable returns true if the error indicates a transient failure that should be retried.
//
// 401/403 HTTP responses are NOT retryable (permanent auth failure).
// Network timeouts, temporary errors, and 5xx/429 responses ARE retryable.
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Check for net.Error (timeout, temporary network issues).
	var netErr net.Error
	if errors.As(err, &netErr) {
		return netErr.Timeout() || netErr.Temporary() //nolint:staticcheck
	}

	// Unwrap *url.Error to inspect the underlying HTTP response status.
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		if urlErr.Timeout() {
			return true
		}
		// Check if the inner error carries an HTTP response.
		var httpErr interface{ StatusCode() int }
		if errors.As(urlErr.Err, &httpErr) {
			return IsRetryableStatus(httpErr.StatusCode())
		}
	}

	// Default: treat unknown errors as non-retryable to avoid infinite loops.
	return false
}

// IsRetryableStatus returns true if the HTTP status code indicates a transient failure.
//
// 5xx server errors and 429 Too Many Requests are retryable.
// 401 Unauthorized and 403 Forbidden are NOT retryable (permanent auth failure).
func IsRetryableStatus(statusCode int) bool {
	switch statusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return false
	case http.StatusTooManyRequests:
		return true
	default:
		return statusCode >= 500 && statusCode < 600
	}
}
