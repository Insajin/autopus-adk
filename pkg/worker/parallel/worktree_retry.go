package parallel

import (
	"errors"
	"strings"
	"time"
)

const (
	lockRetryBase     = 3 * time.Second
	lockRetryFactor   = 2
	lockRetryAttempts = 3
)

type retrySleeper func(time.Duration)

type lockOutputError interface {
	LockOutput() string
}

type worktreeCreateError struct {
	taskID string
	stderr string
	err    error
}

func (e *worktreeCreateError) Error() string {
	return "worktree create " + e.taskID + ": " + strings.TrimSpace(e.stderr) + ": " + e.err.Error()
}

func (e *worktreeCreateError) Unwrap() error {
	return e.err
}

func (e *worktreeCreateError) LockOutput() string {
	return e.stderr
}

func retryOnLock(run func() error, sleep retrySleeper, base time.Duration) error {
	if sleep == nil {
		sleep = time.Sleep
	}
	if base <= 0 {
		base = lockRetryBase
	}

	delay := base
	for attempt := 0; ; attempt++ {
		err := run()
		if err == nil {
			return nil
		}
		if !isLockError(err) || attempt >= lockRetryAttempts {
			return err
		}
		sleep(delay)
		delay *= lockRetryFactor
	}
}

func isLockError(err error) bool {
	if err == nil {
		return false
	}

	var outputErr lockOutputError
	if !errors.As(err, &outputErr) {
		return false
	}

	output := strings.ToLower(outputErr.LockOutput())
	if strings.Contains(output, ".lock") &&
		(strings.Contains(output, "file exists") ||
			strings.Contains(output, "unable to create") ||
			strings.Contains(output, "unable to lock") ||
			strings.Contains(output, "lock file")) {
		return true
	}

	lockPatterns := []string{
		"refs.lock",
		"packed-refs.lock",
		"shallow.lock",
		"index.lock",
		"unable to lock",
		"lock file",
	}
	for _, pattern := range lockPatterns {
		if strings.Contains(output, pattern) {
			return true
		}
	}
	return false
}
