package tui

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRenderHeader_ContainsConnectionStatus(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		status   ConnStatus
		expected string
	}{
		{"connected", ConnConnected, "connected"},
		{"disconnected", ConnDisconnected, "disconnected"},
		{"reconnecting", ConnReconnecting, "reconnecting"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := renderHeader(tt.status, nil, 80)
			assert.Contains(t, result, tt.expected)
		})
	}
}

func TestRenderHeader_WithProviders(t *testing.T) {
	t.Parallel()

	providers := []ProviderInfo{
		{Name: "claude", Available: true},
		{Name: "codex", Available: false},
	}

	result := renderHeader(ConnConnected, providers, 80)
	assert.Contains(t, result, "claude")
	assert.Contains(t, result, "codex")
}

func TestRenderHeader_NoProviders(t *testing.T) {
	t.Parallel()

	result := renderHeader(ConnConnected, nil, 80)
	assert.Contains(t, result, "no providers registered")
}

func TestRenderProgressBar_Boundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		progress float64
		expected string
	}{
		{"zero", 0.0, "  0%"},
		{"half", 0.5, " 50%"},
		{"full", 1.0, "100%"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			result := renderProgressBar(tt.progress, 40)
			assert.Contains(t, result, tt.expected)
			assert.Contains(t, result, "[")
			assert.Contains(t, result, "]")
		})
	}
}

func TestRenderProgressBar_SmallWidth(t *testing.T) {
	t.Parallel()

	// Should not panic even with very small width
	result := renderProgressBar(0.5, 5)
	assert.Contains(t, result, " 50%")
}

func TestRenderTaskQueue_Empty(t *testing.T) {
	t.Parallel()

	result := renderTaskQueue(nil, 80)
	assert.Contains(t, result, "Task Queue")
	assert.Contains(t, result, "idle")
}

func TestRenderTaskQueue_WithTasks(t *testing.T) {
	t.Parallel()

	tasks := []TaskInfo{
		{ID: "t-1", Description: "first task", Status: "running"},
		{ID: "t-2", Description: "second task", Status: "completed"},
	}

	result := renderTaskQueue(tasks, 80)
	assert.Contains(t, result, "t-1")
	assert.Contains(t, result, "first task")
	assert.Contains(t, result, "t-2")
}

func TestRenderTaskQueue_OverflowTruncation(t *testing.T) {
	t.Parallel()

	tasks := make([]TaskInfo, 8)
	for i := range tasks {
		tasks[i] = TaskInfo{ID: "t", Description: "task", Status: "pending"}
	}

	result := renderTaskQueue(tasks, 80)
	assert.Contains(t, result, "+3 more")
}

func TestClampWidth(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 40, clampWidth(10))   // below minimum
	assert.Equal(t, 76, clampWidth(80))   // normal
	assert.Equal(t, 120, clampWidth(200)) // above maximum
}
