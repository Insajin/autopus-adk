package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

const workflowContextObserveCallMaxRequestBytes = 256 << 10

var workflowContextObserveProviderPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$`)

type workflowContextObserveCallEntrypoint func(
	context.Context,
	workflowContextObserveCallRequest,
	workflowContextObserveCallOptions,
) (workflowContextObserveCallResult, error)

type workflowContextObserveCallEntrypointKey struct{}

func newWorkflowContextObserveCallCmd() *cobra.Command {
	var requestJSON, output, format string
	options := workflowContextObserveCallOptions{}
	cmd := &cobra.Command{
		Use:           "observe-call",
		Short:         "Run one explicit observer-canary OMP provider call",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if requestJSON != "-" || output != "-" || strings.ToLower(strings.TrimSpace(format)) != "json" {
				return errors.New("workflow context-runtime observe-call requires --request-json - --output - --format json")
			}
			request, err := decodeWorkflowContextObserveCallRequest(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("workflow context-runtime observe-call: %w", err)
			}
			if err := validateWorkflowContextObserveCallOptions(options); err != nil {
				return fmt.Errorf("workflow context-runtime observe-call: %w", err)
			}
			entrypoint := workflowContextObserveCallEntrypoint(RunWorkflowContextObserveCall)
			if injected, ok := cmd.Context().Value(workflowContextObserveCallEntrypointKey{}).(workflowContextObserveCallEntrypoint); ok && injected != nil {
				entrypoint = injected
			}
			result, err := entrypoint(cmd.Context(), request, options)
			if err != nil {
				return fmt.Errorf("workflow context-runtime observe-call failed closed: %w", err)
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(result)
		},
	}
	cmd.Flags().StringVar(&requestJSON, "request-json", "", "Read one strict observer request from stdin (-)")
	cmd.Flags().StringVar(&options.ProjectDir, "project-dir", ".", "Candidate project root")
	cmd.Flags().StringVar(&options.SpecID, "spec-id", "", "SPEC identity used for canonical delivery")
	cmd.Flags().StringVar(&options.Provider, "provider", "", "Broker provider identity")
	cmd.Flags().StringVar(&options.Model, "model", "", "Broker model identity")
	cmd.Flags().StringVar(&options.Endpoint, "endpoint", "", "Task-owned loopback broker endpoint")
	cmd.Flags().StringVar(&options.CredentialLocator, "credential-locator", "", "Dedicated credential environment key")
	cmd.Flags().StringVar(&options.Executable, "omp", "omp", "OMP executable")
	cmd.Flags().StringVar(&output, "output", "", "Observer result output (-)")
	cmd.Flags().StringVar(&format, "format", "json", "Output format (json)")
	return cmd
}

func decodeWorkflowContextObserveCallRequest(reader io.Reader) (workflowContextObserveCallRequest, error) {
	data, err := io.ReadAll(io.LimitReader(reader, workflowContextObserveCallMaxRequestBytes+1))
	if err != nil || len(data) > workflowContextObserveCallMaxRequestBytes {
		return workflowContextObserveCallRequest{}, errors.New("observer request is unavailable")
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return workflowContextObserveCallRequest{}, errors.New("observer request must contain one object")
	}
	if err := rejectDuplicatePipelineOMPJSON(trimmed); err != nil {
		return workflowContextObserveCallRequest{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	var request workflowContextObserveCallRequest
	if err := decoder.Decode(&request); err != nil {
		return request, fmt.Errorf("decode observer request: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return request, errors.New("observer request contains trailing JSON")
	}
	if request.SchemaVersion != workflowContextObserveCallRequestSchema || request.Sequence < 1 || request.Sequence > 40 ||
		request.PairSequence < 1 || request.PairSequence > 2 || !validPipelineOMPActiveHash(request.TaskID) ||
		(request.Variant != "full" && request.Variant != "optimized") || strings.TrimSpace(request.Prompt) == "" ||
		len(request.Prompt) > workflowContextManagedProductMaxPromptBytes || strings.ContainsRune(request.Prompt, 0) ||
		strings.Contains(request.Prompt, "AUTOPUS_PROVIDER_PHASE=") || strings.Contains(request.Prompt, `"mode":"optimized"`) ||
		strings.Contains(request.Prompt, `"mode":"canonical_full"`) {
		return request, errors.New("observer request identity or prompt is invalid")
	}
	return request, nil
}

func validateWorkflowContextObserveCallOptions(options workflowContextObserveCallOptions) error {
	if !workflowContextObserveProviderPattern.MatchString(options.Provider) ||
		!workflowContextObserveProviderPattern.MatchString(options.Model) ||
		!workflowContextMetadataPattern.MatchString(options.SpecID) ||
		!pipelineOMPContextCohortLocatorPattern.MatchString(options.CredentialLocator) ||
		strings.TrimSpace(options.Endpoint) == "" || strings.TrimSpace(options.Executable) == "" {
		return errors.New("observer runtime coordinates are invalid")
	}
	return nil
}
