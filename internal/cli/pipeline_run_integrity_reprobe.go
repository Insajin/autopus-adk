package cli

import (
	"context"

	"github.com/insajin/autopus-adk/pkg/codexruntime"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/execplane"
)

// execplaneCatalogReprobeFunc re-reads a provider's model catalog under an
// explicit account home. It is the gate's second and last outside-world seam,
// kept package-level for the same reason the tier probe is: a test drives the
// re-probe branch without codex installed.
type execplaneCatalogReprobeFunc func(ctx context.Context, binary, codexHome string) ([]byte, error)

var runtimeExecplaneCatalogReprobe execplaneCatalogReprobeFunc = probeCodexCatalogUnderHome

// probeCodexCatalogUnderHome reads `codex debug models` with CODEX_HOME pinned
// to one account's managed home. It is a single read-only subprocess: no
// worktree, Run, or session is created, and nothing is written into that home.
func probeCodexCatalogUnderHome(ctx context.Context, binary, codexHome string) ([]byte, error) {
	return codexruntime.ProbeModelCatalogUnderHome(ctx, binary, codexHome, codexRuntimeCatalogTimeout)
}

// reprobeCatalogUnderExecutionAccount implements REQ-004's mismatch branch. A
// catalog probed under a different entitlement proves nothing about the account
// that will run the workload, so it is fetched again with CODEX_HOME pinned to
// that account. It returns the entitlement the held catalog now belongs to, and
// the note the receipt's reason must carry.
//
// Matching grades return before any I/O: entitlement comparison is worth having
// only because it spends no subprocess, and re-probing every run would erase
// that. Claude is not re-probed at all — it exposes no account-scoped catalog
// probe, so a mismatch there stays REQ-009 unverified rather than growing a
// second re-probe mechanism that proves nothing.
func reprobeCatalogUnderExecutionAccount(
	ctx context.Context, cfg *config.HarnessConfig, provider string, evidence execplane.Evidence,
) (execplane.Entitlement, string) {
	held := evidence.ProbeEntitlement
	verdict, _ := execplane.CompareEntitlement(evidence.ExecEntitlement, held)
	if verdict != execplane.VerdictReprobe || provider != execplane.ProviderCodex {
		return held, ""
	}
	home, err := execplane.ManagedCodexHome(evidence.Resolution.Account.ID)
	if err == nil {
		_, err = runtimeExecplaneCatalogReprobe(ctx, codexProviderBinary(cfg), home)
	}
	if err != nil {
		// The error itself is dropped: it can name a managed home path, and the
		// receipt only needs to say which step failed.
		return held, "catalog re-probe under the execution account failed"
	}
	// The catalog now comes from the execution account, so that account is its
	// own catalog source and the grade it was probed under is its own.
	return evidence.ExecEntitlement, "catalog was re-probed under the execution account"
}

// codexProviderBinary names the codex CLI this run would launch. A project may
// point it at a wrapper, and a re-probe that read the catalog through a
// different binary would not be evidence about this run.
func codexProviderBinary(cfg *config.HarnessConfig) string {
	if entry, ok := cfg.Orchestra.Providers[execplane.ProviderCodex]; ok && entry.Binary != "" {
		return entry.Binary
	}
	return config.DefaultCodexProviderEntry().Binary
}
