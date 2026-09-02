package cli

import "strings"

// pipelineOMPActiveStderrTailBytes bounds the child stderr kept for diagnosis.
// Large enough for a sandbox denial or a Bun stack trace, small enough that a
// looping child cannot turn an error message into a memory problem.
const pipelineOMPActiveStderrTailBytes = 4 << 10

// pipelineOMPActiveStderrDetail renders the child's stderr for an operator-facing
// startup error. The child environment carries the provider token, so the tail is
// masked the same way JSON output is before it can reach a terminal or a log.
func pipelineOMPActiveStderrDetail(captured *boundedOutput) string {
	if captured == nil {
		return ""
	}
	text := strings.TrimSpace(captured.String())
	if text == "" {
		return ""
	}
	if captured.overflow {
		text = "[truncated] " + text
	}
	return ": child stderr: " + sanitizeJSONString("child_stderr", maskHomePath(text))
}
