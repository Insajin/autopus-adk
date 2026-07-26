package orchestra

import "time"

// @AX:NOTE: [AUTO] one-minute startup floor absorbs post-teardown CLI launch latency; the owning judge request remains the hard cap
const freshJudgeStartupFloor = time.Minute

// freshJudgeStartupTimeout gives an isolated judge enough time to start after
// participant teardown while preserving larger provider-specific budgets. The
// startup phase must never outlive the judge request that owns it.
func freshJudgeStartupTimeout(provider ProviderConfig, requestTimeout time.Duration) time.Duration {
	timeout := startupTimeoutFor(provider)
	if timeout < freshJudgeStartupFloor {
		timeout = freshJudgeStartupFloor
	}
	if requestTimeout > 0 && timeout > requestTimeout {
		return requestTimeout
	}
	return timeout
}
