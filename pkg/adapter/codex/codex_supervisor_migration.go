package codex

import (
	"os"
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

const (
	LegacySupervisorReasonConfigMissing          = "config_missing"
	LegacySupervisorReasonNoProjectOverride      = "no_project_override"
	LegacySupervisorReasonExplicitPolicy         = "explicit_policy"
	LegacySupervisorReasonCodexPlatformDisabled  = "codex_platform_disabled"
	LegacySupervisorReasonUserMarker             = "user_marker"
	LegacySupervisorReasonGeneratedHeaderMissing = "generated_header_missing"
	LegacySupervisorReasonManifestMissing        = "manifest_missing"
	LegacySupervisorReasonManifestEntryMissing   = "manifest_entry_missing"
	LegacySupervisorReasonManifestPolicyMismatch = "manifest_policy_mismatch"
	LegacySupervisorReasonChecksumDrift          = "checksum_drift"
	LegacySupervisorReasonCustomProfile          = "custom_profile"
	LegacySupervisorReasonManagedProfile         = "managed_legacy_profile"
)

// LegacySupervisorModelInspection describes ownership evidence for a project
// Codex model override without changing project state.
type LegacySupervisorModelInspection struct {
	HasProjectOverride bool
	Migratable         bool
	UserOwned          bool
	Reason             string
}

// SupervisorOverrideOwnershipInspection reports which root overrides an
// explicit supervisor policy would preserve or reclaim during an update.
type SupervisorOverrideOwnershipInspection struct {
	HasProjectOverride   bool
	HasManagedOverride   bool
	HasUserOwnedOverride bool
}

// InspectSupervisorOverrideOwnership applies the same per-key ownership rules
// as the Codex config merge without changing project state.
func InspectSupervisorOverrideOwnership(root string) (SupervisorOverrideOwnershipInspection, error) {
	path := filepath.Join(root, codexConfigRelPath)
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SupervisorOverrideOwnershipInspection{}, nil
		}
		return SupervisorOverrideOwnershipInspection{}, err
	}

	overrides := collectCodexConfigOverrides(string(existing))
	manifest, err := adapter.LoadManifest(root, adapterName)
	if err != nil {
		return SupervisorOverrideOwnershipInspection{}, err
	}
	preservation := codexModelSettingsToPreserve(existing, manifest, false)
	inspection := SupervisorOverrideOwnershipInspection{}
	for _, key := range []string{".model", ".model_reasoning_effort"} {
		if _, exists := overrides[key]; !exists {
			continue
		}
		inspection.HasProjectOverride = true
		if _, preserved := preservation.overrides[key]; preserved {
			inspection.HasUserOwnedOverride = true
		} else {
			inspection.HasManagedOverride = true
		}
	}
	return inspection, nil
}

// InspectLegacySupervisorModel identifies an unchanged Autopus-managed root
// profile that predates the explicit supervisor model policy.
func InspectLegacySupervisorModel(
	root string,
	cfg *config.HarnessConfig,
) (LegacySupervisorModelInspection, error) {
	path := filepath.Join(root, codexConfigRelPath)
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return legacySupervisorInspection(LegacySupervisorReasonConfigMissing, false), nil
		}
		return LegacySupervisorModelInspection{}, err
	}

	content := string(existing)
	overrides := collectCodexConfigOverrides(content)
	model, hasModel := overrides[".model"]
	effort, hasEffort := overrides[".model_reasoning_effort"]
	hasOverride := hasModel || hasEffort
	if !hasOverride {
		return legacySupervisorInspection(LegacySupervisorReasonNoProjectOverride, false), nil
	}
	if hasMarkedSupervisorOverride(content, overrides) {
		return legacySupervisorInspection(LegacySupervisorReasonUserMarker, true), nil
	}
	if cfg == nil || cfg.Quality.SupervisorModelPolicy != "" {
		return legacySupervisorInspection(LegacySupervisorReasonExplicitPolicy, false), nil
	}
	if !containsPlatform(cfg.Platforms, "codex") {
		return legacySupervisorInspection(LegacySupervisorReasonCodexPlatformDisabled, false), nil
	}
	if !hasStandaloneCodexComment(content, codexGeneratedConfigHeader) {
		return legacySupervisorInspection(LegacySupervisorReasonGeneratedHeaderMissing, true), nil
	}

	manifest, err := adapter.LoadManifest(root, adapterName)
	if err != nil {
		return LegacySupervisorModelInspection{}, err
	}
	if manifest == nil {
		return legacySupervisorInspection(LegacySupervisorReasonManifestMissing, true), nil
	}
	entry, managed := codexManifestConfigEntry(manifest)
	if !managed {
		return legacySupervisorInspection(LegacySupervisorReasonManifestEntryMissing, true), nil
	}
	if entry.Policy != adapter.OverwriteMerge {
		return legacySupervisorInspection(LegacySupervisorReasonManifestPolicyMismatch, true), nil
	}
	if adapter.Checksum(content) != entry.Checksum {
		return legacySupervisorInspection(LegacySupervisorReasonChecksumDrift, true), nil
	}
	if !isKnownManagedCodexSupervisorTuple(model, hasModel, effort, hasEffort) {
		return legacySupervisorInspection(LegacySupervisorReasonCustomProfile, true), nil
	}

	return LegacySupervisorModelInspection{
		HasProjectOverride: true,
		Migratable:         true,
		Reason:             LegacySupervisorReasonManagedProfile,
	}, nil
}

func legacySupervisorInspection(reason string, userOwned bool) LegacySupervisorModelInspection {
	return LegacySupervisorModelInspection{
		HasProjectOverride: reason != LegacySupervisorReasonConfigMissing && reason != LegacySupervisorReasonNoProjectOverride,
		UserOwned:          userOwned,
		Reason:             reason,
	}
}

func hasMarkedSupervisorOverride(content string, overrides map[string]string) bool {
	marked, ok := markedCodexModelOverrides(content, overrides)
	if !ok {
		return false
	}
	_, modelMarked := marked[".model"]
	_, effortMarked := marked[".model_reasoning_effort"]
	return modelMarked || effortMarked
}
