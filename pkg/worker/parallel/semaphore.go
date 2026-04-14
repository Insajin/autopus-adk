package parallel

import "context"

// TaskSemaphore limits concurrent task execution with FIFO queuing.
// It uses a buffered channel internally so waiters are served in order.
type TaskSemaphore struct {
	sem   chan struct{}
	limit int
}

// NewTaskSemaphore creates a semaphore with the given concurrency limit.
// Panics if limit is less than 1.
func NewTaskSemaphore(limit int) *TaskSemaphore {
	if limit < 1 {
		panic("parallel: semaphore limit must be >= 1")
	}
	return &TaskSemaphore{
		sem:   make(chan struct{}, limit),
		limit: limit,
	}
}

// Acquire blocks until a slot is available or ctx is cancelled.
func (s *TaskSemaphore) Acquire(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	select {
	case s.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Release frees a slot for the next waiting task.
// Must be called exactly once for each successful Acquire.
func (s *TaskSemaphore) Release() {
	select {
	case <-s.sem:
	default:
		panic("parallel: release called on empty semaphore")
	}
}

// Available returns the number of free slots.
func (s *TaskSemaphore) Available() int {
	return s.limit - len(s.sem)
}

// Limit returns the configured concurrency limit.
func (s *TaskSemaphore) Limit() int {
	return s.limit
}
