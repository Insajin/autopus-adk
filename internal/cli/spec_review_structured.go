package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

type specReviewStructuredOutcome struct {
	resp   orchestra.ProviderResponse
	failed *orchestra.FailedProvider
}

func runStructuredSpecReviewOrchestra(ctx context.Context, cfg orchestra.OrchestraConfig) (*orchestra.OrchestraResult, error) {
	if len(cfg.Providers) == 0 {
		return nil, fmt.Errorf("spec review: no providers configured")
	}

	schema := &orchestra.SchemaBuilder{}
	schemaPath, cleanup, err := schema.WriteToFile("reviewer")
	if err != nil {
		return nil, fmt.Errorf("spec review: reviewer schema: %w", err)
	}
	defer cleanup()

	embeddedSchema, err := schema.EmbedInPrompt("reviewer")
	if err != nil {
		return nil, fmt.Errorf("spec review: embed reviewer schema: %w", err)
	}

	backend := specReviewBackendFactory()
	parser := &orchestra.OutputParser{}
	start := time.Now()

	results := make([]specReviewStructuredOutcome, len(cfg.Providers))
	var wg sync.WaitGroup
	for i, provider := range cfg.Providers {
		wg.Add(1)
		go func(idx int, provider orchestra.ProviderConfig) {
			defer wg.Done()

			prompt := buildStructuredSpecReviewPrompt(cfg.Prompt, embeddedSchema, strings.TrimSpace(provider.SchemaFlag) == "")
			req := orchestra.ProviderRequest{
				Provider:   provider.Name,
				Prompt:     prompt,
				SchemaPath: schemaPath,
				Role:       "reviewer",
				Timeout:    specReviewTimeout(provider, cfg.TimeoutSeconds),
				Config:     provider,
			}

			resp, execErr := backend.Execute(ctx, req)
			if execErr != nil {
				results[idx] = malformedStructuredOutcome(provider.Name, fmt.Errorf("execution failed: %w", execErr))
				return
			}
			if resp == nil {
				results[idx] = malformedStructuredOutcome(provider.Name, fmt.Errorf("provider returned nil response"))
				return
			}
			if resp.TimedOut {
				results[idx] = malformedStructuredOutcome(provider.Name, fmt.Errorf("provider timed out"))
				return
			}
			if resp.EmptyOutput {
				results[idx] = malformedStructuredOutcome(provider.Name, fmt.Errorf("provider returned empty output"))
				return
			}
			if _, parseErr := parser.ParseReviewer(resp.Output); parseErr != nil {
				results[idx] = malformedStructuredOutcome(provider.Name, fmt.Errorf("invalid reviewer JSON: %w", parseErr))
				return
			}

			results[idx] = specReviewStructuredOutcome{resp: *resp}
		}(i, provider)
	}
	wg.Wait()

	responses := make([]orchestra.ProviderResponse, 0, len(results))
	failed := make([]orchestra.FailedProvider, 0)
	for _, result := range results {
		responses = append(responses, result.resp)
		if result.failed != nil {
			failed = append(failed, *result.failed)
		}
	}

	return &orchestra.OrchestraResult{
		Strategy:        cfg.Strategy,
		Responses:       responses,
		Duration:        time.Since(start),
		Summary:         fmt.Sprintf("structured spec review: %d providers", len(responses)),
		FailedProviders: failed,
	}, nil
}

func buildStructuredSpecReviewPrompt(basePrompt, schemaJSON string, inlineSchema bool) string {
	var sb strings.Builder
	sb.WriteString(basePrompt)
	sb.WriteString("\n\n### Structured Response Contract\n\n")
	sb.WriteString("Return ONLY valid JSON. Do NOT return progress notes, partial summaries, markdown fences, or prose before/after the JSON.\n")
	sb.WriteString("If you are blocked or the scope is too large, still return valid JSON with `verdict: \"REVISE\"` and describe the blocker in `summary` plus at least one finding.\n")
	sb.WriteString("Use these fields:\n")
	sb.WriteString("- `verdict`: `PASS`, `REVISE`, or `REJECT`\n")
	sb.WriteString("- `summary`: concise explanation of the overall verdict\n")
	sb.WriteString("- `findings`: array of `{severity, category, scope_ref, location, description, suggestion}`\n")
	sb.WriteString("- `checklist`: array of `{id, status, reason}` where `status` is `PASS` or `FAIL`\n")
	sb.WriteString("- `finding_statuses`: array of `{id, status, reason}` where `status` is `open`, `resolved`, or `regressed`\n")
	if inlineSchema {
		sb.WriteString("\nRequired JSON schema:\n```json\n")
		sb.WriteString(schemaJSON)
		sb.WriteString("\n```\n")
	}
	return sb.String()
}

func malformedStructuredOutcome(provider string, err error) specReviewStructuredOutcome {
	resp := orchestra.ProviderResponse{
		Provider: provider,
		Output:   synthesizeMalformedReviewJSON(provider, err),
		Error:    err.Error(),
	}
	failed := &orchestra.FailedProvider{
		Name:  provider,
		Error: err.Error(),
	}
	return specReviewStructuredOutcome{resp: resp, failed: failed}
}

func synthesizeMalformedReviewJSON(provider string, err error) string {
	payload := orchestra.ReviewerOutput{
		Verdict: "REVISE",
		Summary: fmt.Sprintf("Malformed or incomplete review output from %s", provider),
		Findings: []orchestra.Finding{{
			Severity:    "major",
			Category:    "completeness",
			ScopeRef:    "provider:" + provider,
			Location:    "provider:" + provider,
			Description: truncateStructuredReviewError(err.Error(), 240),
			Suggestion:  "Retry the provider with a shorter context or stronger schema enforcement.",
		}},
	}
	data, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return `{"verdict":"REVISE","summary":"Malformed review output","findings":[{"severity":"major","category":"completeness","scope_ref":"provider:unknown","location":"provider:unknown","description":"failed to serialize malformed review result","suggestion":"Retry the provider."}]}`
	}
	return string(data)
}

func truncateStructuredReviewError(s string, max int) string {
	s = strings.TrimSpace(s)
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}

func specReviewTimeout(provider orchestra.ProviderConfig, fallbackSeconds int) time.Duration {
	if provider.ExecutionTimeout > 0 {
		return provider.ExecutionTimeout
	}
	if fallbackSeconds > 0 {
		return time.Duration(fallbackSeconds) * time.Second
	}
	return 120 * time.Second
}
