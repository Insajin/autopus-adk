package orchestra

import (
	"fmt"
	"os"
	"strings"
)

func applyCodexLastMessageOutput(stdout, stderr, path string) (string, string) {
	if path == "" {
		return stdout, stderr
	}
	lastMessage, err := os.ReadFile(path)
	if err != nil {
		return stdout, appendSubprocessDiagnostic(stderr, fmt.Sprintf("read codex last message: %v", err))
	}
	if strings.TrimSpace(string(lastMessage)) != "" {
		return string(lastMessage), stderr
	}
	return stdout, stderr
}
