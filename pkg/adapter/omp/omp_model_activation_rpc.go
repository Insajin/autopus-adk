package omp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
)

var ompModelRoleRPCRequest = []byte(`{"id":"autopus-model-state","type":"get_state"}` + "\n")

type ompModelRoleRPCStateRunner interface {
	RunWithInput(context.Context, string, []byte, ...string) ([]byte, error)
}

// SafeOMPModelRoleRPCArgs accepts only provider-free role resolution sessions.
func SafeOMPModelRoleRPCArgs(args []string) bool {
	if len(args) != 9 || args[0] != "--config" || !filepath.IsAbs(args[1]) ||
		strings.ContainsAny(args[1], "\x00\r\n") || args[2] != "--model" ||
		!strings.HasPrefix(args[3], "@") || !isOMPProjectionIdentifier(strings.TrimPrefix(args[3], "@")) ||
		args[4] != "--mode" || args[5] != "rpc" || args[6] != "--no-tools" ||
		args[7] != "--no-skills" || args[8] != "--no-extensions" {
		return false
	}
	return true
}

func readOMPModelRolesViaRPC(
	ctx context.Context,
	runner ompModelRoleRPCStateRunner,
	configPath string,
	expected map[string]string,
) (map[string]string, error) {
	roles := make([]string, 0, len(expected))
	for role := range expected {
		roles = append(roles, role)
	}
	sort.Strings(roles)
	resolved := make(map[string]string, len(roles))
	for _, role := range roles {
		args := []string{
			"--config", configPath, "--model", "@" + role, "--mode", "rpc",
			"--no-tools", "--no-skills", "--no-extensions",
		}
		output, err := runner.RunWithInput(ctx, cliBinary, ompModelRoleRPCRequest, args...)
		if err != nil {
			return nil, fmt.Errorf("activation role readback %s: %w", role, err)
		}
		selector, err := parseOMPModelRoleRPCState(output)
		if err != nil {
			return nil, fmt.Errorf("activation role readback %s: %w", role, err)
		}
		if selector != expected[role] {
			return nil, fmt.Errorf("activation role readback mismatch: %s", role)
		}
		resolved[role] = selector
	}
	return resolved, nil
}

func parseOMPModelRoleRPCState(output []byte) (string, error) {
	for _, line := range bytes.Split(output, []byte{'\n'}) {
		if !bytes.Contains(line, []byte(`"command":"get_state"`)) ||
			!bytes.Contains(line, []byte(`"id":"autopus-model-state"`)) {
			continue
		}
		if rejectDuplicateOMPModelReceiptJSON(line) != nil {
			return "", fmt.Errorf("RPC state contains duplicate fields")
		}
		var frame struct {
			ID      string `json:"id"`
			Type    string `json:"type"`
			Command string `json:"command"`
			Success bool   `json:"success"`
			Data    struct {
				Model struct {
					Provider string `json:"provider"`
					ID       string `json:"id"`
				} `json:"model"`
				Thinking string `json:"thinkingLevel"`
			} `json:"data"`
		}
		if json.Unmarshal(line, &frame) != nil || frame.ID != "autopus-model-state" ||
			frame.Type != "response" || frame.Command != "get_state" || !frame.Success ||
			!safeOMPModelToken(frame.Data.Model.Provider) || !safeOMPModelToken(frame.Data.Model.ID) ||
			!config.IsOMPNativeThinkingLevel(frame.Data.Thinking) {
			return "", fmt.Errorf("RPC state is invalid")
		}
		return frame.Data.Model.Provider + "/" + frame.Data.Model.ID + ":" + frame.Data.Thinking, nil
	}
	return "", fmt.Errorf("RPC state response is missing")
}
