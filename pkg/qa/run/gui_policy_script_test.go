package run

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGUIPolicyGuardAllowsRelativePlaywrightBaseURL(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("node is required for GUI policy guard preload test")
	}

	dir := t.TempDir()
	moduleDir := filepath.Join(dir, "node_modules", "playwright")
	require.NoError(t, os.MkdirAll(moduleDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(moduleDir, "index.js"), []byte(`
exports.chromium = {
  launch: async () => ({
    newPage: async () => ({
      goto: async (target) => ({ target }),
      locator: () => ({ click: async () => {} }),
      getByRole: () => ({ click: async () => {} })
    })
  })
};
`), 0o644))

	guardPath := filepath.Join(dir, "gui-policy-guard.cjs")
	require.NoError(t, os.WriteFile(guardPath, []byte(guiPolicyGuardScript()), 0o644))
	script := `
const { chromium } = require("playwright");
(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage();
  await page.goto("/autopus-adk");
  try {
    await page.goto("https://evil.invalid/pay");
    throw new Error("expected off-origin navigation to fail");
  } catch (error) {
    if (!String(error.message).includes("AUTOPUS_QAMESH_GUI_OFF_ORIGIN:https://evil.invalid")) {
      throw error;
    }
  }
})().catch((error) => {
  console.error(error && error.stack ? error.stack : error);
  process.exit(1);
});
`
	cmd := exec.Command(node, "-e", script)
	cmd.Dir = dir
	cmd.Env = appendEnvOverrides(os.Environ(), []string{
		"NODE_OPTIONS=--require=" + guardPath,
		"AUTOPUS_QAMESH_GUI_ALLOWED_ORIGINS=https://staging.autopus.co",
		"AUTOPUS_QAMESH_GUI_FORBIDDEN_ACTIONS=mutation,payment,email_send",
		"PLAYWRIGHT_BASE_URL=https://staging.autopus.co",
	})

	output, err := cmd.CombinedOutput()
	require.NoError(t, err, strings.TrimSpace(string(output)))
}
