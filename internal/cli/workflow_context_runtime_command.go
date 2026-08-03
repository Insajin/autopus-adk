package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
)

type workflowContextRuntimeCommandOptions struct {
	projectDir string
	format     string
}

type workflowContextRuntimePolicyOutput struct {
	Enabled bool                           `json:"enabled"`
	Policy  WorkflowContextEffectivePolicy `json:"policy"`
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-OMP-004: product session requests are capped at 1 MiB before strict JSON decoding.
const workflowContextProductRequestMaxBytes = 1 << 20

const workflowContextProductIntentSchemaVersion = "workflow-context-product-intent/v1"

type workflowContextProductIntentV1 struct {
	SchemaVersion string `json:"schema_version"`
	OriginalTask  string `json:"original_task"`
	DecisionDelta string `json:"decision_delta"`
}

type workflowContextProductSessionEntrypoint func(
	context.Context, workflowContextProductIntentV1,
) (WorkflowContextRuntimeReceipt, error)

type workflowContextProductSessionEntrypointKey struct{}

func withWorkflowContextProductSessionEntrypoint(
	ctx context.Context, entrypoint workflowContextProductSessionEntrypoint,
) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, workflowContextProductSessionEntrypointKey{}, entrypoint)
}

func newWorkflowContextRuntimeCmd() *cobra.Command {
	opts := workflowContextRuntimeCommandOptions{}
	cmd := &cobra.Command{
		Use:           "context-runtime",
		Short:         "Inspect the effective OMP context runtime supervisor policy",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.ToLower(strings.TrimSpace(opts.format)) != "json" {
				return fmt.Errorf("workflow context-runtime: unsupported format %q", opts.format)
			}
			cfg, err := loadHarnessConfigForDir(opts.projectDir, globalFlags{})
			if err != nil {
				return fmt.Errorf("workflow context-runtime: %w", err)
			}
			policy, enabled, err := workflowContextPolicyFromConfig(cfg)
			if err != nil {
				return fmt.Errorf("workflow context-runtime: %w", err)
			}
			data, err := json.Marshal(workflowContextRuntimePolicyOutput{Enabled: enabled, Policy: policy})
			if err != nil {
				return fmt.Errorf("workflow context-runtime: encode policy: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.projectDir, "project-dir", ".", "Project root containing autopus.yaml")
	cmd.Flags().StringVar(&opts.format, "format", "json", "Output format (json)")
	cmd.AddCommand(newWorkflowContextProductSessionCmd())
	return cmd
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: process-private CLI boundary for versioned product intent.
// @AX:REASON [AUTO]: Cobra registration and external callers depend on stdin-only decoding and injected runtime authority remaining fail-closed.
func newWorkflowContextProductSessionCmd() *cobra.Command {
	var requestJSON, format string
	cmd := &cobra.Command{
		Use:           "session",
		Short:         "Run an authoritative managed OMP context session",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.ToLower(strings.TrimSpace(format)) != "json" {
				return fmt.Errorf("workflow context-runtime session: unsupported format %q", format)
			}
			if requestJSON != "-" {
				return fmt.Errorf("workflow context-runtime session: --request-json must be - (stdin)")
			}
			intent, err := decodeWorkflowContextProductIntent(cmd.InOrStdin())
			if err != nil {
				return fmt.Errorf("workflow context-runtime session: %w", err)
			}
			entrypoint, injected := workflowContextInjectedEntrypoint(cmd.Context())
			if !injected {
				return fmt.Errorf("workflow context-runtime session: authoritative runtime context is unavailable")
			}
			receipt, err := entrypoint(cmd.Context(), intent)
			if err != nil {
				return err
			}
			return json.NewEncoder(cmd.OutOrStdout()).Encode(receipt)
		},
	}
	cmd.Flags().StringVar(&requestJSON, "request-json", "", "Read one product session request from stdin (-)")
	cmd.Flags().StringVar(&format, "format", "json", "Output format (json)")
	return cmd
}

// @AX:WARN [AUTO]: product intent decoding has nine fail-closed if branches.
// @AX:REASON [AUTO]: size, object shape, duplicate keys, unknown fields, trailing data, schema, and prompt presence are one admission boundary.
func decodeWorkflowContextProductIntent(reader io.Reader) (workflowContextProductIntentV1, error) {
	data, err := io.ReadAll(io.LimitReader(reader, workflowContextProductRequestMaxBytes+1))
	if err != nil {
		return workflowContextProductIntentV1{}, fmt.Errorf("read request JSON: %w", err)
	}
	if len(data) > workflowContextProductRequestMaxBytes {
		return workflowContextProductIntentV1{}, fmt.Errorf("request JSON exceeds %d bytes", workflowContextProductRequestMaxBytes)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return workflowContextProductIntentV1{}, fmt.Errorf("request JSON must contain exactly one object")
	}
	if err := rejectWorkflowContextProductDuplicateKeys(trimmed); err != nil {
		return workflowContextProductIntentV1{}, err
	}
	decoder := json.NewDecoder(bytes.NewReader(trimmed))
	decoder.DisallowUnknownFields()
	var intent workflowContextProductIntentV1
	if err := decoder.Decode(&intent); err != nil {
		return workflowContextProductIntentV1{}, fmt.Errorf("decode request JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return workflowContextProductIntentV1{}, fmt.Errorf("request JSON must contain exactly one object")
		}
		return workflowContextProductIntentV1{}, fmt.Errorf("decode trailing request JSON: %w", err)
	}
	if intent.SchemaVersion != workflowContextProductIntentSchemaVersion {
		return workflowContextProductIntentV1{}, fmt.Errorf("unsupported product intent schema %q", intent.SchemaVersion)
	}
	if strings.TrimSpace(intent.OriginalTask) == "" || strings.TrimSpace(intent.DecisionDelta) == "" {
		return workflowContextProductIntentV1{}, fmt.Errorf("product intent requires original_task and decision_delta")
	}
	return intent, nil
}

func rejectWorkflowContextProductDuplicateKeys(data []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := scanWorkflowContextProductJSONValue(decoder); err != nil {
		return fmt.Errorf("decode request JSON: %w", err)
	}
	if _, err := decoder.Token(); err != io.EOF {
		if err == nil {
			return fmt.Errorf("request JSON must contain exactly one object")
		}
		return fmt.Errorf("decode trailing request JSON: %w", err)
	}
	return nil
}

func scanWorkflowContextProductJSONValue(decoder *json.Decoder) error {
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delim, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delim {
	case '{':
		seen := map[string]struct{}{}
		for decoder.More() {
			keyToken, err := decoder.Token()
			if err != nil {
				return err
			}
			key, ok := keyToken.(string)
			if !ok {
				return fmt.Errorf("object key is invalid")
			}
			if _, duplicate := seen[key]; duplicate {
				return fmt.Errorf("duplicate object key %q", key)
			}
			seen[key] = struct{}{}
			if err := scanWorkflowContextProductJSONValue(decoder); err != nil {
				return err
			}
		}
	case '[':
		for decoder.More() {
			if err := scanWorkflowContextProductJSONValue(decoder); err != nil {
				return err
			}
		}
	default:
		return fmt.Errorf("unexpected JSON delimiter %q", delim)
	}
	_, err = decoder.Token()
	return err
}

func workflowContextProductEntrypoint(ctx context.Context) workflowContextProductSessionEntrypoint {
	if entrypoint, ok := workflowContextInjectedEntrypoint(ctx); ok {
		return entrypoint
	}
	return func(context.Context, workflowContextProductIntentV1) (WorkflowContextRuntimeReceipt, error) {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("authoritative runtime context is unavailable")
	}
}

func workflowContextInjectedEntrypoint(ctx context.Context) (workflowContextProductSessionEntrypoint, bool) {
	if ctx == nil {
		return nil, false
	}
	entrypoint, ok := ctx.Value(workflowContextProductSessionEntrypointKey{}).(workflowContextProductSessionEntrypoint)
	return entrypoint, ok && entrypoint != nil
}
