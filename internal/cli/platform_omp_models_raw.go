package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/adapter/omp"
)

const ompRawCatalogFallbackReason = "catalog_metadata_insufficient"

type rawOMPDisplayCatalog struct {
	Models *[]rawOMPDisplayModel `json:"models"`
}

type rawOMPDisplayModel struct {
	Provider      string      `json:"provider"`
	ID            string      `json:"id"`
	Selector      string      `json:"selector"`
	Name          string      `json:"name"`
	ContextWindow json.Number `json:"contextWindow"`
	Reasoning     *bool       `json:"reasoning"`
	Thinking      *[]string   `json:"thinking"`
	Input         *[]string   `json:"input"`
}

func runOMPRawDisplayCatalogCommand(
	cmd *cobra.Command,
	jsonMode bool,
	runner omp.OMPModelCatalogRunner,
	strict omp.OMPModelCatalogProbeResult,
) error {
	body, runErr := runner.Run(cmd.Context(), "omp", "models", "--json", "--no-extensions")
	models, reason := normalizeOMPRawDisplayCatalog(body, ompModelDoctorProbeOutput)
	if runErr != nil || reason != "catalog_ready" {
		if reason == "catalog_ready" {
			reason = "catalog_invalid"
		}
		strict.Status, strict.Reason = "blocked", reason
		err := ompCatalogUnavailableError{reason: reason}
		payload := catalogPayloadFromProbe(strict)
		if jsonMode {
			return writeJSONResultAndExit(
				cmd, jsonStatusError, err, "omp_catalog_unavailable", payload,
				[]jsonMessage{{Code: "omp_catalog_unavailable", Message: reason}}, nil,
			)
		}
		return err
	}
	payload := ompCatalogPayload{
		Platform: "omp", Status: "degraded", Reason: ompRawCatalogFallbackReason,
		Version: safeOMPOperatorVersion(strict.Version), StrictRoutingReady: false, Models: models,
	}
	if jsonMode {
		return writeJSONResult(
			cmd, jsonStatusWarn, payload,
			[]jsonMessage{{
				Code:    "omp_catalog_metadata_insufficient",
				Message: "observed native models are display-only; profile init/apply and routing remain blocked",
			}},
			[]jsonCheck{{
				ID: "omp.catalog.semantic_metadata", Severity: "warning", Status: "warn",
				Detail: ompRawCatalogFallbackReason,
			}},
		)
	}
	return renderOMPModelsText(cmd, payload)
}

func normalizeOMPRawDisplayCatalog(body []byte, maxBytes int) ([]ompCatalogModelPayload, string) {
	if len(body) == 0 || maxBytes <= 0 || len(body) > maxBytes || rejectDuplicatePipelineOMPJSON(body) != nil {
		return nil, "catalog_invalid"
	}
	if secretKeyInOMPDisplayJSON(body) {
		return nil, "catalog_invalid"
	}
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var raw rawOMPDisplayCatalog
	if decoder.Decode(&raw) != nil {
		return nil, "catalog_invalid"
	}
	var trailing any
	if decoder.Decode(&trailing) != io.EOF || raw.Models == nil || len(*raw.Models) == 0 {
		return nil, "catalog_invalid"
	}
	models := make([]ompCatalogModelPayload, 0, len(*raw.Models))
	seen := make(map[string]struct{}, len(*raw.Models))
	for _, entry := range *raw.Models {
		model, err := normalizeOMPRawDisplayModel(entry)
		if err != nil {
			return nil, "catalog_invalid"
		}
		if _, exists := seen[model.Selector]; exists {
			return nil, "catalog_invalid"
		}
		seen[model.Selector] = struct{}{}
		models = append(models, model)
	}
	sort.Slice(models, func(i, j int) bool {
		if models[i].Provider != models[j].Provider {
			return models[i].Provider < models[j].Provider
		}
		return models[i].Model < models[j].Model
	})
	return models, "catalog_ready"
}

func normalizeOMPRawDisplayModel(raw rawOMPDisplayModel) (ompCatalogModelPayload, error) {
	provider, model := safeOMPOperatorToken(raw.Provider), safeOMPOperatorToken(raw.ID)
	if provider != raw.Provider || model != raw.ID || raw.Selector != raw.Provider+"/"+raw.ID ||
		safeOMPOperatorToken(raw.Selector) != raw.Selector || !safeOMPDisplayName(raw.Name) ||
		raw.Reasoning == nil || raw.Input == nil {
		return ompCatalogModelPayload{}, errors.New("unsafe native model metadata")
	}
	contextWindow, err := raw.ContextWindow.Int64()
	if err != nil || contextWindow <= 0 || contextWindow > 1<<30 {
		return ompCatalogModelPayload{}, errors.New("invalid native context window")
	}
	input, err := normalizeOMPDisplayInput(*raw.Input)
	if err != nil {
		return ompCatalogModelPayload{}, err
	}
	thinking, err := normalizeOMPDisplayThinking(raw.Thinking)
	if err != nil {
		return ompCatalogModelPayload{}, err
	}
	return ompCatalogModelPayload{
		Provider: provider, Model: model, Selector: raw.Selector, Name: raw.Name,
		ContextWindow: contextWindow, Reasoning: raw.Reasoning, NativeThinking: thinking,
		Input: input, AuthAvailability: "unknown", SemanticMetadataReady: false,
	}, nil
}

func normalizeOMPDisplayInput(values []string) ([]string, error) {
	if len(values) == 0 || len(values) > 2 {
		return nil, errors.New("invalid native input modes")
	}
	seen := map[string]bool{}
	for _, value := range values {
		if value != "text" && value != "image" || seen[value] {
			return nil, errors.New("invalid native input mode")
		}
		seen[value] = true
	}
	result := append([]string(nil), values...)
	sort.Strings(result)
	return result, nil
}

func normalizeOMPDisplayThinking(values *[]string) ([]string, error) {
	if values == nil {
		return nil, nil
	}
	if len(*values) == 0 || len(*values) > 6 {
		return nil, errors.New("invalid native thinking levels")
	}
	order := map[string]int{"minimal": 0, "low": 1, "medium": 2, "high": 3, "xhigh": 4, "max": 5}
	seen := map[string]bool{}
	result := append([]string(nil), (*values)...)
	for _, value := range result {
		if _, allowed := order[value]; !allowed || seen[value] {
			return nil, errors.New("invalid native thinking level")
		}
		seen[value] = true
	}
	sort.Slice(result, func(i, j int) bool { return order[result[i]] < order[result[j]] })
	return result, nil
}

func safeOMPDisplayName(value string) bool {
	if value == "" || len(value) > 256 || strings.TrimSpace(value) != value || !utf8.ValidString(value) {
		return false
	}
	lower := strings.ToLower(value)
	for _, marker := range ompDisplaySecretMarkers() {
		if strings.Contains(lower, marker) {
			return false
		}
	}
	for _, char := range value {
		if char < 0x20 || char == 0x7f {
			return false
		}
	}
	return true
}

func secretKeyInOMPDisplayJSON(body []byte) bool {
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.UseNumber()
	var value any
	if decoder.Decode(&value) != nil {
		return true
	}
	var inspect func(any) bool
	inspect = func(current any) bool {
		switch typed := current.(type) {
		case map[string]any:
			for key, child := range typed {
				lower := strings.ToLower(key)
				for _, marker := range ompDisplaySecretMarkers() {
					if strings.Contains(lower, marker) {
						return true
					}
				}
				if inspect(child) {
					return true
				}
			}
		case []any:
			for _, child := range typed {
				if inspect(child) {
					return true
				}
			}
		}
		return false
	}
	return inspect(value)
}

func ompDisplaySecretMarkers() []string {
	return []string{
		"authorization", "api_key", "api-key", "apikey", "password", "secret",
		"access_token", "refresh_token", "credential", "private_key",
	}
}

func formatOMPDisplayBool(value *bool) string {
	if value == nil {
		return "unknown"
	}
	return fmt.Sprint(*value)
}

func formatOMPDisplayThinking(values []string) string {
	if len(values) == 0 {
		return "unknown"
	}
	return strings.Join(values, ",")
}
