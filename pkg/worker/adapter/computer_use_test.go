// Package adapter - tests for ComputerUse support in BuildCommand and audit events.
//
// Phase 1.5 RED scaffold: TaskConfig.ComputerUse field does NOT exist yet.
// All tests MUST fail until Phase 2 implementation.
package adapter

import (
	"context"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildCommand_ComputerUseEnabled_AddsFlag verifies that when
// TaskConfig.ComputerUse is true, the Claude adapter includes --computer-use
// in the command arguments.
func TestBuildCommand_ComputerUseEnabled_AddsFlag(t *testing.T) {
	t.Parallel()

	a := NewClaudeAdapter()
	task := TaskConfig{
		TaskID:      "task-cu-1",
		ComputerUse: true, // Does not exist yet — RED.
	}

	cmd := a.BuildCommand(context.Background(), task)

	// Assert: --computer-use flag is present in args.
	require.NotNil(t, cmd, "BuildCommand should return a valid command")
	assert.True(t, slices.Contains(cmd.Args, "--computer-use"),
		"args should contain --computer-use when ComputerUse=true, got: %v", cmd.Args)
}

// TestBuildCommand_ComputerUseDisabled_NoFlag verifies that when
// TaskConfig.ComputerUse is false (default), no computer-use flag is added.
func TestBuildCommand_ComputerUseDisabled_NoFlag(t *testing.T) {
	t.Parallel()

	a := NewClaudeAdapter()
	task := TaskConfig{
		TaskID:      "task-cu-2",
		ComputerUse: false, // Does not exist yet — RED.
	}

	cmd := a.BuildCommand(context.Background(), task)

	// Assert: no computer-use flag.
	require.NotNil(t, cmd, "BuildCommand should return a valid command")
	assert.False(t, slices.Contains(cmd.Args, "--computer-use"),
		"args should NOT contain --computer-use when ComputerUse=false, got: %v", cmd.Args)
}

// TestAuditEvent_ComputerUse_IncludesMetadata verifies that when a task
// is executed with ComputerUse=true, the generated AuditEvent includes
// the ComputerUse metadata field.
func TestAuditEvent_ComputerUse_IncludesMetadata(t *testing.T) {
	t.Parallel()

	// This test references AuditEvent.ComputerUse which does NOT exist yet.
	// The import path and type will come from the worker package.
	// For now, we test at the adapter level by verifying TaskConfig propagation.

	task := TaskConfig{
		TaskID:      "task-cu-audit-1",
		ComputerUse: true, // Does not exist yet — RED.
	}

	// Assert: ComputerUse field is accessible and set to true.
	assert.True(t, task.ComputerUse,
		"TaskConfig.ComputerUse should be true")

	// Assert: the field value can be used to populate audit metadata.
	// This verifies the data flow: TaskConfig → execution → AuditEvent.
	evt := buildAuditMetadata(task) // Does not exist yet — RED.
	assert.True(t, evt.ComputerUse,
		"AuditEvent.ComputerUse should reflect TaskConfig.ComputerUse=true")
}

// AuditMetadata represents audit event metadata generated from task config.
// This is a scaffold — the actual type lives in the worker package.
// type AuditMetadata struct {
//     ComputerUse bool
// }

// buildAuditMetadata is a scaffold for the function that maps TaskConfig
// fields to audit event metadata. Does NOT exist yet — RED.
// func buildAuditMetadata(task TaskConfig) AuditMetadata
