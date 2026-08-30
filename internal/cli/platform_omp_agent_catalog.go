package cli

import (
	"context"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/insajin/autopus-adk/pkg/config"
)

const (
	ompExpectedAgentCatalogSize = 16
	ompMaxAgentDefinitionBytes  = 1 << 20
	ompAgentManifestPath        = ".autopus/omp-manifest.json"
)

type ompAgentCatalogSummary struct {
	Status    string
	Reason    string
	Expected  int
	Installed int
	Verified  int
}

func newOMPModelOperatorProjection(
	rows []ompEffectiveModelProjection,
	catalog ompAgentCatalogSummary,
) ompModelOperatorProjection {
	return ompModelOperatorProjection{
		Status: "disabled", Reason: "profile_not_selected", CatalogStatus: "not_probed",
		CatalogReason: "profile_not_selected", CatalogTrust: config.RoleModelCatalogTrustStrict,
		ReceiptStatus:      "not_applicable",
		AgentCatalogStatus: catalog.Status, AgentCatalogReason: catalog.Reason,
		ExpectedAgents: catalog.Expected, InstalledAgents: catalog.Installed,
		VerifiedAgents: catalog.Verified, Models: rows,
	}
}
func buildOMPAgentCatalog(
	ctx context.Context,
	root string,
	runner omp.OMPModelCatalogRunner,
) ([]ompEffectiveModelProjection, ompAgentCatalogSummary) {
	mapping := config.OMPAgentRoleMapping()
	names := make([]string, 0, len(mapping))
	for name := range mapping {
		names = append(names, name)
	}
	sort.Strings(names)

	verified := validateOMPAgentDefinitions(ctx, root, runner, names)
	rows := make([]ompEffectiveModelProjection, 0, len(names))
	summary := ompAgentCatalogSummary{Expected: len(names)}
	for _, name := range names {
		role := mapping[name]
		capability, _ := config.OMPNativeRoleCapability(role)
		definitionPath := path.Join(".omp", "agents", name+".md")
		installStatus := inspectOMPAgentDefinition(root, definitionPath)
		definitionVerified := verified[definitionPath]
		if installStatus == "installed" {
			summary.Installed++
		}
		if definitionVerified {
			summary.Verified++
		}
		rows = append(rows, ompEffectiveModelProjection{
			Agent: name, Role: role, Capability: capability,
			ModelAlias: "inherit", EffectiveSelector: "",
			Source: "generated_omp_agent_catalog", ConfigSource: "inherited",
			Status: "inherited", Reason: "profile_not_selected",
			DefinitionPath: definitionPath, InstallStatus: installStatus,
			DefinitionVerified: definitionVerified, FallbackAttempts: []ompFallbackProjection{},
		})
	}
	if summary.Expected == ompExpectedAgentCatalogSize &&
		summary.Installed == summary.Expected && summary.Verified == summary.Expected {
		summary.Status, summary.Reason = "ready", "agent_catalog_ready"
	} else {
		summary.Status, summary.Reason = "blocked", "agent_catalog_incomplete"
	}
	return rows, summary
}

func validateOMPAgentDefinitions(
	ctx context.Context,
	root string,
	runner omp.OMPModelCatalogRunner,
	names []string,
) map[string]bool {
	verified := make(map[string]bool, len(names))
	for _, name := range names {
		verified[path.Join(".omp", "agents", name+".md")] = true
	}
	if adapter.RejectSymlinkComponents(root, ompAgentManifestPath) != nil {
		return map[string]bool{}
	}
	manifestPath := filepath.Join(root, filepath.FromSlash(ompAgentManifestPath))
	manifestBefore, err := os.Lstat(manifestPath)
	if err != nil || !manifestBefore.Mode().IsRegular() {
		return map[string]bool{}
	}
	findings, err := omp.NewWithRoot(root).WithModelIntegrationRunner(runner).Validate(ctx)
	if err != nil {
		return map[string]bool{}
	}
	manifestAfter, err := os.Lstat(manifestPath)
	if err != nil || !manifestAfter.Mode().IsRegular() || !os.SameFile(manifestBefore, manifestAfter) {
		return map[string]bool{}
	}
	allInvalid := false
	for _, finding := range findings {
		file := filepath.ToSlash(finding.File)
		if file == ".autopus/omp-manifest.json" || file == ".omp/agents" {
			allInvalid = true
		}
		if strings.HasPrefix(file, ".omp/agents/") {
			verified[file] = false
		}
	}
	if allInvalid {
		for file := range verified {
			verified[file] = false
		}
	}
	return verified
}

func inspectOMPAgentDefinition(root, definitionPath string) string {
	if adapter.RejectSymlinkComponents(root, definitionPath) != nil {
		return "not_regular"
	}
	fullPath := filepath.Join(root, filepath.FromSlash(definitionPath))
	info, err := os.Lstat(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "missing"
		}
		return "unreadable"
	}
	if !info.Mode().IsRegular() || info.Size() > ompMaxAgentDefinitionBytes {
		return "not_regular"
	}
	return "installed"
}

func overlayOMPAgentCatalog(
	rows []ompEffectiveModelProjection,
	resolutions []omp.OMPModelRouteResolution,
	profile config.RoleModelProfileConf,
	receiptVerified bool,
) []ompEffectiveModelProjection {
	byAgent := make(map[string]omp.OMPModelRouteResolution, len(resolutions))
	for _, resolution := range resolutions {
		if _, exists := byAgent[resolution.Agent]; !exists {
			byAgent[resolution.Agent] = resolution
		}
	}
	for index := range rows {
		row := &rows[index]
		resolution, exists := byAgent[row.Agent]
		if !exists {
			continue
		}
		row.ModelAlias = "@" + row.Role
		row.Source = "autopus.yaml"
		row.ConfigSource = safeOMPOperatorToken(profile.ConfigMode)
		row.Status = safeOMPOperatorReason(resolution.Status)
		row.Reason = safeOMPOperatorReason(resolution.Reason)
		row.FallbackAttempts = projectOMPFallbackAttempts(resolution.FallbackAttempts)
		row.FallbackUsed = ompFallbackWasUsed(resolution.FallbackAttempts)
		row.Verified = receiptVerified && resolution.Status == "selected"
		if resolution.RequestedRole != "" {
			row.ModelAlias = "@" + safeOMPOperatorToken(resolution.RequestedRole)
		}
		if resolution.Status != "selected" {
			continue
		}
		row.Provider = safeOMPOperatorToken(resolution.EffectiveProvider)
		row.Model = safeOMPOperatorToken(resolution.EffectiveModel)
		row.Thinking = safeOMPOperatorToken(resolution.Thinking)
		row.EffectiveSelector = safeOMPOperatorToken(resolution.EffectiveSelector)
	}
	return rows
}

func projectOMPFallbackAttempts(attempts []omp.OMPRoutingAttempt) []ompFallbackProjection {
	projected := make([]ompFallbackProjection, 0, len(attempts))
	for _, attempt := range attempts {
		projected = append(projected, ompFallbackProjection{
			Index: attempt.Index, Selector: safeOMPOperatorToken(attempt.Selector),
			Status: safeOMPOperatorReason(attempt.Status), Reason: safeOMPOperatorReason(attempt.Reason),
		})
	}
	return projected
}

func ompFallbackWasUsed(attempts []omp.OMPRoutingAttempt) bool {
	for _, attempt := range attempts {
		if attempt.Status == "selected" && attempt.Index > 0 {
			return true
		}
	}
	return false
}
