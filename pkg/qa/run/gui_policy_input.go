package run

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

const (
	guiPolicyPathEnv          = "AUTOPUS_QAMESH_GUI_POLICY_PATH"
	guiAllowedOriginsEnv      = "AUTOPUS_QAMESH_GUI_ALLOWED_ORIGINS"
	guiForbiddenActionsEnv    = "AUTOPUS_QAMESH_GUI_FORBIDDEN_ACTIONS"
	guiGuardReadyPathEnv      = "AUTOPUS_QAMESH_GUI_GUARD_READY_PATH"
	guiPolicyRuntimeCheckID   = "gui-policy-runtime"
	guiPolicyRuntimeCheckType = "gui_runtime_policy"
	guiGuardPreflightTimeout  = 15 * time.Second
)

type guiRuntimeInput struct {
	Env            []string
	GuardReadyPath string
}

func prepareGUIPolicyInput(pack journey.Pack, artifactDir string) (guiRuntimeInput, error) {
	if pack.Adapter.ID != "gui-explore" {
		return guiRuntimeInput{}, nil
	}
	allowed := cleanedList(pack.GUI.AllowedOrigins)
	forbidden := cleanedList(pack.GUI.ForbiddenActions)
	if len(allowed) == 0 || len(forbidden) == 0 {
		return guiRuntimeInput{}, fmt.Errorf("gui.allowed_origins and gui.forbidden_actions are required for runtime policy")
	}
	policy := map[string]any{
		"schema_version":     "autopus.qamesh.gui_policy.v1",
		"allowed_origins":    allowed,
		"forbidden_actions":  forbidden,
		"selector_strategy":  strings.TrimSpace(pack.GUI.SelectorStrategy),
		"network_policy":     pack.GUI.NetworkPolicy,
		"artifact_retention": pack.GUI.ArtifactRetention,
		"screen_matrix":      pack.GUI.ScreenMatrix,
	}
	body, err := json.MarshalIndent(policy, "", "  ")
	if err != nil {
		return guiRuntimeInput{}, err
	}
	path := filepath.Join(artifactDir, "gui-policy.json")
	guardPath := filepath.Join(artifactDir, "gui-policy-guard.cjs")
	readyPath := filepath.Join(artifactDir, "gui-policy-guard-ready.json")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return guiRuntimeInput{}, err
	}
	if err := os.WriteFile(path, append(body, '\n'), 0o644); err != nil {
		return guiRuntimeInput{}, err
	}
	if err := os.WriteFile(guardPath, []byte(guiPolicyGuardScript()), 0o644); err != nil {
		return guiRuntimeInput{}, err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return guiRuntimeInput{}, err
	}
	absGuardPath, err := filepath.Abs(guardPath)
	if err != nil {
		return guiRuntimeInput{}, err
	}
	absReadyPath, err := filepath.Abs(readyPath)
	if err != nil {
		return guiRuntimeInput{}, err
	}
	return guiRuntimeInput{
		GuardReadyPath: absReadyPath,
		Env: []string{
			guiPolicyPathEnv + "=" + absPath,
			guiAllowedOriginsEnv + "=" + strings.Join(allowed, ","),
			guiForbiddenActionsEnv + "=" + strings.Join(forbidden, ","),
			guiGuardReadyPathEnv + "=" + absReadyPath,
			"NODE_OPTIONS=" + appendNodeRequire(os.Getenv("NODE_OPTIONS"), absGuardPath),
		},
	}, nil
}

func verifyGUIGuardPreflight(ctx context.Context, dir string, env []string, input guiRuntimeInput, args []string) error {
	if input.GuardReadyPath == "" {
		return nil
	}
	if !guiCommandSupportsNodeGuard(args) {
		return fmt.Errorf("gui runtime guard cannot be installed for command %q", firstArg(args))
	}
	_ = os.Remove(input.GuardReadyPath)
	preflightCtx, cancel := context.WithTimeout(ctx, guiGuardPreflightTimeout)
	defer cancel()
	script := `const fs = require("fs"); process.exit(fs.existsSync(process.env.AUTOPUS_QAMESH_GUI_GUARD_READY_PATH) ? 0 : 42);`
	cmd := exec.CommandContext(preflightCtx, "node", "-e", script)
	cmd.Dir = dir
	cmd.Env = env
	if err := cmd.Run(); err != nil {
		if preflightCtx.Err() == context.DeadlineExceeded {
			return fmt.Errorf("gui runtime guard preflight timed out")
		}
		return fmt.Errorf("gui runtime guard preflight failed")
	}
	if _, err := os.Stat(input.GuardReadyPath); err != nil {
		return fmt.Errorf("gui runtime guard preflight did not confirm installation")
	}
	return nil
}

func guiCommandSupportsNodeGuard(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch filepath.Base(args[0]) {
	case "node", "npm", "npx", "pnpm", "yarn", "yarnpkg", "playwright":
		return true
	default:
		return false
	}
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

func appendNodeRequire(existing, path string) string {
	requireOpt := "--require=" + path
	if strings.TrimSpace(existing) == "" {
		return requireOpt
	}
	return existing + " " + requireOpt
}

func appendEnvOverrides(base, overrides []string) []string {
	if len(overrides) == 0 {
		return base
	}
	names := map[string]bool{}
	for _, item := range overrides {
		if name, _, ok := strings.Cut(item, "="); ok {
			names[name] = true
		}
	}
	env := make([]string, 0, len(base)+len(overrides))
	for _, item := range base {
		name, _, ok := strings.Cut(item, "=")
		if ok && names[name] {
			continue
		}
		env = append(env, item)
	}
	return append(env, overrides...)
}
