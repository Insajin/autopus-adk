package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/processprobe"
	"gopkg.in/yaml.v3"
)

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: each effective-setting probe is capped at five seconds and 16 KiB.
func verifyWorkflowContextManagedRPCConfig(
	ctx context.Context, options WorkflowContextManagedRPCOptions,
) error {
	wants := map[string]any{"compaction.autoContinue": false}
	if len(options.Prompts) != 0 {
		if err := verifyWorkflowContextManagedRPCModelConfig(options); err != nil {
			return err
		}
		wants = map[string]any{
			"compaction.enabled": true, "compaction.strategy": "snapcompact",
			"compaction.autoContinue": false, "memory.backend": "off",
			"extensions": []any{}, "mcp.enableProjectConfig": false,
			"commands.enableClaudeProject": false, "commands.enableClaudeUser": false,
			"commands.enableOpencodeProject": false, "commands.enableOpencodeUser": false,
			"skills.enableAgentsProject": true, "skills.enableAgentsUser": false,
			"skills.enableClaudeProject": false, "skills.enableClaudeUser": false,
			"skills.enableCodexUser": false, "skills.enablePiProject": false,
			"skills.enablePiUser": false,
		}
	}
	for key, want := range wants {
		if err := verifyWorkflowContextManagedRPCSetting(ctx, options, key, want); err != nil {
			return err
		}
	}
	return nil
}

func verifyWorkflowContextManagedRPCModelConfig(options WorkflowContextManagedRPCOptions) error {
	providerID, modelID, ok := strings.Cut(options.Model, "/")
	endpoint, err := url.Parse(options.AllowedEndpoint)
	if !ok || providerID == "" || modelID == "" || err != nil {
		return errors.New("managed OMP product model authority is invalid")
	}
	data, err := os.ReadFile(filepath.Join(options.RuntimeRoot, "models.yml"))
	if err != nil || len(data) > 1<<20 {
		return errors.New("managed OMP product model config is unavailable")
	}
	var config struct {
		Providers map[string]struct {
			BaseURL string `yaml:"baseUrl"`
			Auth    string `yaml:"auth"`
			Models  []struct {
				ID string `yaml:"id"`
			} `yaml:"models"`
		} `yaml:"providers"`
	}
	if yaml.Unmarshal(data, &config) != nil || len(config.Providers) != 1 {
		return errors.New("managed OMP product model config is invalid")
	}
	provider, found := config.Providers[providerID]
	wantBase := strings.TrimSuffix(endpoint.String(), "/") + "/v1"
	if !found || provider.Auth != "none" || provider.BaseURL != wantBase || len(provider.Models) != 1 ||
		provider.Models[0].ID != modelID {
		return errors.New("managed OMP product model config does not match loopback authority")
	}
	return nil
}

func verifyWorkflowContextManagedRPCSetting(
	ctx context.Context, options WorkflowContextManagedRPCOptions, key string, want any,
) error {
	probeCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(probeCtx, options.Executable, "config", "get", key, "--json")
	cmd.Dir = options.Workspace
	if len(options.Prompts) != 0 {
		cmd.Dir = options.ProjectDir
	}
	cmd.Env = append([]string(nil), options.Environment...)
	if _, err := configureWorkflowContextManagedRPCSandbox(cmd, options.AllowedEndpoint); err != nil {
		return err
	}
	output, err := processprobe.OutputLimited(cmd, 16<<10)
	if err != nil {
		return fmt.Errorf("read managed OMP setting %s: %w", key, err)
	}
	var readback struct {
		Key   string `json:"key"`
		Value any    `json:"value"`
	}
	if json.Unmarshal(output, &readback) != nil || readback.Key != key || !reflect.DeepEqual(readback.Value, want) {
		return errors.New("managed OMP effective configuration is not admission-safe")
	}
	return nil
}
