package cli

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

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
