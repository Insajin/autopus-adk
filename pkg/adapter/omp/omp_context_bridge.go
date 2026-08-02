package omp

import (
	"fmt"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

const (
	ompContextBridgeTarget = ".omp/extensions/autopus-context.ts"
	ompContextBridgeSource = `type BridgeEvent = "pre_compaction" | "post_compaction";

type BridgeContext = {
  ui: {
    notify(message: string, level: "info"): void | Promise<void>;
  };
};

type BridgeHashes = {
  binding_hash: string;
  options_hash: string;
  session_hash: string;
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
    ) => PreCompactResult | Promise<PreCompactResult>,
  ): void;
  on(
    event: "session_compact",
    handler: (_event: unknown, context: BridgeContext) => void | Promise<void>,
  ): void;
};

const HASH_PATTERN = /^sha256:[0-9a-f]{64}$/;
const NOTIFY_TIMEOUT_MS = 250;

function checkedHash(value: string | undefined): string | null {
  return value !== undefined && HASH_PATTERN.test(value) ? value : null;
}

function readBridgeHashes(): BridgeHashes | null {
  const bindingHash = checkedHash(process.env.AUTOPUS_OMP_CONTEXT_BINDING_HASH);
  const optionsHash = checkedHash(process.env.AUTOPUS_OMP_CONTEXT_OPTIONS_HASH);
  const sessionHash = checkedHash(process.env.AUTOPUS_OMP_CONTEXT_SESSION_HASH);
  if (bindingHash === null || optionsHash === null || sessionHash === null) {
    return null;
  }
  return {
    binding_hash: bindingHash,
    options_hash: optionsHash,
    session_hash: sessionHash,
  };
}

async function notifySafely(
  context: BridgeContext,
  event: BridgeEvent,
  hashes: BridgeHashes,
): Promise<void> {
  const message = JSON.stringify({
    schema_version: "autopus.omp-context-bridge.v1",
    event,
    ...hashes,
  });
  await new Promise<void>((resolve) => {
    let settled = false;
    const finish = (): void => {
      if (settled) {
        return;
      }
      settled = true;
      clearTimeout(timer);
      resolve();
    };
    const timer = setTimeout(finish, NOTIFY_TIMEOUT_MS);
    try {
      Promise.resolve(context.ui.notify(message, "info")).then(finish, finish);
    } catch {
      finish();
    }
  });
}

export default function autopusContextBridge(pi: BridgeAPI): void {
  const hashes = readBridgeHashes();
  pi.on("session_before_compact", async (_event, context) => {
    try {
      if (hashes !== null) {
        await notifySafely(context, "pre_compaction", hashes);
      }
    } finally {
      return { cancel: true } as const;
    }
  });
  if (hashes === null) {
    return;
  }
  pi.on("session_compact", async (_event, context) => {
    await notifySafely(context, "post_compaction", hashes);
  });
}
`
)

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
