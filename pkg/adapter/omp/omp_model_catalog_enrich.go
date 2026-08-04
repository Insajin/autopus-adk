package omp

import (
	"sort"
)

type OMPModelCatalogDeclaration struct {
	Selector   string `json:"selector"`
	Family     string `json:"family"`
	Capability string `json:"capability"`
}

type availableOMPModelRow struct {
	Provider  string    `json:"provider"`
	ID        string    `json:"id"`
	Selector  string    `json:"selector"`
	Reasoning *bool     `json:"reasoning"`
	Input     []string  `json:"input"`
	Thinking  *[]string `json:"thinking"`
}

type availableOMPModelCatalog struct {
	Models *[]availableOMPModelRow `json:"models"`
}

type groupedOMPModelDeclaration struct {
	family       string
	capabilities map[string]struct{}
}

// NormalizeOMPAvailableCatalog intersects exact profile declarations with the
// installed OMP available-only catalog. It never infers family or capability
// intent from vendor or model names.
// @AX:WARN [AUTO]: available-catalog normalization has cyclomatic complexity 17.
// @AX:REASON [AUTO]: gocyclo reports 17 across output bounds, catalog parsing, exact declaration matching, and ambiguity handling.
func NormalizeOMPAvailableCatalog(
	data []byte,
	maxOutput int,
	declarations []OMPModelCatalogDeclaration,
) (OMPModelCatalog, string) {
	if maxOutput <= 0 || len(data) > maxOutput {
		return OMPModelCatalog{}, "catalog_oversized"
	}
	declared, ok := groupOMPModelDeclarations(declarations)
	if !ok {
		return OMPModelCatalog{}, "catalog_invalid"
	}
	var raw availableOMPModelCatalog
	if !decodeOMPModelCatalogJSON(data, &raw) || raw.Models == nil {
		return OMPModelCatalog{}, "catalog_invalid"
	}
	if len(*raw.Models) == 0 || len(declared) == 0 {
		return OMPModelCatalog{}, "catalog_empty"
	}

	models := make([]OMPModelMetadata, 0, len(declared))
	seen := make(map[string]struct{}, len(*raw.Models))
	for _, row := range *raw.Models {
		selector := row.Provider + "/" + row.ID
		declaration, wanted := declared[selector]
		if !wanted {
			continue
		}
		if _, duplicate := seen[selector]; duplicate || row.Selector != selector ||
			!safeOMPModelToken(row.Provider) || !safeOMPModelToken(row.ID) || row.Thinking == nil {
			return OMPModelCatalog{}, "catalog_invalid"
		}
		seen[selector] = struct{}{}
		thinking := filterOMPObservedThinking(*row.Thinking)
		if len(thinking) == 0 {
			return OMPModelCatalog{}, "catalog_metadata_insufficient"
		}
		models = append(models, OMPModelMetadata{
			Provider: row.Provider, Model: row.ID, Family: declaration.family,
			Capabilities: observedOMPDeclaredCapabilities(row, declaration.capabilities),
			Thinking:     thinking, AuthEnabled: true,
		})
	}
	if len(models) == 0 {
		return OMPModelCatalog{}, "catalog_empty"
	}
	return canonicalOMPModelCatalog(models)
}

func groupOMPModelDeclarations(
	declarations []OMPModelCatalogDeclaration,
) (map[string]groupedOMPModelDeclaration, bool) {
	grouped := make(map[string]groupedOMPModelDeclaration)
	for _, declaration := range declarations {
		provider, model, selectorOK := parseOMPRoutingSelector(declaration.Selector)
		_, capabilityOK := ompModelCapabilityAllowlist[declaration.Capability]
		if !selectorOK || provider+"/"+model != declaration.Selector ||
			!safeOMPModelToken(declaration.Family) || !capabilityOK {
			return nil, false
		}
		entry, exists := grouped[declaration.Selector]
		if exists && entry.family != declaration.Family {
			return nil, false
		}
		if !exists {
			entry = groupedOMPModelDeclaration{
				family: declaration.Family, capabilities: make(map[string]struct{}),
			}
		}
		entry.capabilities[declaration.Capability] = struct{}{}
		grouped[declaration.Selector] = entry
	}
	return grouped, true
}

func observedOMPDeclaredCapabilities(
	row availableOMPModelRow,
	declared map[string]struct{},
) []string {
	capabilities := make([]string, 0, len(declared))
	for capability := range declared {
		if capability == "deep_reasoning" && (row.Reasoning == nil || !*row.Reasoning) {
			continue
		}
		if capability == "vision_design" && !containsOMPModelValue(row.Input, "image") {
			continue
		}
		capabilities = append(capabilities, capability)
	}
	sort.Strings(capabilities)
	return capabilities
}

func filterOMPObservedThinking(values []string) []string {
	filtered := make([]string, 0, len(values))
	for _, value := range values {
		if _, ok := ompThinkingAllowlist[value]; ok {
			filtered = append(filtered, value)
		}
	}
	sort.Strings(filtered)
	result := filtered[:0]
	for _, value := range filtered {
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result
}
