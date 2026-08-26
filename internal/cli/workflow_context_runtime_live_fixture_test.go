package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	errWorkflowContextLiveEarlyAdmission     = errors.New("provider admission occurred before rehydration")
	errWorkflowContextLiveUnexpectedDispatch = errors.New("unexpected context dispatch mode")
	workflowContextLiveVersionPattern        = regexp.MustCompile(`^omp/[0-9]+\.[0-9]+\.[0-9]+$`)
)

type workflowContextLiveLayout struct {
	base, runtime, workspace, sessions, overlay string
}

func newWorkflowContextLiveLayout(base string) (workflowContextLiveLayout, error) {
	if err := os.Chmod(base, 0o700); err != nil {
		return workflowContextLiveLayout{}, errors.New("base-mode")
	}
	layout := workflowContextLiveLayout{
		base: base, runtime: filepath.Join(base, "omp-runtime"),
		workspace: filepath.Join(base, "workspace"), sessions: filepath.Join(base, "omp-runtime", "sessions"),
		overlay: filepath.Join(base, "omp-runtime", "context-live.yml"),
	}
	for _, path := range []string{layout.runtime, layout.workspace, layout.sessions,
		filepath.Join(layout.runtime, "home"), filepath.Join(layout.runtime, "tmp"),
		filepath.Join(layout.runtime, "cache"), filepath.Join(layout.runtime, "config"),
		filepath.Join(layout.runtime, "data"), filepath.Join(layout.runtime, "state")} {
		if err := os.MkdirAll(path, 0o700); err != nil {
			return workflowContextLiveLayout{}, errors.New("mkdir")
		}
		if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o700 {
			return workflowContextLiveLayout{}, errors.New("directory-mode")
		}
	}
	return layout, nil
}

func (l workflowContextLiveLayout) writeConfig(endpoint string) error {
	models := fmt.Sprintf(`providers:
  contextfake:
    baseUrl: %s/v1
    auth: none
    api: openai-completions
    models:
      - id: context-canary
        name: Context Canary
        reasoning: false
        input: [text, image]
        contextWindow: 8192
        maxTokens: 512
`, endpoint)
	overlay := `contextPromotion:
  enabled: false
compaction:
  enabled: true
  methodOrder: [snapcompact]
  midTurnEnabled: false
  thresholdTokens: 7000
  thresholdPercent: -1
  reserveTokens: 128
  keepRecentTokens: 256
  autoContinue: false
memory:
  backend: off
skills:
  enableCodexUser: false
  enableClaudeUser: false
  enableClaudeProject: false
  enablePiUser: false
  enablePiProject: false
  enableAgentsUser: false
  enableAgentsProject: false
`
	for path, body := range map[string]string{
		filepath.Join(l.runtime, "models.yml"): models, l.overlay: overlay,
	} {
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			return errors.New("config-write")
		}
		if info, err := os.Stat(path); err != nil || info.Mode().Perm() != 0o600 {
			return errors.New("config-mode")
		}
	}
	return nil
}

func (l workflowContextLiveLayout) env() []string {
	closedProxy := "http://127.0.0.1:1"
	return []string{
		"HOME=" + filepath.Join(l.runtime, "home"), "TMPDIR=" + filepath.Join(l.runtime, "tmp"),
		"XDG_CACHE_HOME=" + filepath.Join(l.runtime, "cache"), "XDG_CONFIG_HOME=" + filepath.Join(l.runtime, "config"),
		"XDG_DATA_HOME=" + filepath.Join(l.runtime, "data"), "XDG_STATE_HOME=" + filepath.Join(l.runtime, "state"),
		"PI_CODING_AGENT_DIR=" + l.runtime, "PI_CONFIG_FILES=" + l.overlay, "PATH=" + os.Getenv("PATH"),
		"NO_PROXY=127.0.0.1,localhost,::1", "no_proxy=127.0.0.1,localhost,::1",
		"HTTP_PROXY=" + closedProxy, "HTTPS_PROXY=" + closedProxy, "ALL_PROXY=" + closedProxy,
	}
}

func probeWorkflowContextLiveVersion(
	ctx context.Context, executable string, layout workflowContextLiveLayout, endpoint string,
) (string, error) {
	output, err := runWorkflowContextLiveMetadata(ctx, executable, layout, endpoint, "--version")
	version := strings.TrimSpace(string(output))
	if err != nil || !workflowContextLiveVersionPattern.MatchString(version) {
		return "", errors.New("identity-unverified")
	}
	return version, nil
}

func verifyWorkflowContextLiveOverlay(
	ctx context.Context, executable string, layout workflowContextLiveLayout, endpoint string,
) error {
	expected := map[string]any{
		"compaction.enabled": true, "compaction.methodOrder": []any{"snapcompact"},
		"compaction.thresholdTokens": float64(7000), "compaction.autoContinue": false,
		"memory.backend": "off",
	}
	for key, want := range expected {
		output, err := runWorkflowContextLiveMetadata(ctx, executable, layout, endpoint,
			"config", "get", key, "--json")
		if err != nil {
			return fmt.Errorf("readback-exec:%s", key)
		}
		var got struct {
			Key   string `json:"key"`
			Value any    `json:"value"`
		}
		if json.Unmarshal(output, &got) != nil || got.Key != key || !reflectLiveValue(got.Value, want) {
			return fmt.Errorf("readback-mismatch:%s", key)
		}
	}
	return nil
}

func reflectLiveValue(got, want any) bool { return fmt.Sprint(got) == fmt.Sprint(want) }

func runWorkflowContextLiveMetadata(
	ctx context.Context, executable string, layout workflowContextLiveLayout, endpoint string, args ...string,
) ([]byte, error) {
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Dir, cmd.Env = layout.workspace, layout.env()
	if _, err := configureWorkflowContextLiveSandbox(cmd, endpoint); err != nil {
		return nil, errors.New("sandbox-unavailable")
	}
	output, err := cmd.Output()
	if err != nil {
		return nil, errors.New("metadata-exit")
	}
	return output, nil
}

func workflowContextLiveRootCount(path string) int {
	if _, err := os.Lstat(path); os.IsNotExist(err) {
		return 0
	}
	return 1
}

func liveReason(err error) string {
	if err == nil {
		return "none"
	}
	return strings.ReplaceAll(err.Error(), "\n", " ")
}
