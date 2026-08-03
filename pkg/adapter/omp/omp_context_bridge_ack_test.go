package omp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const ompContextBridgeACKHarness = `
type ACKHandler = (event: unknown, context: unknown) => unknown;
type ConfirmCall = { title: string; message: string };
type CaseReceipt = {
  state: "cancel" | "undefined" | "other";
  confirm_calls: number;
  notify_calls: number;
  send_message_calls: number;
};

const hashes = ["a", "b", "c", "d"].map((value) => "sha256:" + value.repeat(64));
const names = [
  "AUTOPUS_OMP_CONTEXT_BINDING_HASH",
  "AUTOPUS_OMP_CONTEXT_OPTIONS_HASH",
  "AUTOPUS_OMP_CONTEXT_SESSION_HASH",
  "AUTOPUS_OMP_CONTEXT_NONCE_HASH",
];
for (let index = 0; index < names.length; index++) process.env[names[index]] = hashes[index];
process.env.AUTOPUS_OMP_CONTEXT_ACK_TIMEOUT_MS = "50";

function stateOf(value: unknown): CaseReceipt["state"] {
  if ((value as { cancel?: boolean } | undefined)?.cancel === true) return "cancel";
  return value === undefined ? "undefined" : "other";
}

function installBridge() {
  const handlers = new Map<string, ACKHandler>();
  let sendMessageCalls = 0;
  autopusContextBridge({
    on(name: string, handler: ACKHandler) { handlers.set(name, handler); },
    sendMessage() { sendMessageCalls++; },
  } as any);
  return { handlers, sendMessageCalls: () => sendMessageCalls };
}

async function invoke(handler: ACKHandler, context: unknown, label: string): Promise<unknown> {
  return await Promise.race([
    Promise.resolve(handler(undefined, context)),
    new Promise((_, reject) => setTimeout(() => reject(new Error(label + ": bridge timeout")), 1000)),
  ]);
}

async function runPreCase(label: string, confirm: (() => unknown) | undefined): Promise<CaseReceipt> {
  const bridge = installBridge();
  const pre = bridge.handlers.get("session_before_compact");
  if (pre === undefined) throw new Error(label + ": pre handler missing");
  let confirmCalls = 0;
  let notifyCalls = 0;
  const ui: Record<string, unknown> = { notify() { notifyCalls++; } };
  if (confirm !== undefined) {
    ui.confirm = () => {
      confirmCalls++;
      return confirm();
    };
  }
  const result = await invoke(pre, { ui }, label);
  return {
    state: stateOf(result),
    confirm_calls: confirmCalls,
    notify_calls: notifyCalls,
    send_message_calls: bridge.sendMessageCalls(),
  };
}

async function runPostCase(label: string, confirm: () => unknown): Promise<CaseReceipt> {
  const bridge = installBridge();
  const post = bridge.handlers.get("session_compact");
  if (post === undefined) throw new Error(label + ": post handler missing");
  let confirmCalls = 0;
  let notifyCalls = 0;
  const result = await invoke(post, { ui: {
    confirm() { confirmCalls++; return confirm(); },
    notify() { notifyCalls++; },
  } }, label);
  return {
    state: stateOf(result),
    confirm_calls: confirmCalls,
    notify_calls: notifyCalls,
    send_message_calls: bridge.sendMessageCalls(),
  };
}

async function runInvalidTimeoutCase(value: string): Promise<CaseReceipt & { has_post: boolean }> {
  process.env.AUTOPUS_OMP_CONTEXT_ACK_TIMEOUT_MS = value;
  const bridge = installBridge();
  const pre = bridge.handlers.get("session_before_compact");
  if (pre === undefined) throw new Error("invalid timeout: pre handler missing");
  let confirmCalls = 0;
  const result = await invoke(pre, { ui: {
    confirm() { confirmCalls++; return true; },
    notify() { throw new Error("invalid timeout used notify"); },
  } }, "invalid-timeout-" + value);
  return {
    state: stateOf(result),
    confirm_calls: confirmCalls,
    notify_calls: 0,
    send_message_calls: bridge.sendMessageCalls(),
    has_post: bridge.handlers.has("session_compact"),
  };
}

const positiveBridge = installBridge();
const pre = positiveBridge.handlers.get("session_before_compact");
const post = positiveBridge.handlers.get("session_compact");
if (pre === undefined || post === undefined) throw new Error("positive handlers missing");
const calls: ConfirmCall[] = [];
let positiveNotifyCalls = 0;
const positiveContext = { ui: {
  confirm(title: string, message: string) { calls.push({ title, message }); return true; },
  notify() { positiveNotifyCalls++; },
} };
const positivePre = await invoke(pre, positiveContext, "positive-pre");
const positivePost = await invoke(post, positiveContext, "positive-post");

const receipt = {
  positive: {
    pre_state: stateOf(positivePre),
    post_state: stateOf(positivePost),
    confirm_calls: calls,
    notify_calls: positiveNotifyCalls,
    send_message_calls: positiveBridge.sendMessageCalls(),
  },
  pre: {
    denied: await runPreCase("pre-false", () => false),
    timeout: await runPreCase("pre-timeout", () => new Promise(() => {})),
    thrown: await runPreCase("pre-throw", () => { throw new Error("confirm throw"); }),
    missing: await runPreCase("pre-missing", undefined),
  },
  post: {
    denied: await runPostCase("post-false", () => false),
    timeout: await runPostCase("post-timeout", () => new Promise(() => {})),
    thrown: await runPostCase("post-throw", () => { throw new Error("confirm throw"); }),
  },
  invalid_timeout: {
    below_minimum: await runInvalidTimeoutCase("49"),
    above_maximum: await runInvalidTimeoutCase("10001"),
    fractional: await runInvalidTimeoutCase("50.5"),
    not_numeric: await runInvalidTimeoutCase("invalid"),
    empty: await runInvalidTimeoutCase(""),
  },
};
console.log(JSON.stringify(receipt));
`

type ompBridgeACKCaseReceipt struct {
	State            string `json:"state"`
	ConfirmCalls     int    `json:"confirm_calls"`
	NotifyCalls      int    `json:"notify_calls"`
	SendMessageCalls int    `json:"send_message_calls"`
}

type ompBridgeInvalidTimeoutReceipt struct {
	ompBridgeACKCaseReceipt
	HasPost bool `json:"has_post"`
}

func TestOMPContextBridgeACK_RequiresBodyFreeCorrelatedConfirm(t *testing.T) {
	bun, err := exec.LookPath("bun")
	if err != nil {
		t.Skip("bun is required to execute the generated TypeScript ACK contract")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, bun, "run", "-")
	cmd.Env = []string{"HOME=" + t.TempDir(), "PATH=" + os.Getenv("PATH")}
	cmd.Stdin = strings.NewReader(ompContextBridgeSource + ompContextBridgeACKHarness)
	output, runErr := cmd.CombinedOutput()
	require.NoError(t, runErr, "generated bridge ACK harness failed: %s", output)

	var receipt struct {
		Positive struct {
			PreState     string `json:"pre_state"`
			PostState    string `json:"post_state"`
			ConfirmCalls []struct {
				Title   string `json:"title"`
				Message string `json:"message"`
			} `json:"confirm_calls"`
			NotifyCalls      int `json:"notify_calls"`
			SendMessageCalls int `json:"send_message_calls"`
		} `json:"positive"`
		Pre            map[string]ompBridgeACKCaseReceipt        `json:"pre"`
		Post           map[string]ompBridgeACKCaseReceipt        `json:"post"`
		InvalidTimeout map[string]ompBridgeInvalidTimeoutReceipt `json:"invalid_timeout"`
	}
	require.NoError(t, json.Unmarshal(output, &receipt), "receipt: %s", output)

	assert.Equal(t, "undefined", receipt.Positive.PreState, "confirm=true is the only compaction admission")
	assert.Equal(t, "undefined", receipt.Positive.PostState)
	require.Len(t, receipt.Positive.ConfirmCalls, 2, "pre and post each require one ACK")
	assert.Zero(t, receipt.Positive.NotifyCalls, "notify is not an authority channel")
	assert.Zero(t, receipt.Positive.SendMessageCalls, "the bridge must not inject provider content")

	hashes := map[string]string{
		"binding_hash": "sha256:" + strings.Repeat("a", 64),
		"options_hash": "sha256:" + strings.Repeat("b", 64),
		"session_hash": "sha256:" + strings.Repeat("c", 64),
		"nonce_hash":   "sha256:" + strings.Repeat("d", 64),
	}
	for index, event := range []string{"pre_compaction", "post_compaction"} {
		call := receipt.Positive.ConfirmCalls[index]
		assert.Equal(t, "Autopus context "+event, call.Title)
		var envelope map[string]string
		require.NoError(t, json.Unmarshal([]byte(call.Message), &envelope))
		require.Len(t, envelope, 6, "the body-free ACK envelope has exact keys only")
		assert.Equal(t, "autopus.omp-context-bridge.v1", envelope["schema_version"])
		assert.Equal(t, event, envelope["event"])
		for key, value := range hashes {
			assert.Equal(t, value, envelope[key])
		}
	}

	for name, got := range receipt.Pre {
		assert.Equal(t, "cancel", got.State, "pre case %s must fail closed", name)
		assert.Zero(t, got.NotifyCalls, "pre case %s used notify authority", name)
		assert.Zero(t, got.SendMessageCalls, "pre case %s injected provider content", name)
		if name == "missing" {
			assert.Zero(t, got.ConfirmCalls)
		} else {
			assert.Equal(t, 1, got.ConfirmCalls)
		}
	}
	for name, got := range receipt.Post {
		assert.Equal(t, "undefined", got.State, "post case %s must finish body-free", name)
		assert.Equal(t, 1, got.ConfirmCalls)
		assert.Zero(t, got.NotifyCalls, "post case %s used notify authority", name)
		assert.Zero(t, got.SendMessageCalls, "post case %s injected provider content", name)
	}
	for name, got := range receipt.InvalidTimeout {
		assert.Equal(t, "cancel", got.State, "invalid timeout %s must fail closed", name)
		assert.Zero(t, got.ConfirmCalls, "invalid timeout %s must not open an ACK dialog", name)
		assert.Zero(t, got.NotifyCalls)
		assert.Zero(t, got.SendMessageCalls)
		assert.False(t, got.HasPost, "invalid timeout %s must not register post authority", name)
	}
}
