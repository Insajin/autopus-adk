package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/worker/setup"
)

const defaultBackendURL = "https://api.autopus.co"

// userError wraps a technical error with a Korean context label and friendly message.
func userError(context string, err error) error {
	return fmt.Errorf("%s: %s", context, setup.HumanError(err))
}

// runWorkerSetup executes the 3-step setup wizard.
// Non-interactive modes (for agents/CI):
//
//   - preToken + preWorkspaceID: use JWT token, skip browser OAuth and workspace prompt
//
//   - preAPIKey + preWorkspaceID: use Worker API Key, skip browser OAuth and workspace prompt
//
//     Step 1: Device Auth OAuth (PKCE) → Autopus server login  [skipped if preToken/preAPIKey != ""]
//     Step 2: Workspace selection                              [skipped if preWorkspaceID != ""]
//     Step 3: Provider auth check (claude/codex/gemini)
func runWorkerSetup(cmd *cobra.Command, backendURL, preToken, preWorkspaceID, preAPIKey string) error {
	out := cmd.OutOrStdout()
	if backendURL == "" {
		backendURL = defaultBackendURL
	}

	fmt.Fprintln(out, "🐙 Autopus Worker Setup")
	fmt.Fprintln(out, "───────────────────────────────")
	fmt.Fprintln(out)

	// Step 1: Auth — bypass if token or API key provided.
	var token string
	switch {
	case preAPIKey != "":
		fmt.Fprintln(out, "Step 1/3: Autopus 서버 인증 (Worker API Key 사용 — 비대화형)")
		if err := setup.SaveAPIKeyCredentials(preAPIKey, backendURL); err != nil {
			return userError("API 키 저장", err)
		}
		fmt.Fprintln(out, "  ✓ Worker API Key 저장 완료")
		// token stays empty — API key is used for A2A WebSocket auth directly.
	case preToken != "":
		fmt.Fprintln(out, "Step 1/3: Autopus 서버 인증 (토큰 사용 — 비대화형)")
		if err := setup.SaveCredentials(map[string]any{
			"access_token": preToken,
			"backend_url":  backendURL,
		}); err != nil {
			return userError("토큰 저장", err)
		}
		fmt.Fprintln(out, "  ✓ 인증 토큰 저장 완료")
		token = preToken
	default:
		fmt.Fprintln(out, "Worker는 Autopus 서버에서 작업을 받아 자동으로 실행하는")
		fmt.Fprintln(out, "백그라운드 서비스입니다. 설정은 약 2분 정도 소요됩니다.")
		fmt.Fprintln(out)
		var err error
		token, err = stepDeviceAuth(cmd, backendURL)
		if err != nil {
			return userError("인증", err)
		}
	}

	// Step 2: Workspace — bypass if ID provided.
	// When using an API key without a workspace ID, we still need it for the worker config.
	var workspace *setup.Workspace
	if preWorkspaceID != "" {
		fmt.Fprintln(out)
		fmt.Fprintln(out, "Step 2/3: 워크스페이스 선택 (ID 직접 사용)")
		if token != "" {
			ws, err := setup.FindWorkspaceByID(backendURL, token, preWorkspaceID)
			if err != nil {
				return userError("워크스페이스 조회", err)
			}
			fmt.Fprintf(out, "  ✓ 워크스페이스: %s (%s)\n", ws.Name, ws.ID)
			workspace = ws
		} else {
			// API key mode: workspace lookup requires auth — use ID directly.
			workspace = &setup.Workspace{ID: preWorkspaceID, Name: preWorkspaceID}
			fmt.Fprintf(out, "  ✓ 워크스페이스 ID: %s\n", preWorkspaceID)
		}
	} else {
		var err error
		workspace, err = stepSelectWorkspace(cmd, backendURL, token)
		if err != nil {
			return userError("워크스페이스 선택", err)
		}
	}

	// Step 3: Save config and check providers
	if err := stepSaveAndCheckProviders(cmd, backendURL, token, workspace); err != nil {
		return userError("설정 저장", err)
	}

	fmt.Fprintln(out)
	fmt.Fprintln(out, "🐙 Setup complete!")
	fmt.Fprintf(out, "   Worker config: %s\n", setup.DefaultWorkerConfigPath())
	fmt.Fprintf(out, "   MCP config:    %s\n", setup.DefaultMCPConfigPath())
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Next: `auto worker start` to run the worker.")
	return nil
}

// stepDeviceAuth performs Device Authorization with PKCE.
func stepDeviceAuth(cmd *cobra.Command, backendURL string) (string, error) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out, "Step 1/3: Autopus 서버 인증")

	_ = setup.SaveProgress(1)

	verifier, _, err := setup.GeneratePKCE()
	if err != nil {
		return "", fmt.Errorf("%s", setup.HumanError(err))
	}

	dc, err := setup.RequestDeviceCode(backendURL, verifier)
	if err != nil {
		return "", fmt.Errorf("request device code: %w", err)
	}

	// Use the complete URI (includes code) if available, otherwise the base URI.
	verifyURL := dc.VerificationURIComplete
	if verifyURL == "" {
		verifyURL = dc.VerificationURI
	}

	fmt.Fprintln(out)
	fmt.Fprintf(out, "  브라우저에서 아래 URL을 열고 코드를 입력하세요:\n")
	fmt.Fprintf(out, "  URL:  %s\n", verifyURL)
	fmt.Fprintf(out, "  Code: %s\n", dc.UserCode)
	fmt.Fprintln(out)

	// Try to open browser automatically.
	if verifyURL == "" {
		fmt.Fprintf(out, "  ⚠ 인증 URL이 비어있습니다. 백엔드 연결을 확인하세요.\n")
		return "", fmt.Errorf("empty verification URI from backend")
	}
	if err := setup.OpenBrowser(verifyURL); err != nil {
		fmt.Fprintf(out, "  브라우저를 자동으로 열 수 없습니다. 위 URL을 복사하여 브라우저에 붙여넣으세요.\n")
	} else {
		fmt.Fprintf(out, "  브라우저가 열렸습니다. 인증을 완료해주세요.\n")
	}

	fmt.Fprintf(out, "  인증 대기 중...")

	// Start countdown display goroutine
	done := make(chan struct{})
	go func() {
		deadline := time.Now().Add(time.Duration(dc.ExpiresIn) * time.Second)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				remaining := time.Until(deadline)
				if remaining < 0 {
					remaining = 0
				}
				mins := int(remaining.Minutes())
				secs := int(remaining.Seconds()) % 60
				fmt.Fprintf(out, "\r  인증 대기 중... (남은 시간: %d분 %02d초)", mins, secs)
			}
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(dc.ExpiresIn)*time.Second)
	defer cancel()

	tokenResp, err := setup.PollForToken(ctx, backendURL, dc.DeviceCode, verifier, dc.Interval)
	close(done)
	fmt.Fprintln(out) // newline after countdown
	if err != nil {
		return "", fmt.Errorf("%s", setup.HumanError(err))
	}

	// Save credentials.
	creds := map[string]any{
		"access_token":  tokenResp.AccessToken,
		"refresh_token": tokenResp.RefreshToken,
		"backend_url":   backendURL,
		"created_at":    time.Now().Format(time.RFC3339),
	}
	if err := setup.SaveCredentials(creds); err != nil {
		return "", fmt.Errorf("save credentials: %w", err)
	}

	fmt.Fprintln(out, "  ✓ 인증 성공")
	return tokenResp.AccessToken, nil
}

// stepSelectWorkspace fetches workspaces and lets the user pick one.
func stepSelectWorkspace(cmd *cobra.Command, backendURL, token string) (*setup.Workspace, error) {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Step 2/3: 워크스페이스 선택")

	_ = setup.SaveProgress(2)

	workspaces, err := setup.FetchWorkspaces(backendURL, token)
	if err != nil {
		return nil, fmt.Errorf("fetch workspaces: %w", err)
	}

	ws, err := setup.SelectWorkspace(workspaces)
	if err != nil {
		return nil, err
	}

	fmt.Fprintf(out, "  ✓ 워크스페이스: %s (%s)\n", ws.Name, ws.ID)
	return ws, nil
}

// stepSaveAndCheckProviders saves worker/MCP config and reports provider status.
func stepSaveAndCheckProviders(cmd *cobra.Command, backendURL, token string, ws *setup.Workspace) error {
	out := cmd.OutOrStdout()
	fmt.Fprintln(out)
	fmt.Fprintln(out, "Step 3/3: 설정 저장 및 프로바이더 확인")

	_ = setup.SaveProgress(3)
	setupToken := token

	// Exchange JWT for a long-lived Worker API Key for A2A WebSocket auth.
	// JWT tokens are not accepted by the A2A endpoint — only acos_worker_ keys are.
	if token != "" {
		apiKey, err := setup.CreateWorkerAPIKey(backendURL, token, ws.ID)
		if err != nil {
			fmt.Fprintf(out, "  ⚠ Worker API Key 생성 실패 (JWT로 계속 진행): %v\n", err)
		} else {
			if err := setup.SaveAPIKeyCredentials(apiKey, backendURL); err != nil {
				return fmt.Errorf("save worker API key: %w", err)
			}
			fmt.Fprintln(out, "  ✓ Worker API Key 발급 및 저장 완료")
			// Clear token so MCP config uses the API key path too.
			token = apiKey
		}
	}

	// Save worker config.
	workerCfg := setup.WorkerConfig{
		BackendURL:  backendURL,
		WorkspaceID: ws.ID,
		Concurrency: 3,
	}

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

	// Detect installed providers and add to config.
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

	// Generate and save MCP config.
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

	// Report provider status.
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
