package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

// stepSaveAndCheckProviders saves worker/MCP config and reports provider status.
func stepSaveAndCheckProviders(cmd *cobra.Command, backendURL, token string, ws *setup.Workspace) error {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Step 3/3: 설정 저장 및 프로바이더 확인")

	_ = setup.SaveProgress(3)
	setupToken := token
	if token != "" {
		fmt.Fprintln(out, "  ✓ 워커 인증은 JWT/refresh 토큰으로 유지됩니다")
	}

	workDir, err := os.Getwd()
	if err != nil || workDir == "" {
		workDir = "."
	}

	workerCfg := setup.WorkerConfig{
		BackendURL:        backendURL,
		WorkspaceID:       ws.ID,
		WorkDir:           workDir,
		WorktreeIsolation: true,
		KnowledgeDir:      workDir,
		Concurrency:       1,
	}

	fmt.Fprintln(out, "  ✓ 자동 knowledge file sync는 더 이상 설정하지 않습니다")

	if setupToken != "" {
		agents, err := setup.FetchWorkspaceAgents(backendURL, setupToken, ws.ID)
		if err != nil {
			fmt.Fprintf(out, "  ⚠ memory agent 조회 실패 (수동 설정 필요): %v\n", err)
		} else if memoryAgentID := setup.SelectMemoryAgentID(agents); memoryAgentID != "" {
			workerCfg.MemoryAgentID = memoryAgentID
			fmt.Fprintf(out, "  ✓ Memory agent 연결: %s\n", memoryAgentID)
		} else {
			fmt.Fprintln(out, "  ⚠ worker tier agent 없음 — memory_agent_id 수동 설정 필요")
		}
	}

	providers := setup.DetectProviders()
	for _, p := range providers {
		if p.Installed {
			workerCfg.Providers = append(workerCfg.Providers, p.Name)
		}
	}

	if err := setup.SaveWorkerConfig(workerCfg); err != nil {
		return fmt.Errorf("save worker config: %w", err)
	}
	fmt.Fprintf(out, "  ✓ Worker config 저장: %s\n", setup.DefaultWorkerConfigPath())

	mcpCfg, err := setup.GenerateMCPConfig(setup.MCPConfigOptions{
		BackendURL:  backendURL,
		AuthToken:   token,
		WorkspaceID: ws.ID,
		OutputPath:  setup.DefaultMCPConfigPath(),
	})
	if err != nil {
		return fmt.Errorf("generate MCP config: %w", err)
	}

	if err := setup.WriteMCPConfig(mcpCfg, setup.DefaultMCPConfigPath()); err != nil {
		return fmt.Errorf("write MCP config: %w", err)
	}
	fmt.Fprintf(out, "  ✓ MCP config 저장: %s\n", setup.DefaultMCPConfigPath())

	fmt.Fprintln(out)
	fmt.Fprintln(out, "  프로바이더 상태:")
	for _, p := range providers {
		status := "❌ 미설치"
		if p.Installed {
			authed, guide := setup.CheckProviderAuth(p.Name)
			if authed {
				status = fmt.Sprintf("✅ %s (인증됨)", p.Version)
			} else {
				status = fmt.Sprintf("⚠️  %s (인증 필요: %s)", p.Version, guide)
			}
		}
		fmt.Fprintf(out, "    %-10s %s\n", p.Name, status)
	}

	_ = setup.ClearProgress()
	return nil
}
