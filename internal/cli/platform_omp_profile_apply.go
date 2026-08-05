package cli

import (
	"context"
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/config"
)

type ompProfileActivator func(context.Context, string, *config.HarnessConfig) error

type ompProfileApplyPayload struct {
	Platform           string                        `json:"platform"`
	Name               string                        `json:"name"`
	Status             string                        `json:"status"`
	Generated          bool                          `json:"generated"`
	ConfigPath         string                        `json:"config_path"`
	Activation         string                        `json:"activation"`
	CatalogVersion     string                        `json:"catalog_version"`
	CatalogFingerprint string                        `json:"catalog_fingerprint"`
	Capabilities       []ompProfileCapabilityPayload `json:"capabilities"`
}

func runOMPProfileApplyCommand(
	cmd *cobra.Command,
	root string,
	name string,
	jsonMode bool,
	runner omp.OMPModelCatalogRunner,
	activate ompProfileActivator,
) error {
	payload, err := applyOMPProfile(cmd.Context(), root, name, runner, activate)
	if err != nil {
		if !jsonMode {
			return err
		}
		return writeJSONResultAndExit(
			cmd, jsonStatusError, err, "omp_profile_apply_failed",
			map[string]any{"platform": "omp", "name": name, "status": "blocked"},
			[]jsonMessage{{Code: "omp_profile_apply_failed", Message: err.Error()}}, nil,
		)
	}
	if jsonMode {
		return writeJSONResult(cmd, jsonStatusOK, payload, nil, []jsonCheck{{
			ID: "omp.profile.activation", Severity: "info", Status: "pass", Detail: "omp_update",
		}})
	}
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintf(out, "OMP profile applied: %s\n", payload.Name)
	_, _ = fmt.Fprintf(out, "Config: %s\nActivation: %s\n", payload.ConfigPath, payload.Activation)
	_, _ = fmt.Fprintf(out, "Catalog: %s %s\n", payload.CatalogVersion, payload.CatalogFingerprint)
	return nil
}

func applyOMPProfile(
	ctx context.Context,
	root string,
	name string,
	runner omp.OMPModelCatalogRunner,
	activate ompProfileActivator,
) (ompProfileApplyPayload, error) {
	if !config.IsValidQualityPresetName(name) {
		return ompProfileApplyPayload{}, ompProfilePlanError{reason: "profile_name_invalid"}
	}
	path, original, mode, err := readOwnedAutopusConfig(root)
	if err != nil {
		return ompProfileApplyPayload{}, err
	}
	cfg, err := config.LoadPreview(root)
	if err != nil {
		return ompProfileApplyPayload{}, errors.New("autopus_config_invalid")
	}
	if !containsOMPString(cfg.Platforms, "omp") {
		return ompProfileApplyPayload{}, errors.New("omp_platform_not_configured")
	}
	if activate == nil {
		return ompProfileApplyPayload{}, errors.New("omp_activation_path_unavailable")
	}
	probe, err := probeInstalledOMPCatalog(ctx, runner)
	if err != nil {
		return ompProfileApplyPayload{}, err
	}

	profile, exists := cfg.RoleModelPolicy.Profiles[name]
	generated := !exists
	if !exists {
		proposal, proposalErr := buildOMPProfileProposal(name, probe.Catalog)
		if proposalErr != nil {
			return ompProfileApplyPayload{}, proposalErr
		}
		profile = proposal.profile
	}
	if err := preflightOMPProfile(name, profile, probe.Catalog); err != nil {
		return ompProfileApplyPayload{}, err
	}
	if cfg.RoleModelPolicy.Profiles == nil {
		cfg.RoleModelPolicy.Profiles = make(map[string]config.RoleModelProfileConf)
	}
	if cfg.RoleModelPolicy.Version == "" {
		cfg.RoleModelPolicy.Version = config.RoleModelPolicyVersionV1
	}
	cfg.RoleModelPolicy.Profiles[name] = profile
	cfg.RoleModelPolicy.Profile = name
	if err := cfg.Validate(); err != nil {
		return ompProfileApplyPayload{}, errors.New("omp_profile_validation_failed")
	}

	encoded, err := marshalAutopusConfig(original, cfg)
	if err != nil {
		return ompProfileApplyPayload{}, err
	}
	if err := verifyAutopusConfigSnapshot(path, original, mode); err != nil {
		return ompProfileApplyPayload{}, err
	}
	if err := atomicWriteAutopusConfig(root, path, encoded, mode); err != nil {
		return ompProfileApplyPayload{}, fmt.Errorf("persist OMP profile: %w", err)
	}
	if err := activate(ctx, root, cfg); err != nil {
		if verifyErr := verifyAutopusConfigSnapshot(path, encoded, mode); verifyErr != nil {
			return ompProfileApplyPayload{}, fmt.Errorf(
				"activate OMP profile: %w; config rollback blocked: autopus_config_changed", err,
			)
		}
		if rollbackErr := atomicWriteAutopusConfig(root, path, original, mode); rollbackErr != nil {
			return ompProfileApplyPayload{}, fmt.Errorf(
				"activate OMP profile: %w; config rollback failed: %v", err, rollbackErr,
			)
		}
		return ompProfileApplyPayload{}, fmt.Errorf("activate OMP profile: %w", err)
	}
	if err := verifyAutopusConfigSnapshot(path, encoded, mode); err != nil {
		return ompProfileApplyPayload{}, errors.New("autopus_config_changed_during_activation")
	}
	return ompProfileApplyPayload{
		Platform: "omp", Name: name, Status: "applied", Generated: generated,
		ConfigPath: "autopus.yaml", Activation: "omp_update",
		CatalogVersion:     safeOMPOperatorVersion(probe.Version),
		CatalogFingerprint: probe.Catalog.Fingerprint,
		Capabilities:       profileCapabilityPayloads(profile),
	}, nil
}

func preflightOMPProfile(
	name string,
	profile config.RoleModelProfileConf,
	catalog omp.OMPModelCatalog,
) error {
	policy := config.RoleModelPolicyConf{
		Version:  config.RoleModelPolicyVersionV1,
		Profile:  name,
		Profiles: map[string]config.RoleModelProfileConf{name: profile},
	}
	if err := policy.Validate(); err != nil {
		return errors.New("omp_profile_validation_failed")
	}
	if !profile.FamilyDiversity.Enabled {
		return errors.New("family_diversity_required")
	}
	compilation := compileOMPModelDoctorRouting(profile, catalog)
	if len(compilation.Resolutions) != len(config.OMPAgentRoleMapping()) {
		return errors.New("omp_profile_projection_incomplete")
	}
	for _, resolution := range compilation.Resolutions {
		if resolution.Status != "selected" {
			return fmt.Errorf(
				"omp_profile_candidate_unavailable: agent=%s reason=%s",
				safeOMPOperatorToken(resolution.Agent), safeOMPOperatorReason(resolution.Reason),
			)
		}
	}
	return nil
}

func profileCapabilityPayloads(profile config.RoleModelProfileConf) []ompProfileCapabilityPayload {
	rows := make([]ompProfileCapabilityPayload, 0, len(config.OMPProviderNeutralCapabilities()))
	for _, capability := range config.OMPProviderNeutralCapabilities() {
		route := profile.Capabilities[capability]
		row := ompProfileCapabilityPayload{Capability: capability, Required: route.Required}
		for _, candidate := range route.Candidates {
			row.Candidates = append(row.Candidates, ompProfileCandidatePayload(candidate))
		}
		if row.Candidates == nil {
			row.Candidates = []ompProfileCandidatePayload{}
		}
		rows = append(rows, row)
	}
	return rows
}
