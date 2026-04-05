package pipeline_test

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"github.com/insajin/autopus-adk/pkg/pipeline"
)

// FakeBackend is a test double for pipeline.PhaseBackend that records calls
// and returns pre-configured responses in order.
type FakeBackend struct {
	mu              sync.Mutex
	Responses       []string
	ReceivedPrompts []string
	callIdx         int
	CallCount       int
}

// Execute records the prompt and returns the next configured response.
func (f *FakeBackend) Execute(_ context.Context, req pipeline.PhaseRequest) (*pipeline.PhaseResponse, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.ReceivedPrompts = append(f.ReceivedPrompts, req.Prompt)
	f.CallCount++

	resp := ""
	if f.callIdx < len(f.Responses) {
		resp = f.Responses[f.callIdx]
		f.callIdx++
	}
	return &pipeline.PhaseResponse{Output: resp}, nil
}

// FakeConcurrentBackend is a test double that tracks maximum concurrent
// executions to verify parallel execution.
type FakeConcurrentBackend struct {
	mu            sync.Mutex
	Responses     map[pipeline.PhaseID]string
	current       int64
	MaxConcurrent int
}

// Execute records concurrency level and returns the pre-configured response
// for the given phase.
func (f *FakeConcurrentBackend) Execute(_ context.Context, req pipeline.PhaseRequest) (*pipeline.PhaseResponse, error) {
	cur := int(atomic.AddInt64(&f.current, 1))
	defer atomic.AddInt64(&f.current, -1)

	f.mu.Lock()
	if cur > f.MaxConcurrent {
		f.MaxConcurrent = cur
	}
	f.mu.Unlock()

	// Small delay to allow goroutines to overlap and measure concurrency.
	time.Sleep(10 * time.Millisecond)

	resp := ""
	if f.Responses != nil {
		resp = f.Responses[req.PhaseID]
	}
	return &pipeline.PhaseResponse{Output: resp}, nil
}
