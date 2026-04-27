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
				results[idx] = malformedStructuredOutcome(provider.Name, fmt.Errorf("execution failed: %w", execErr), nil)
				return
			}
			if resp == nil {
				results[idx] = malformedStructuredOutcome(provider.Name, fmt.Errorf("provider returned nil response"), nil)
				return
			}
			if resp.TimedOut {
				results[idx] = malformedStructuredOutcome(provider.Name, fmt.Errorf("provider timed out after %s", req.Timeout), resp)
				return
			}
			if resp.EmptyOutput {
				results[idx] = malformedStructuredOutcome(provider.Name, fmt.Errorf("provider returned empty output"), resp)
				return
			}
			if _, parseErr := parser.ParseReviewer(resp.Output); parseErr != nil {
				results[idx] = malformedStructuredOutcome(provider.Name, fmt.Errorf("invalid reviewer JSON: %w", parseErr), resp)
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

func malformedStructuredOutcome(provider string, err error, sourceResp *orchestra.ProviderResponse) specReviewStructuredOutcome {
	description := structuredFailureDescription(err, sourceResp)
	response := orchestra.ProviderResponse{
		Provider: provider,
		Output:   synthesizeMalformedReviewJSON(provider, description),
		Error:    description,
	}
	failed := &orchestra.FailedProvider{
		Name:            provider,
		Error:           description,
		FailureClass:    structuredFailureClass(err),
		NextRemediation: structuredFailureRemediation(err, provider),
		CollectionMode:  "subprocess_stdout",
	}
	if sourceResp != nil {
		failed.StderrPreview = truncateStructuredReviewError(sourceResp.Error, 240)
		failed.OutputPreview = truncateStructuredReviewError(sourceResp.Output, 240)
	}
	return specReviewStructuredOutcome{resp: response, failed: failed}
}

func synthesizeMalformedReviewJSON(provider, description string) string {
	payload := orchestra.ReviewerOutput{
		Verdict: "REVISE",
		Summary: fmt.Sprintf("Malformed or incomplete review output from %s", provider),
		Findings: []orchestra.Finding{{
			Severity:    "major",
			Category:    "completeness",
			ScopeRef:    "provider:" + provider,
			Location:    "provider:" + provider,
			Description: truncateStructuredReviewError(description, 240),
			Suggestion:  structuredFailureRemediationText(description, provider),
		}},
	}
	data, marshalErr := json.Marshal(payload)
	if marshalErr != nil {
		return `{"verdict":"REVISE","summary":"Malformed review output","findings":[{"severity":"major","category":"completeness","scope_ref":"provider:unknown","location":"provider:unknown","description":"failed to serialize malformed review result","suggestion":"Retry the provider."}]}`
	}
	return string(data)
}

func structuredFailureDescription(err error, resp *orchestra.ProviderResponse) string {
	if err == nil {
		return "unknown provider failure"
	}
	parts := []string{err.Error()}
	if resp == nil {
		return strings.Join(parts, "; ")
	}
	if resp.Duration > 0 {
		parts = append(parts, "duration "+resp.Duration.String())
	}
	if strings.TrimSpace(resp.Error) != "" {
		parts = append(parts, "stderr: "+truncateStructuredReviewError(resp.Error, 160))
	}
	if strings.TrimSpace(resp.Output) != "" {
		parts = append(parts, "stdout preview: "+truncateStructuredReviewError(resp.Output, 160))
	}
	return strings.Join(parts, "; ")
}

func structuredFailureClass(err error) string {
	if err == nil {
		return "execution_error"
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "timed out"):
		return "timeout"
	case strings.Contains(msg, "empty output"):
		return "execution_error"
	case strings.Contains(msg, "invalid reviewer json"):
		return "execution_error"
	default:
		return "execution_error"
	}
}

func structuredFailureRemediation(err error, provider string) string {
	if err == nil {
		return "Retry the provider and inspect subprocess diagnostics."
	}
	return structuredFailureRemediationText(err.Error(), provider)
}

func structuredFailureRemediationText(description, provider string) string {
	msg := strings.ToLower(description)
	if strings.Contains(msg, "timed out") {
		return fmt.Sprintf("Increase --timeout or set orchestra.providers.%s.subprocess.timeout, then retry with a smaller review context if needed.", provider)
	}
	if strings.Contains(msg, "invalid reviewer json") {
		return "Retry with stricter JSON-only prompting or provider-specific structured output settings."
	}
	return "Retry the provider with a shorter context or stronger schema enforcement."
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
