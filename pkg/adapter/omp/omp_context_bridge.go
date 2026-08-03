package omp

import (
	"fmt"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

const (
	ompContextBridgeTarget = ".omp/extensions/autopus-context.ts"
	// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: embedded OMP extension is the external pre/post-compaction acknowledgement contract.
	// @AX:REASON [AUTO]: generated bridge code and the managed RPC driver must agree on schema, authority hashes, event names, and fail-closed confirmation behavior.
	// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: the 50-10000 ms ACK window defaults to 5 seconds and rejects out-of-range environment overrides.
	ompContextBridgeSource = `type BridgeEvent = "pre_compaction" | "post_compaction";

type BridgeContext = {
  ui: {
    confirm(title: string, message: string): boolean | Promise<boolean>;
  };
};

type BridgeHashes = {
  binding_hash: string;
  options_hash: string;
  session_hash: string;
  nonce_hash: string;
};

type BridgeConfig = {
  hashes: BridgeHashes;
  ackTimeoutMs: number;
};

type PreCompactResult = {
  cancel: true;
};

type BridgeAPI = {
  on(
    event: "session_before_compact",
    handler: (
      _event: unknown,
      context: BridgeContext,
    ) => PreCompactResult | undefined | Promise<PreCompactResult | undefined>,
  ): void;
  on(
    event: "session_compact",
    handler: (_event: unknown, context: BridgeContext) => void | Promise<void>,
  ): void;
};

const HASH_PATTERN = /^sha256:[0-9a-f]{64}$/;
const INTEGER_PATTERN = /^[0-9]+$/;
const DEFAULT_ACK_TIMEOUT_MS = 5000;
const MIN_ACK_TIMEOUT_MS = 50;
const MAX_ACK_TIMEOUT_MS = 10000;

function checkedHash(value: string | undefined): string | null {
  return value !== undefined && HASH_PATTERN.test(value) ? value : null;
}

function readACKTimeout(): number | null {
  const value = process.env.AUTOPUS_OMP_CONTEXT_ACK_TIMEOUT_MS;
  if (value === undefined) {
    return DEFAULT_ACK_TIMEOUT_MS;
  }
  if (!INTEGER_PATTERN.test(value)) {
    return null;
  }
  const timeout = Number(value);
  return Number.isSafeInteger(timeout) && timeout >= MIN_ACK_TIMEOUT_MS && timeout <= MAX_ACK_TIMEOUT_MS
    ? timeout
    : null;
}

function readBridgeConfig(): BridgeConfig | null {
  const bindingHash = checkedHash(process.env.AUTOPUS_OMP_CONTEXT_BINDING_HASH);
  const optionsHash = checkedHash(process.env.AUTOPUS_OMP_CONTEXT_OPTIONS_HASH);
  const sessionHash = checkedHash(process.env.AUTOPUS_OMP_CONTEXT_SESSION_HASH);
  const nonceHash = checkedHash(process.env.AUTOPUS_OMP_CONTEXT_NONCE_HASH);
  const ackTimeoutMs = readACKTimeout();
  if (bindingHash === null || optionsHash === null || sessionHash === null ||
      nonceHash === null || ackTimeoutMs === null) {
    return null;
  }
  return {
    hashes: {
      binding_hash: bindingHash,
      options_hash: optionsHash,
      session_hash: sessionHash,
      nonce_hash: nonceHash,
    },
    ackTimeoutMs,
  };
}

async function confirmSafely(
  context: BridgeContext,
  event: BridgeEvent,
  hashes: BridgeHashes,
  ackTimeoutMs: number,
): Promise<boolean> {
  const message = JSON.stringify({
    schema_version: "autopus.omp-context-bridge.v1",
    event,
    ...hashes,
  });
  return new Promise<boolean>((resolve) => {
    let settled = false;
    const finish = (confirmed: boolean): void => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);
      resolve(confirmed);
    };
    const timer = setTimeout(() => finish(false), ackTimeoutMs);
    try {
      if (typeof context.ui.confirm !== "function") {
        finish(false);
        return;
      }
      Promise.resolve(context.ui.confirm("Autopus context " + event, message)).then(
        (confirmed) => finish(confirmed === true),
        () => finish(false),
      );
    } catch {
      finish(false);
    }
  });
}

export default function autopusContextBridge(pi: BridgeAPI): void {
  const config = readBridgeConfig();
  pi.on("session_before_compact", async (_event, context) => {
    if (config === null || !(await confirmSafely(
      context,
      "pre_compaction",
      config.hashes,
      config.ackTimeoutMs,
    ))) {
      return { cancel: true } as const;
    }
    return undefined;
  });
  if (config === null) {
    return;
  }
  pi.on("session_compact", async (_event, context) => {
    await confirmSafely(context, "post_compaction", config.hashes, config.ackTimeoutMs);
  });
}
`
)

// OMPContextBridgeSourceIdentity describes the only generated extension admitted by a managed runtime.
type OMPContextBridgeSourceIdentity struct {
	TargetPath string
	SHA256     string
	Size       int64
}

// ExpectedOMPContextBridgeSourceIdentity returns the immutable embedded bridge identity.
func ExpectedOMPContextBridgeSourceIdentity() OMPContextBridgeSourceIdentity {
	return OMPContextBridgeSourceIdentity{
		TargetPath: ompContextBridgeTarget,
		SHA256:     adapter.Checksum(ompContextBridgeSource),
		Size:       int64(len(ompContextBridgeSource)),
	}
}

func prepareOMPContextBridgeMappings(cfg *config.HarnessConfig) ([]adapter.FileMapping, error) {
	_, _, selected, err := cfg.OMPContextPolicy.SelectedOMPContextProfile()
	if err != nil {
		return nil, fmt.Errorf("OMP context bridge policy invalid: %w", err)
	}
	if !selected {
		return nil, nil
	}
	return []adapter.FileMapping{{
		SourceTemplate:  "embedded:omp-context-bridge-v1",
		TargetPath:      ompContextBridgeTarget,
		OverwritePolicy: adapter.OverwriteAlways,
		Checksum:        adapter.Checksum(ompContextBridgeSource),
		Content:         []byte(ompContextBridgeSource),
	}}, nil
}
