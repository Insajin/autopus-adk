package cli

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"time"
)

// runCanaryRemoteBrowser probes the deployed frontend from the project's own
// node tree. It used to require an Autopus/frontend directory and probe the
// Autopus routes /login, /docs and /marketplace, so it could only ever run for
// one repository; every other project got a bare SKIPPED with a reason that
// described a directory it never had.
func runCanaryRemoteBrowser(ctx context.Context, projectDir string, targets canaryTargets, result *canaryResult) string {
	if targets.FrontendURL == "" {
		result.Skipped = append(result.Skipped, canarySkippedCheck{"browser", "no frontend URL supplied (pass --frontend-url or --url)"})
		return "SKIPPED"
	}
	browserDir, reason := canaryBrowserTarget(projectDir, canaryBuildUnits(projectDir))
	if reason != "" {
		result.Skipped = append(result.Skipped, canarySkippedCheck{"browser", reason})
		return "SKIPPED"
	}
	run := runCanaryBrowserScript(ctx, browserDir, strings.TrimRight(targets.FrontendURL, "/"), targets.BrowserPaths)
	run.ID = "browser-remote"
	result.Targets = append(result.Targets, run)
	if run.Status != "PASS" {
		return "FAIL"
	}
	return "PASS"
}

// canaryBrowserScript probes each requested path. Paths arrive as JSON through
// the environment rather than being interpolated into the script body so a path
// can never terminate the string literal and inject code.
const canaryBrowserScript = `
const { chromium } = require('playwright');
const base = process.env.AUTOPUS_CANARY_BASE_URL;
const targets = JSON.parse(process.env.AUTOPUS_CANARY_PATHS);
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

func runCanaryBrowserScript(ctx context.Context, dir, baseURL string, paths []string) canaryTargetResult {
	encodedPaths, err := json.Marshal(paths)
	if err != nil {
		return canaryTargetResult{ID: "browser-remote", Status: "FAIL", Detail: err.Error()}
	}
	display := "node playwright probe " + baseURL + " " + strings.Join(paths, ",")
	timeoutCtx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	cmd := exec.CommandContext(timeoutCtx, "node", "-e", canaryBrowserScript) //nolint:gosec
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"AUTOPUS_CANARY_BASE_URL="+baseURL,
		"AUTOPUS_CANARY_PATHS="+string(encodedPaths),
	)
	output, runErr := cmd.CombinedOutput()
	if timeoutCtx.Err() == context.DeadlineExceeded {
		return canaryTargetResult{ID: "browser-remote", Command: display, Status: "FAIL", Detail: "timed out"}
	}
	if runErr != nil {
		return canaryCheckFailure("browser-remote", display, output, runErr)
	}
	return canaryTargetResult{ID: "browser-remote", Command: display, Status: "PASS"}
}
