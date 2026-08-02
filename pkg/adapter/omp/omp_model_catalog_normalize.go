package omp

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"
)

var ompModelCapabilityAllowlist = map[string]struct{}{
	"coding_tool_use":         {},
	"deep_reasoning":          {},
	"deterministic_transform": {},
	"fast_validation":         {},
	"independent_dissent":     {},
	"vision_design":           {},
}

var ompThinkingAllowlist = map[string]struct{}{
	"off": {}, "none": {}, "minimal": {}, "low": {}, "medium": {}, "high": {}, "xhigh": {},
}

type rawOMPModelCatalog struct {
	Models *[]rawOMPModelMetadata `json:"models"`
}

type rawOMPModelMetadata struct {
	Provider     string    `json:"provider"`
	ID           string    `json:"id"`
	Family       string    `json:"family"`
	Capabilities *[]string `json:"capabilities"`
	Thinking     *[]string `json:"thinking"`
	AuthEnabled  *bool     `json:"auth_enabled"`
	Available    *bool     `json:"available"`
	Keyless      bool      `json:"keyless"`
	Disabled     bool      `json:"disabled"`
}

// NormalizeOMPModelCatalog drops unrecognized fields before fingerprinting.
func NormalizeOMPModelCatalog(data []byte, maxOutput int) (OMPModelCatalog, string) {
	if maxOutput <= 0 || len(data) > maxOutput {
		return OMPModelCatalog{}, "catalog_oversized"
	}
	var raw rawOMPModelCatalog
	if !decodeOMPModelCatalogJSON(data, &raw) {
		return OMPModelCatalog{}, "catalog_invalid"
	}
	if raw.Models == nil {
		return OMPModelCatalog{}, "catalog_invalid"
	}
	if len(*raw.Models) == 0 {
		return OMPModelCatalog{}, "catalog_empty"
	}

	models := make([]OMPModelMetadata, 0, len(*raw.Models))
	seen := make(map[string]struct{}, len(*raw.Models))
	for _, entry := range *raw.Models {
		model, reason := normalizeOMPModelMetadata(entry)
		if reason != "" {
			return OMPModelCatalog{}, reason
		}
		selector := model.Provider + "/" + model.Model
		if _, duplicate := seen[selector]; duplicate {
			return OMPModelCatalog{}, "catalog_invalid"
		}
		seen[selector] = struct{}{}
		models = append(models, model)
	}
	return canonicalOMPModelCatalog(models)
}

func canonicalOMPModelCatalog(models []OMPModelMetadata) (OMPModelCatalog, string) {
	models = append([]OMPModelMetadata(nil), models...)
	sort.Slice(models, func(i, j int) bool {
		if models[i].Provider != models[j].Provider {
			return models[i].Provider < models[j].Provider
		}
		return models[i].Model < models[j].Model
	})
	payload, err := json.Marshal(struct {
		Models []OMPModelMetadata `json:"models"`
	}{Models: models})
	if err != nil {
		return OMPModelCatalog{}, "catalog_invalid"
	}
	sum := sha256.Sum256(payload)
	return OMPModelCatalog{Models: models, Fingerprint: "sha256:" + hex.EncodeToString(sum[:])}, "catalog_ready"
}

func decodeOMPModelCatalogJSON(data []byte, target any) bool {
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	if err := decoder.Decode(target); err != nil {
		return false
	}
	var trailing any
	return errors.Is(decoder.Decode(&trailing), io.EOF)
}

// @AX:WARN [AUTO]: model metadata normalization has cyclomatic complexity 17.
// @AX:REASON [AUTO]: gocyclo reports 17 across token, capability, thinking-level, context-window, and duplicate-value checks.
func normalizeOMPModelMetadata(raw rawOMPModelMetadata) (OMPModelMetadata, string) {
	if !safeOMPModelToken(raw.Provider) || !safeOMPModelToken(raw.ID) {
		return OMPModelMetadata{}, "catalog_invalid"
	}
	if raw.Family == "" || raw.Capabilities == nil || raw.Thinking == nil ||
		(!raw.Keyless && raw.AuthEnabled == nil && raw.Available == nil) {
		return OMPModelMetadata{}, "catalog_metadata_insufficient"
	}
	if !safeOMPModelToken(raw.Family) || len(*raw.Capabilities) == 0 || len(*raw.Thinking) == 0 {
		return OMPModelMetadata{}, "catalog_metadata_insufficient"
	}
	capabilities, ok := normalizeOMPAllowlistedValues(*raw.Capabilities, ompModelCapabilityAllowlist)
	if !ok {
		return OMPModelMetadata{}, "catalog_invalid"
	}
	thinking, ok := normalizeOMPAllowlistedValues(*raw.Thinking, ompThinkingAllowlist)
	if !ok {
		return OMPModelMetadata{}, "catalog_invalid"
	}
	authEnabled := raw.AuthEnabled != nil && *raw.AuthEnabled
	if raw.AuthEnabled == nil && raw.Available != nil {
		authEnabled = *raw.Available
	}
	return OMPModelMetadata{
		Provider: raw.Provider, Model: raw.ID, Family: raw.Family,
		Capabilities: capabilities, Thinking: thinking,
		AuthEnabled: authEnabled, Keyless: raw.Keyless, Disabled: raw.Disabled,
	}, ""
}

func normalizeOMPAllowlistedValues(values []string, allowed map[string]struct{}) ([]string, bool) {
	normalized := append([]string(nil), values...)
	sort.Strings(normalized)
	result := normalized[:0]
	for _, value := range normalized {
		if _, ok := allowed[value]; !ok {
			return nil, false
		}
		if len(result) == 0 || result[len(result)-1] != value {
			result = append(result, value)
		}
	}
	return result, true
}

func safeOMPModelToken(value string) bool {
	if value == "" || strings.TrimSpace(value) != value || strings.Contains(value, "/") {
		return false
	}
	for _, char := range value {
		if char < 0x21 || char == 0x7f {
			return false
		}
	}
	return true
}
