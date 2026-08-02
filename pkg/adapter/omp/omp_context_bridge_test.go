package omp

import (
	"context"
	"encoding/json"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

const expectedOMPContextBridgeSource = `type BridgeEvent = "pre_compaction" | "post_compaction";

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

func TestOMPContextBridge_NoOptInPreservesExactPreparedFiles(t *testing.T) {
	adapterUnderTest := NewWithRoot(t.TempDir())
	baseline := configForOMP()
	baselineFiles, err := adapterUnderTest.prepareFiles(context.Background(), baseline)
	require.NoError(t, err)

	catalogOnly := configForOMP()
	catalogOnly.OMPContextPolicy.Profiles = map[string]config.OMPContextProfileConf{
		"bridge": {},
	}
	catalogFiles, err := adapterUnderTest.prepareFiles(context.Background(), catalogOnly)
	require.NoError(t, err)

	const priorPreparedFilesFingerprint = "bdcb57a29fc98ed038c49f4af3bef0472e7373987f11b5434461d180fd62e9de"
	assert.Equal(t, priorPreparedFilesFingerprint, fingerprintOMPFileMappings(t, baselineFiles))
	assert.Equal(t, fingerprintOMPFileMappings(t, baselineFiles), fingerprintOMPFileMappings(t, catalogFiles))
	assert.NotContains(t, ompMappingTargets(baselineFiles), ".omp/extensions/autopus-context.ts")
}

func TestOMPContextBridge_OptInEmitsExactBodyFreeMapping(t *testing.T) {
	cfg := optedInOMPContextBridgeConfig()
	files, err := NewWithRoot(t.TempDir()).prepareFiles(context.Background(), cfg)
	require.NoError(t, err)

	var bridgeMappings []adapter.FileMapping
	for _, file := range files {
		if file.TargetPath == ".omp/extensions/autopus-context.ts" {
			bridgeMappings = append(bridgeMappings, file)
		}
	}
	require.Len(t, bridgeMappings, 1)
	bridge := bridgeMappings[0]
	assert.Equal(t, adapter.OverwriteAlways, bridge.OverwritePolicy)
	assert.Equal(t, adapter.Checksum(expectedOMPContextBridgeSource), bridge.Checksum)
	assert.Equal(t, expectedOMPContextBridgeSource, string(bridge.Content))

	for _, forbidden := range []string{
		"branchEntries", "compactionEntry", "customInstructions", "preparation",
		"sessionManager", "getSessionId", "sendMessage", "appendEntry",
		"registerProvider", "registerTool", "fetch(", "node:fs", "Deno.", "Bun.",
		"OPENAI_API_KEY", "ANTHROPIC_API_KEY", "/Users/", "../",
	} {
		assert.NotContains(t, string(bridge.Content), forbidden)
	}
	assert.Equal(t, 1, strings.Count(string(bridge.Content), "process.env.AUTOPUS_OMP_CONTEXT_BINDING_HASH"))
	assert.Equal(t, 1, strings.Count(string(bridge.Content), "process.env.AUTOPUS_OMP_CONTEXT_OPTIONS_HASH"))
	assert.Equal(t, 1, strings.Count(string(bridge.Content), "process.env.AUTOPUS_OMP_CONTEXT_SESSION_HASH"))
	assert.Contains(t, string(bridge.Content), "return { cancel: true } as const;")
	assert.Contains(t, string(bridge.Content), "const NOTIFY_TIMEOUT_MS = 250;")
	assert.Contains(t, string(bridge.Content), "} finally {")
	assert.False(t, NewWithRoot(t.TempDir()).SupportsHooks(), "generic hook semantics remain unsupported")
}

func optedInOMPContextBridgeConfig() *config.HarnessConfig {
	cfg := configForOMP()
	cfg.OMPContextPolicy = config.OMPContextPolicyConf{
		Profile: "bridge",
		Profiles: map[string]config.OMPContextProfileConf{
			"bridge": {},
		},
	}
	return cfg
}

func fingerprintOMPFileMappings(t *testing.T, files []adapter.FileMapping) string {
	t.Helper()
	type exactMapping struct {
		SourceTemplate  string                  `json:"source_template"`
		TargetPath      string                  `json:"target_path"`
		OverwritePolicy adapter.OverwritePolicy `json:"overwrite_policy"`
		Checksum        string                  `json:"checksum"`
		Content         string                  `json:"content"`
	}
	exact := make([]exactMapping, 0, len(files))
	for _, file := range files {
		exact = append(exact, exactMapping{
			SourceTemplate: file.SourceTemplate, TargetPath: file.TargetPath,
			OverwritePolicy: file.OverwritePolicy, Checksum: file.Checksum,
			Content: string(file.Content),
		})
	}
	sort.Slice(exact, func(i, j int) bool { return exact[i].TargetPath < exact[j].TargetPath })
	encoded, err := json.Marshal(exact)
	require.NoError(t, err)
	return adapter.Checksum(string(encoded))
}

func ompMappingTargets(files []adapter.FileMapping) []string {
	targets := make([]string, 0, len(files))
	for _, file := range files {
		targets = append(targets, file.TargetPath)
	}
	sort.Strings(targets)
	return targets
}
