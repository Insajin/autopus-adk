package cli

import (
	"bytes"
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func runCanaryLocalBrowser(ctx context.Context, projectDir string, result *canaryResult) string {
	frontendDir := filepath.Join(projectDir, "Autopus", "frontend")
	if _, err := os.Stat(filepath.Join(frontendDir, "package.json")); err != nil {
		result.Skipped = append(result.Skipped, canarySkippedCheck{"browser", "frontend package.json not found"})
		return "SKIPPED"
	}
	port, err := reserveLocalPort()
	if err != nil {
		result.Targets = append(result.Targets, canaryTargetResult{ID: "browser", Status: "FAIL", Detail: err.Error()})
		return "FAIL"
	}
	baseURL := "http://127.0.0.1:" + strconv.Itoa(port)
	server, serverLog, err := startNextServer(ctx, frontendDir, port)
	if err != nil {
		result.Targets = append(result.Targets, canaryTargetResult{ID: "browser", Status: "FAIL", Detail: err.Error()})
		return "FAIL"
	}
	defer stopProcess(server)

	if !waitForCanaryURL(ctx, baseURL+"/login", 20*time.Second) {
		detail := "frontend server did not become ready"
		if serverLog.Len() > 0 {
			detail += ": " + serverLog.String()
		}
		result.Targets = append(result.Targets, canaryTargetResult{ID: "browser", Status: "FAIL", Detail: detail})
		return "FAIL"
	}
	run := runCanaryBrowserScript(ctx, frontendDir, baseURL)
	result.Targets = append(result.Targets, run)
	if run.Status != "PASS" {
		return "FAIL"
	}
	return "PASS"
}

func runCanaryRemoteBrowser(ctx context.Context, projectDir string, baseURL string, result *canaryResult) string {
	frontendDir := filepath.Join(projectDir, "Autopus", "frontend")
	if _, err := os.Stat(filepath.Join(frontendDir, "package.json")); err != nil {
		result.Skipped = append(result.Skipped, canarySkippedCheck{"browser", "frontend package.json not found"})
		return "SKIPPED"
	}
	run := runCanaryBrowserScript(ctx, frontendDir, strings.TrimRight(baseURL, "/"))
	run.ID = "browser-staging"
	result.Targets = append(result.Targets, run)
	if run.Status != "PASS" {
		return "FAIL"
	}
	return "PASS"
}

func reserveLocalPort() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("resolve local port")
	}
	return addr.Port, nil
}

func startNextServer(ctx context.Context, dir string, port int) (*exec.Cmd, *bytes.Buffer, error) {
	cmd := exec.CommandContext(ctx, "npm", "run", "start", "--", "-p", strconv.Itoa(port), "-H", "127.0.0.1") //nolint:gosec
	cmd.Dir = dir
	var log bytes.Buffer
	cmd.Stdout = &log
	cmd.Stderr = &log
	if err := cmd.Start(); err != nil {
		return nil, &log, err
	}
	return cmd, &log, nil
}

func waitForCanaryURL(ctx context.Context, url string, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if canaryHTTPCheck(ctx, url) {
			return true
		}
		time.Sleep(250 * time.Millisecond)
	}
	return false
}

func runCanaryBrowserScript(ctx context.Context, dir, baseURL string) canaryTargetResult {
	script := `
const { chromium } = require('playwright');
const base = process.env.AUTOPUS_CANARY_BASE_URL;
const targets = ['/login', '/docs', '/marketplace'];
(async () => {
  const browser = await chromium.launch({ headless: true });
  const failures = [];
  for (const path of targets) {
    const page = await browser.newPage();
    const consoleErrors = [];
    const pageErrors = [];
    page.on('console', msg => { if (msg.type() === 'error') consoleErrors.push(msg.text()); });
    page.on('pageerror', err => pageErrors.push(err.message));
    try {
      const response = await page.goto(base + path, { waitUntil: 'networkidle', timeout: 15000 });
      const body = (await page.locator('body').innerText({ timeout: 5000 })).trim();
      if (!response || response.status() >= 400 || body.length === 0 || consoleErrors.length || pageErrors.length) {
        failures.push({ path, status: response && response.status(), consoleErrors, pageErrors, bodyLength: body.length });
      }
    } catch (err) {
      failures.push({ path, error: err.message, consoleErrors, pageErrors });
    } finally {
      await page.close();
    }
  }
  await browser.close();
  if (failures.length) {
    console.error(JSON.stringify(failures));
    process.exit(1);
  }
})().catch(err => { console.error(err.stack || err.message); process.exit(1); });
`
	timeoutCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, "node", "-e", script) //nolint:gosec
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "AUTOPUS_CANARY_BASE_URL="+baseURL)
	output, err := cmd.CombinedOutput()
	if timeoutCtx.Err() == context.DeadlineExceeded {
		return canaryTargetResult{ID: "browser-local", Status: "FAIL", Detail: "timed out"}
	}
	if err != nil {
		return canaryTargetResult{ID: "browser-local", Status: "FAIL", Detail: string(output)}
	}
	return canaryTargetResult{ID: "browser-local", Status: "PASS"}
}

func stopProcess(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	done := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		close(done)
	}()
	_ = cmd.Process.Signal(os.Interrupt)
	select {
	case <-done:
		return
	case <-time.After(2 * time.Second):
	}
	_ = cmd.Process.Kill()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
	}
}
