package omp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

const ompContextBridgeSecurityHarness = `
type SmokeHandler = (event: unknown, context: unknown) => unknown;

async function runSecurityCase(
  label: string,
  hashes: Array<string | undefined>,
  notify: () => unknown,
): Promise<void> {
  const names = [
    "AUTOPUS_OMP_CONTEXT_BINDING_HASH",
    "AUTOPUS_OMP_CONTEXT_OPTIONS_HASH",
    "AUTOPUS_OMP_CONTEXT_SESSION_HASH",
  ];
  for (let index = 0; index < names.length; index++) {
    const value = hashes[index];
    if (value === undefined) {
      delete process.env[names[index]];
    } else {
      process.env[names[index]] = value;
    }
  }
  const handlers = new Map<string, SmokeHandler>();
  autopusContextBridge({
    on(eventName: string, handler: SmokeHandler): void {
      handlers.set(eventName, handler);
    },
  } as any);
  const pre = handlers.get("session_before_compact");
  if (pre === undefined) {
    throw new Error(label + ": pre canceller missing");
  }
  const result = await Promise.race([
    Promise.resolve(pre(undefined, { ui: { notify } })),
    new Promise((_, reject) => setTimeout(() => reject(new Error(label + ": timeout")), 1000)),
  ]);
  if ((result as { cancel?: boolean } | undefined)?.cancel !== true) {
    throw new Error(label + ": fail-open result");
  }
  const valid = hashes.every((value) => /^sha256:[0-9a-f]{64}$/.test(value ?? ""));
  if (handlers.has("session_compact") !== valid) {
    throw new Error(label + ": post handler registration mismatch");
  }
}

const good = "sha256:" + "a".repeat(64);
let invalidNotifyCalls = 0;
await runSecurityCase("missing", [good, good, undefined], () => { invalidNotifyCalls++; });
await runSecurityCase("uppercase", [good, "sha256:" + "B".repeat(64), good], () => { invalidNotifyCalls++; });
await runSecurityCase("throw", [good, good, good], () => { throw new Error("throw"); });
await runSecurityCase("reject", [good, good, good], () => Promise.reject(new Error("reject")));
await runSecurityCase("hang", [good, good, good], () => new Promise<void>(() => {}));
if (invalidNotifyCalls !== 0) {
  throw new Error("invalid hashes emitted a notification");
}
console.log(JSON.stringify({ cases: 5, invalid_notify_calls: invalidNotifyCalls, pre_cancel: true }));
`

func TestOMPContextBridgeSecurity_PreAlwaysCancelsNotifyFailures(t *testing.T) {
	bun, err := exec.LookPath("bun")
	if err != nil {
		t.Skip("bun is required to execute the generated TypeScript security contract")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bun, "run", "-")
	cmd.Env = []string{"HOME=" + t.TempDir(), "PATH=" + os.Getenv("PATH")}
	cmd.Stdin = strings.NewReader(ompContextBridgeSource + ompContextBridgeSecurityHarness)
	output, err := cmd.Output()
	if err != nil {
		t.Fatalf("generated bridge security harness failed: %v", err)
	}
	var receipt struct {
		Cases              int  `json:"cases"`
		InvalidNotifyCalls int  `json:"invalid_notify_calls"`
		PreCancel          bool `json:"pre_cancel"`
	}
	if json.Unmarshal(output, &receipt) != nil || receipt.Cases != 5 ||
		receipt.InvalidNotifyCalls != 0 || !receipt.PreCancel {
		t.Fatalf("unexpected bridge security receipt: %s", output)
	}
}
