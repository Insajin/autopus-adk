package detect

import (
	"os"
	"path/filepath"
	"strings"
)

// AgentRuntime identifies the agent CLI that owns the current process tree.
type AgentRuntime string

const (
	AgentRuntimeUnknown        AgentRuntime = "unknown"
	AgentRuntimeClaudeCode     AgentRuntime = "claude-code"
	AgentRuntimeCodex          AgentRuntime = "codex"
	AgentRuntimeAntigravityCLI AgentRuntime = "antigravity-cli"
	AgentRuntimeOpenCode       AgentRuntime = "opencode"
)

// DetectAgentRuntime walks the bounded parent process tree looking for a known
// agent CLI executable.
// @AX:ANCHOR: [AUTO] public runtime-identity boundary shared by terminal and orchestra invocation detection
// @AX:REASON: [AUTO] callers rely on bounded ancestry and exact executable identity to avoid ambient marker false positives
func DetectAgentRuntime() AgentRuntime {
	return detectAgentRuntimeFromProcessTree(
		os.Getppid(),
		maxTreeDepth,
		processArgs,
		parentPIDOf,
	)
}

// @AX:NOTE: [AUTO] ancestry traversal is depth-bounded and cycle-safe; the nearest exact known runtime wins
func detectAgentRuntimeFromProcessTree(
	startPID int,
	maxDepth int,
	argsOf func(int) (string, error),
	parentOf func(int) (int, error),
) AgentRuntime {
	if startPID <= 1 || maxDepth <= 0 || argsOf == nil || parentOf == nil {
		return AgentRuntimeUnknown
	}

	visited := make(map[int]struct{}, maxDepth)
	pid := startPID
	for depth := 0; depth < maxDepth && pid > 1; depth++ {
		if _, seen := visited[pid]; seen {
			return AgentRuntimeUnknown
		}
		visited[pid] = struct{}{}

		if args, err := argsOf(pid); err == nil {
			if runtime := agentRuntimeFromProcessArgs(args); runtime != AgentRuntimeUnknown {
				return runtime
			}
		}

		parentPID, err := parentOf(pid)
		if err != nil || parentPID <= 1 {
			return AgentRuntimeUnknown
		}
		pid = parentPID
	}

	return AgentRuntimeUnknown
}

// @AX:NOTE: [AUTO] executable basenames and known Node entrypoint suffixes are matched exactly; do not broaden to substring detection
func agentRuntimeFromProcessArgs(args string) AgentRuntime {
	fields := strings.Fields(args)
	if len(fields) == 0 {
		return AgentRuntimeUnknown
	}

	executable := filepath.Base(strings.Trim(fields[0], `"'`))
	switch executable {
	case "codex":
		return AgentRuntimeCodex
	case "claude", "claude-code":
		return AgentRuntimeClaudeCode
	case "agy", "gemini", "gemini-cli":
		return AgentRuntimeAntigravityCLI
	case "opencode":
		return AgentRuntimeOpenCode
	case "node":
		return agentRuntimeFromNodeEntrypoint(fields)
	default:
		return AgentRuntimeUnknown
	}
}

func agentRuntimeFromNodeEntrypoint(fields []string) AgentRuntime {
	if len(fields) < 2 {
		return AgentRuntimeUnknown
	}

	entrypoint := filepath.ToSlash(strings.Trim(fields[1], `"'`))
	switch {
	case strings.HasSuffix(entrypoint, "/@openai/codex/bin/codex.js"):
		return AgentRuntimeCodex
	case strings.HasSuffix(entrypoint, "/@anthropic-ai/claude-code/cli.js"):
		return AgentRuntimeClaudeCode
	case strings.HasSuffix(entrypoint, "/@google/gemini-cli/bundle/gemini.js"):
		return AgentRuntimeAntigravityCLI
	case strings.HasSuffix(entrypoint, "/opencode-ai/bin/opencode"):
		return AgentRuntimeOpenCode
	default:
		return AgentRuntimeUnknown
	}
}
