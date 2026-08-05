package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/adapter/omp"
)

type ompCatalogPayload struct {
	Platform           string                   `json:"platform"`
	Status             string                   `json:"status"`
	Reason             string                   `json:"reason"`
	Version            string                   `json:"version,omitempty"`
	Fingerprint        string                   `json:"fingerprint,omitempty"`
	StrictRoutingReady bool                     `json:"strict_routing_ready"`
	Models             []ompCatalogModelPayload `json:"models"`
}

type ompCatalogModelPayload struct {
	Provider              string   `json:"provider"`
	Model                 string   `json:"model"`
	Selector              string   `json:"selector"`
	Name                  string   `json:"name,omitempty"`
	Family                string   `json:"family,omitempty"`
	Capabilities          []string `json:"capabilities,omitempty"`
	Thinking              []string `json:"thinking,omitempty"`
	AuthAvailability      string   `json:"auth_availability"`
	Disabled              bool     `json:"disabled,omitempty"`
	SemanticMetadataReady bool     `json:"semantic_metadata_ready"`
	ContextWindow         int64    `json:"context_window,omitempty"`
	Reasoning             *bool    `json:"reasoning,omitempty"`
	NativeThinking        []string `json:"native_thinking,omitempty"`
	Input                 []string `json:"input,omitempty"`
}

type ompCatalogUnavailableError struct {
	reason string
}

func (e ompCatalogUnavailableError) Error() string {
	return "OMP model catalog unavailable: " + e.reason
}

func probeInstalledOMPCatalog(
	ctx context.Context,
	runner omp.OMPModelCatalogRunner,
) (omp.OMPModelCatalogProbeResult, error) {
	result := omp.ProbeOMPModelCatalog(ctx, omp.OMPModelCatalogProbeOptions{
		Executable: "omp",
		Runner:     runner,
		Timeout:    ompDoctorProbeTimeout,
		MaxOutput:  ompModelDoctorProbeOutput,
	})
	if result.Status != "ready" || result.Reason != "catalog_ready" {
		reason := safeOMPOperatorReason(result.Reason)
		if reason == "not_available" {
			reason = "catalog_invalid"
		}
		return result, ompCatalogUnavailableError{reason: reason}
	}
	return result, nil
}

func runOMPModelsCommand(
	cmd *cobra.Command,
	_ string,
	jsonMode bool,
	runner omp.OMPModelCatalogRunner,
) error {
	result, err := probeInstalledOMPCatalog(cmd.Context(), runner)
	if err != nil && result.Reason == ompRawCatalogFallbackReason {
		return runOMPRawDisplayCatalogCommand(cmd, jsonMode, runner, result)
	}
	payload := catalogPayloadFromProbe(result)
	if err != nil {
		if jsonMode {
			return writeJSONResultAndExit(
				cmd,
				jsonStatusError,
				err,
				"omp_catalog_unavailable",
				payload,
				[]jsonMessage{{Code: "omp_catalog_unavailable", Message: payload.Reason}},
				nil,
			)
		}
		return err
	}
	if jsonMode {
		return writeJSONResult(cmd, jsonStatusOK, payload, nil, []jsonCheck{{
			ID: "omp.catalog", Severity: "info", Status: "pass", Detail: "catalog_ready",
		}})
	}
	return renderOMPModelsText(cmd, payload)
}

func catalogPayloadFromProbe(result omp.OMPModelCatalogProbeResult) ompCatalogPayload {
	models := []ompCatalogModelPayload{}
	strictRoutingReady := result.Status == "ready" && result.Reason == "catalog_ready"
	if strictRoutingReady {
		models = make([]ompCatalogModelPayload, 0, len(result.Catalog.Models))
		for _, model := range result.Catalog.Models {
			models = append(models, ompCatalogModelPayload{
				Provider: model.Provider, Model: model.Model, Selector: model.Provider + "/" + model.Model,
				Family: model.Family, Capabilities: append([]string(nil), model.Capabilities...),
				Thinking: append([]string(nil), model.Thinking...), AuthAvailability: ompModelAvailability(model),
				Disabled: model.Disabled, SemanticMetadataReady: true,
			})
		}
	}
	return ompCatalogPayload{
		Platform: "omp", Status: result.Status, Reason: safeOMPOperatorReason(result.Reason),
		Version: safeOMPOperatorVersion(result.Version), Fingerprint: result.Catalog.Fingerprint,
		StrictRoutingReady: strictRoutingReady, Models: models,
	}
}

func renderOMPModelsText(cmd *cobra.Command, payload ompCatalogPayload) error {
	out := cmd.OutOrStdout()
	_, _ = fmt.Fprintln(out, "OMP installed model catalog")
	_, _ = fmt.Fprintf(out, "Status: %s (%s)\n", payload.Status, payload.Reason)
	_, _ = fmt.Fprintf(out, "Version: %s\n", payload.Version)
	_, _ = fmt.Fprintf(out, "Strict routing ready: %t\n", payload.StrictRoutingReady)
	if payload.Fingerprint != "" {
		_, _ = fmt.Fprintf(out, "Fingerprint: %s\n", payload.Fingerprint)
	}
	_, _ = fmt.Fprintf(out, "Models: %d\n", len(payload.Models))
	for _, model := range payload.Models {
		if model.SemanticMetadataReady {
			_, _ = fmt.Fprintf(
				out,
				"- %s family=%s availability=%s capabilities=%s thinking=%s\n",
				model.Selector,
				model.Family,
				model.AuthAvailability,
				strings.Join(model.Capabilities, ","),
				strings.Join(model.Thinking, ","),
			)
			continue
		}
		_, _ = fmt.Fprintf(
			out,
			"- %s name=%q context_window=%d reasoning=%s native_thinking=%s input=%s semantic_metadata=unavailable\n",
			model.Selector,
			model.Name,
			model.ContextWindow,
			formatOMPDisplayBool(model.Reasoning),
			formatOMPDisplayThinking(model.NativeThinking),
			strings.Join(model.Input, ","),
		)
	}
	if !payload.StrictRoutingReady && len(payload.Models) != 0 {
		_, _ = fmt.Fprintln(out, "Routing/profile apply: blocked (catalog_metadata_insufficient)")
	}
	return nil
}

func ompModelAvailability(model omp.OMPModelMetadata) string {
	switch {
	case model.Disabled:
		return "disabled"
	case !model.Keyless && !model.AuthEnabled:
		return "unauthorized"
	default:
		return "available"
	}
}
