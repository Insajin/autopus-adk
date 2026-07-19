package detect

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type runtimeProcessSnapshot struct {
	args      string
	parentPID int
	argsErr   error
	parentErr error
}

func runtimeProcessReaders(snapshot map[int]runtimeProcessSnapshot) (
	func(int) (string, error),
	func(int) (int, error),
) {
	argsOf := func(pid int) (string, error) {
		process, ok := snapshot[pid]
		if !ok {
			return "", assert.AnError
		}
		return process.args, process.argsErr
	}
	parentOf := func(pid int) (int, error) {
		process, ok := snapshot[pid]
		if !ok {
			return 0, assert.AnError
		}
		return process.parentPID, process.parentErr
	}
	return argsOf, parentOf
}

func TestDetectAgentRuntime_HasExportedEntrypoint(t *testing.T) {
	t.Parallel()

	var detector func() AgentRuntime = DetectAgentRuntime
	require.NotNil(t, detector)
}

func TestDetectAgentRuntimeFromProcessTree_AgentAncestor_ReturnsRuntime(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		agentArgs string
		want      AgentRuntime
	}{
		{
			name:      "codex native binary ancestor",
			agentArgs: "/opt/homebrew/lib/node_modules/@openai/codex/vendor/aarch64-apple-darwin/bin/codex --dangerously-bypass-approvals-and-sandbox",
			want:      AgentRuntimeCodex,
		},
		{
			name:      "claude code binary ancestor",
			agentArgs: "/opt/homebrew/bin/claude --session-id test-session --dangerously-skip-permissions",
			want:      AgentRuntimeClaudeCode,
		},
		{
			name:      "codex node entrypoint ancestor",
			agentArgs: "/opt/homebrew/bin/node /opt/homebrew/lib/node_modules/@openai/codex/bin/codex.js",
			want:      AgentRuntimeCodex,
		},
		{
			name:      "claude code node entrypoint ancestor",
			agentArgs: "/opt/homebrew/bin/node /opt/homebrew/lib/node_modules/@anthropic-ai/claude-code/cli.js",
			want:      AgentRuntimeClaudeCode,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			argsOf, parentOf := runtimeProcessReaders(map[int]runtimeProcessSnapshot{
				300: {args: "/bin/zsh -lc auto spec review", parentPID: 200},
				200: {args: "/usr/local/bin/auto spec review", parentPID: 100},
				100: {args: tt.agentArgs, parentPID: 1},
			})

			got := detectAgentRuntimeFromProcessTree(300, maxTreeDepth, argsOf, parentOf)

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDetectAgentRuntimeFromProcessTree_UnrelatedCommands_ReturnsUnknown(t *testing.T) {
	t.Parallel()

	argsOf, parentOf := runtimeProcessReaders(map[int]runtimeProcessSnapshot{
		30: {args: `/bin/zsh -lc "echo codex claude"`, parentPID: 20},
		20: {args: "/usr/bin/node /workspace/claude-worker.js --output codex-report.json", parentPID: 1},
	})

	got := detectAgentRuntimeFromProcessTree(30, maxTreeDepth, argsOf, parentOf)

	assert.Equal(t, AgentRuntimeUnknown, got)
}

func TestDetectAgentRuntimeFromProcessTree_ArgsLookupFails_ContinuesToAgentAncestor(t *testing.T) {
	t.Parallel()

	argsOf, parentOf := runtimeProcessReaders(map[int]runtimeProcessSnapshot{
		30: {argsErr: assert.AnError, parentPID: 20},
		20: {args: "/bin/bash -lc auto orchestra brainstorm", parentPID: 10},
		10: {args: "/opt/homebrew/bin/claude --session-id test-session", parentPID: 1},
	})

	got := detectAgentRuntimeFromProcessTree(30, maxTreeDepth, argsOf, parentOf)

	assert.Equal(t, AgentRuntimeClaudeCode, got)
}

func TestDetectAgentRuntimeFromProcessTree_ParentLookupFails_ReturnsUnknown(t *testing.T) {
	t.Parallel()

	argsOf, parentOf := runtimeProcessReaders(map[int]runtimeProcessSnapshot{
		30: {args: "/bin/zsh -lc auto spec review", parentErr: assert.AnError},
	})

	got := detectAgentRuntimeFromProcessTree(30, maxTreeDepth, argsOf, parentOf)

	assert.Equal(t, AgentRuntimeUnknown, got)
}

func TestDetectAgentRuntimeFromProcessTree_ParentCycle_StopsSafely(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		snapshot  map[int]runtimeProcessSnapshot
		wantCalls int
	}{
		{
			name:      "self parent",
			snapshot:  map[int]runtimeProcessSnapshot{30: {args: "/bin/sh", parentPID: 30}},
			wantCalls: 1,
		},
		{
			name: "two process cycle",
			snapshot: map[int]runtimeProcessSnapshot{
				30: {args: "/bin/sh", parentPID: 20},
				20: {args: "/usr/bin/env", parentPID: 30},
			},
			wantCalls: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			argsCalls := 0
			baseArgsOf, parentOf := runtimeProcessReaders(tt.snapshot)
			argsOf := func(pid int) (string, error) {
				argsCalls++
				return baseArgsOf(pid)
			}

			got := detectAgentRuntimeFromProcessTree(30, maxTreeDepth, argsOf, parentOf)

			assert.Equal(t, AgentRuntimeUnknown, got)
			assert.Equal(t, tt.wantCalls, argsCalls)
		})
	}
}

func TestDetectAgentRuntimeFromProcessTree_DepthBound_StopsBeforeLaterAgent(t *testing.T) {
	t.Parallel()

	argsCalls := 0
	baseArgsOf, parentOf := runtimeProcessReaders(map[int]runtimeProcessSnapshot{
		30: {args: "/bin/sh", parentPID: 20},
		20: {args: "/usr/bin/env", parentPID: 10},
		10: {args: "/opt/homebrew/bin/codex", parentPID: 1},
	})
	argsOf := func(pid int) (string, error) {
		argsCalls++
		return baseArgsOf(pid)
	}

	got := detectAgentRuntimeFromProcessTree(30, 2, argsOf, parentOf)

	assert.Equal(t, AgentRuntimeUnknown, got)
	assert.Equal(t, 2, argsCalls)
}

func TestDetectAgentRuntimeFromProcessTree_NonPositiveDepth_DoesNotScan(t *testing.T) {
	t.Parallel()

	for _, depth := range []int{0, -1} {
		argsCalls := 0
		argsOf := func(int) (string, error) {
			argsCalls++
			return "/opt/homebrew/bin/codex", nil
		}
		parentOf := func(int) (int, error) { return 1, nil }

		got := detectAgentRuntimeFromProcessTree(30, depth, argsOf, parentOf)

		assert.Equal(t, AgentRuntimeUnknown, got, "depth %d", depth)
		assert.Zero(t, argsCalls, "depth %d", depth)
	}
}
