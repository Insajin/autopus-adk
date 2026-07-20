package terminal

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Compile-time interface satisfaction checks.
var _ SignalCapable = (*CmuxAdapter)(nil)

// TestPlainAdapter_NotSignalCapable verifies PlainAdapter does NOT satisfy SignalCapable.
func TestPlainAdapter_NotSignalCapable(t *testing.T) {
	t.Parallel()

	var term Terminal = &PlainAdapter{}
	_, ok := term.(SignalCapable)
	assert.False(t, ok, "PlainAdapter must not implement SignalCapable")
}

// TestTmuxAdapter_NotSignalCapable verifies TmuxAdapter does NOT satisfy SignalCapable.
func TestTmuxAdapter_NotSignalCapable(t *testing.T) {
	t.Parallel()

	var term Terminal = &TmuxAdapter{}
	_, ok := term.(SignalCapable)
	assert.False(t, ok, "TmuxAdapter must not implement SignalCapable")
}

// TestCmuxAdapter_SurfaceHealth_Success verifies output parsing of surface-health.
func TestCmuxAdapter_SurfaceHealth_Success(t *testing.T) {
	t.Setenv("CMUX_WORKSPACE_ID", "workspace:2")
	restore, captured := newCmuxMockV2("surface:7 type=terminal in_window=true", nil)
	defer restore()

	a := &CmuxAdapter{}
	status, err := a.SurfaceHealth(context.Background(), "surface:7")
	require.NoError(t, err)
	assert.True(t, status.Valid)
	assert.Equal(t, "surface:7", status.SurfaceRef)
	assert.True(t, status.InWindow)
	require.Len(t, captured.calls, 1)
	assert.Equal(t, []string{"surface-health", "--workspace", "workspace:2"}, captured.calls[0].args)
}

// TestCmuxAdapter_SurfaceHealth_NotInWindow verifies in_window=false parsing.
func TestCmuxAdapter_SurfaceHealth_NotInWindow(t *testing.T) {
	restore, _ := newCmuxMockV2("surface:3 type=terminal in_window=false", nil)
	defer restore()

	a := &CmuxAdapter{}
	status, err := a.SurfaceHealth(context.Background(), "surface:3")
	require.NoError(t, err)
	assert.True(t, status.Valid)
	assert.Equal(t, "surface:3", status.SurfaceRef)
	assert.False(t, status.InWindow)
}

// TestCmuxAdapter_SurfaceHealth_MultipleSurfacesSelectsRequested verifies that
// workspace-wide health output cannot overwrite the requested pane with the last line.
func TestCmuxAdapter_SurfaceHealth_MultipleSurfacesSelectsRequested(t *testing.T) {
	output := strings.Join([]string{
		"surface:3 type=terminal in_window=false",
		"surface:7 type=terminal in_window=true",
		"surface:9 type=terminal in_window=false",
	}, "\n")
	restore, _ := newCmuxMockV2(output, nil)
	defer restore()

	a := &CmuxAdapter{}
	status, err := a.SurfaceHealth(context.Background(), "surface:7")
	require.NoError(t, err)
	assert.True(t, status.Valid)
	assert.Equal(t, "surface:7", status.SurfaceRef)
	assert.True(t, status.InWindow)
}

// TestCmuxAdapter_SurfaceHealth_MissingRequestedSurfaceReturnsError verifies that
// a different healthy surface cannot make a missing provider pane look healthy.
func TestCmuxAdapter_SurfaceHealth_MissingRequestedSurfaceReturnsError(t *testing.T) {
	output := strings.Join([]string{
		"surface:3 type=terminal in_window=true",
		"surface:70 type=terminal in_window=true",
	}, "\n")
	restore, _ := newCmuxMockV2(output, nil)
	defer restore()

	a := &CmuxAdapter{}
	status, err := a.SurfaceHealth(context.Background(), "surface:7")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "surface:7")
	assert.False(t, status.Valid)
}

// TestCmuxAdapter_SurfaceHealth_InvalidPaneID verifies validation rejects bad pane IDs.
func TestCmuxAdapter_SurfaceHealth_InvalidPaneID(t *testing.T) {
	t.Parallel()

	a := &CmuxAdapter{}
	_, err := a.SurfaceHealth(context.Background(), "invalid;id")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid pane ID")
}

// TestCmuxAdapter_SurfaceHealth_CommandError verifies error propagation.
func TestCmuxAdapter_SurfaceHealth_CommandError(t *testing.T) {
	restore, _ := newCmuxMockV2("", fmt.Errorf("command failed"))
	defer restore()

	a := &CmuxAdapter{}
	_, err := a.SurfaceHealth(context.Background(), "surface:7")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "surface-health")
}

// TestParseSurfaceHealth_Variants tests various output formats.
func TestParseSurfaceHealth_Variants(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		input     string
		wantRef   string
		wantInWin bool
		wantErr   bool
	}{
		{
			name:      "standard output",
			input:     "surface:7 type=terminal in_window=true",
			wantRef:   "surface:7",
			wantInWin: true,
		},
		{
			name:      "pane ref",
			input:     "pane:3 type=terminal in_window=false",
			wantRef:   "pane:3",
			wantInWin: false,
		},
		{
			name:    "empty output",
			input:   "",
			wantErr: true,
		},
		{
			name:    "no surface ref",
			input:   "type=terminal in_window=true",
			wantErr: true,
		},
		{
			name:    "multiple surface lines",
			input:   "surface:7 type=terminal in_window=true\nsurface:9 type=terminal in_window=false",
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			status, err := parseSurfaceHealth(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantRef, status.SurfaceRef)
			assert.Equal(t, tt.wantInWin, status.InWindow)
			assert.True(t, status.Valid)
		})
	}
}

// TestCmuxAdapter_WaitForSignal_ContextCancellation verifies cancellation behavior.
func TestCmuxAdapter_WaitForSignal_ContextCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	a := &CmuxAdapter{}
	err := a.WaitForSignal(ctx, "test-signal", 5*time.Second)
	assert.Error(t, err)
}

// TestCmuxAdapter_WaitForSignal_InvalidName verifies signal name validation.
func TestCmuxAdapter_WaitForSignal_InvalidName(t *testing.T) {
	t.Parallel()

	a := &CmuxAdapter{}
	err := a.WaitForSignal(context.Background(), "bad;name", 1*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signal name")
}

// TestCmuxAdapter_WaitForSignal_EmptyName verifies empty signal name is rejected.
func TestCmuxAdapter_WaitForSignal_EmptyName(t *testing.T) {
	t.Parallel()

	a := &CmuxAdapter{}
	err := a.WaitForSignal(context.Background(), "", 1*time.Second)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "signal name must not be empty")
}

// TestCmuxAdapter_SendSignal_Success verifies correct cmux command is issued.
func TestCmuxAdapter_SendSignal_Success(t *testing.T) {
	restore, captured := newCmuxMockV2("", nil)
	defer restore()

	a := &CmuxAdapter{}
	err := a.SendSignal(context.Background(), "my-signal")
	require.NoError(t, err)
	require.Len(t, captured.calls, 1)
	combined := strings.Join(captured.calls[0].args, " ")
	assert.Contains(t, combined, "wait-for")
	assert.Contains(t, combined, "-S")
	assert.Contains(t, combined, "my-signal")
}

// TestCmuxAdapter_SendSignal_InvalidName verifies signal name validation on send.
func TestCmuxAdapter_SendSignal_InvalidName(t *testing.T) {
	t.Parallel()

	a := &CmuxAdapter{}
	err := a.SendSignal(context.Background(), "bad name spaces")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signal name")
}

// TestCmuxAdapter_SendSignal_CommandError verifies error propagation from cmux.
func TestCmuxAdapter_SendSignal_CommandError(t *testing.T) {
	orig := execCommand
	execCommand = func(name string, args ...string) *exec.Cmd {
		return exec.Command("false")
	}
	defer func() { execCommand = orig }()

	a := &CmuxAdapter{}
	err := a.SendSignal(context.Background(), "test-signal")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "send signal")
}

// TestValidateSignalName verifies signal name validation rules.
func TestValidateSignalName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		input   string
		wantErr bool
	}{
		{"valid alphanumeric", "test123", false},
		{"valid with hyphens", "my-signal", false},
		{"empty", "", true},
		{"spaces", "bad name", true},
		{"semicolon injection", "bad;rm -rf /", true},
		{"starts with hyphen", "-bad", true},
		{"special chars", "test@signal", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := validateSignalName(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
