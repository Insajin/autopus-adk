package omp

import (
	"fmt"
	"time"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
)

func (i *ompModelIntegration) receiptMapping(
	a *Adapter,
	configSource string,
	configRelative string,
	configData []byte,
	readback []byte,
	projectOwnershipDigest string,
) (adapter.FileMapping, error) {
	now := time.Now
	if a.modelIntegrationClock != nil {
		now = a.modelIntegrationClock
	}
	safetySource := "user_effective"
	if i.profile.Safety.ApprovalMode != "" || i.profile.Safety.IsolationMode != "" {
		safetySource = "autopus_profile"
	}
	resolutions := make(map[string]OMPModelRouteResolution, len(i.routing.Resolutions))
	for _, resolution := range i.routing.Resolutions {
		resolutions[resolution.Capability] = resolution
	}
	roles := make([]OMPModelRoleReceipt, 0, len(i.projection.Agents))
	for _, agent := range i.projection.Agents {
		capability, err := config.OMPNativeRoleCapability(agent.Role)
		if err != nil {
			return adapter.FileMapping{}, err
		}
		resolution, ok := resolutions[capability]
		if !ok || resolution.Status != "selected" {
			return adapter.FileMapping{}, fmt.Errorf("receipt resolution missing: %s", capability)
		}
		roles = append(roles, ompIntegratedRoleReceipt(
			i.profileName, configSource, safetySource, agent, resolution,
		))
	}
	receipt := OMPModelResolutionReceipt{
		OMPVersion: i.probe.Version, CatalogFingerprint: i.probe.Catalog.Fingerprint,
		Profile: i.profileName, ConfigSource: configSource, GeneratedAt: now().UTC(),
		ProjectOwnershipDigest: projectOwnershipDigest,
		Activation: OMPModelActivationReceipt{
			Argv:       []string{"--config", configRelative},
			ConfigHash: OMPModelSHA256(configData), ReadbackHash: OMPModelSHA256(readback),
		},
		Roles: roles,
		Safety: OMPModelSafetyReceipt{
			ApprovalMode:  i.profile.Safety.ApprovalMode,
			IsolationMode: i.profile.Safety.IsolationMode,
			Source:        safetySource,
		},
	}
	_, data, err := CanonicalOMPModelResolutionReceipt(receipt)
	if err != nil {
		return adapter.FileMapping{}, err
	}
	return ompIntegratedMapping(OMPModelReceiptRelativePath, data), nil
}

func ompIntegratedRoleReceipt(
	profile string,
	configSource string,
	safetySource string,
	agent OMPAgentModelProjection,
	resolution OMPModelRouteResolution,
) OMPModelRoleReceipt {
	attempts := make([]OMPModelFallbackAttemptReceipt, 0, len(resolution.FallbackAttempts))
	for _, attempt := range resolution.FallbackAttempts {
		attempts = append(attempts, OMPModelFallbackAttemptReceipt{
			Selector: attempt.Selector, Reason: attempt.Reason,
		})
	}
	return OMPModelRoleReceipt{
		Agent: agent.Agent, Profile: profile, ConfigSource: configSource,
		RequestedRole: agent.Role, EffectiveRole: agent.Role, Capability: resolution.Capability,
		Provider: resolution.EffectiveProvider, Model: resolution.EffectiveModel,
		Selector: resolution.EffectiveProvider + "/" + resolution.EffectiveModel,
		Thinking: resolution.Thinking, FallbackAttempts: attempts,
		FallbackReason: resolution.Reason, DegradedReason: resolution.DegradedReason,
		FamilyDiversity: OMPModelFamilyDiversityReceipt{
			Status: resolution.FamilyDiversity.Status, Reason: resolution.FamilyDiversity.Reason,
			ExecutorFamily:              resolution.FamilyDiversity.Executor,
			EffectiveFamily:             resolution.FamilyDiversity.Reviewer,
			IndependentProviderEvidence: resolution.IndependentProviderEvidence,
		},
		SafetySource: safetySource,
	}
}
