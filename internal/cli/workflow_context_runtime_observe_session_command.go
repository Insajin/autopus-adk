package cli

import (
	"errors"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

func newWorkflowContextObserveSessionCmd() *cobra.Command {
	var input, output, format string
	var explicitLive, inheritParentSandbox bool
	options := workflowContextObserveSessionOptions{ModelContextWindow: pipelineOMPActiveDefaultContextWindow}
	cmd := &cobra.Command{
		Use:   "observe-session",
		Short: "Run a production explicit-live OMP cohort and write body-free promotion evidence",
		Long: "Run exactly 20 balanced AB/BA task pairs through two reusable installed OMP sessions. " +
			"The endpoint must be an exact loopback gateway; the upstream credential stays in the named environment variable. " +
			"On success the command writes .autopus/runtime/omp-context/promotion-report-v1.json and evidence-v1.json. " +
			"On failure it writes one body-free error frame and returns non-zero. " +
			"The report remains inactive until a trusted promotion-attestation.v2 and release lineage are installed.",
		Example: "  auto workflow context-runtime observe-session --explicit-live --input-jsonl - --output - --format jsonl \\\n" +
			"    --project-dir . --spec-id SPEC-OMP-004 --provider openai --model gpt-5.6-sol \\\n" +
			"    --model-context-window 272000 --endpoint http://127.0.0.1:43123 \\\n" +
			"    --credential-locator AUTOPUS_OMP_CONTEXT_PROVIDER_OPENAI \\\n" +
			"    --producer-repository org/omp-evals --producer-workflow-ref refs/heads/main@REV \\\n" +
			"    --producer-run-id 123 --producer-run-attempt 1 --candidate-repository org/autopus-adk \\\n" +
			"    --policy-id omp-context-active-v1 --oracle-policy-digest sha256:DIGEST --target-git-commit REV",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !explicitLive {
				return errors.New("workflow context-runtime observe-session requires --explicit-live")
			}
			if input != "-" || output != "-" || strings.ToLower(strings.TrimSpace(format)) != "jsonl" {
				return errors.New("workflow context-runtime observe-session requires --input-jsonl - --output - --format jsonl")
			}
			if err := populateWorkflowContextObserveSessionPolicy(&options); err != nil {
				return err
			}
			options.SandboxMode = workflowContextObserveSessionSandboxMode(inheritParentSandbox)
			if err := validateWorkflowContextObserveSessionOptions(options); err != nil {
				return err
			}
			return RunWorkflowContextObserveSession(cmd.Context(), cmd.InOrStdin(), cmd.OutOrStdout(), options)
		},
	}
	cmd.Flags().BoolVar(&explicitLive, "explicit-live", false, "Acknowledge external provider execution")
	cmd.Flags().BoolVar(&inheritParentSandbox, "inherit-parent-sandbox", false, "Reuse the producer-owned Darwin sandbox")
	cmd.Flags().StringVar(&input, "input-jsonl", "", "Read strict handshake/call/shutdown JSONL from stdin (-)")
	cmd.Flags().StringVar(&output, "output", "", "Write strict response JSONL (-)")
	cmd.Flags().StringVar(&format, "format", "jsonl", "Output format (jsonl)")
	cmd.Flags().StringVar(&options.ProjectDir, "project-dir", ".", "Candidate project root")
	cmd.Flags().StringVar(&options.SpecID, "spec-id", "", "SPEC identity")
	cmd.Flags().StringVar(&options.Provider, "provider", "", "Signed cohort provider identity")
	cmd.Flags().StringVar(&options.Model, "model", "", "Signed cohort model identity")
	cmd.Flags().IntVar(&options.ModelContextWindow, "model-context-window",
		pipelineOMPActiveDefaultContextWindow, "Provider model context window bound into runtime authority")
	cmd.Flags().StringVar(&options.Endpoint, "endpoint", "", "Task-owned loopback broker endpoint")
	cmd.Flags().StringVar(&options.CredentialLocator, "credential-locator", "", "Dedicated credential environment key")
	cmd.Flags().StringVar(&options.Executable, "omp", "omp", "OMP executable")
	cmd.Flags().StringVar(&options.TargetGitCommit, "target-git-commit", "", "Exact target project commit")
	cmd.Flags().StringVar(&options.ProducerRepository, "producer-repository", "", "Authorized evidence producer repository")
	cmd.Flags().StringVar(&options.ProducerWorkflowRef, "producer-workflow-ref", "", "Immutable evidence workflow reference")
	cmd.Flags().StringVar(&options.ProducerRunID, "producer-run-id", "", "Authorized evidence workflow run ID")
	cmd.Flags().IntVar(&options.ProducerRunAttempt, "producer-run-attempt", 0, "Authorized evidence workflow run attempt")
	cmd.Flags().StringVar(&options.CandidateRepository, "candidate-repository", "", "Candidate source repository")
	cmd.Flags().StringVar(&options.PolicyID, "policy-id", "", "Signed active-history policy identity")
	cmd.Flags().StringVar(&options.OraclePolicyDigest, "oracle-policy-digest", "", "Body-free quality/security oracle digest")
	cmd.Flags().DurationVar(&options.EvidenceValidFor, "evidence-valid-for", time.Hour, "Current-run evidence validity window")
	return cmd
}

func workflowContextObserveSessionSandboxMode(inheritParent bool) pipelineOMPActiveSandboxMode {
	if inheritParent {
		return pipelineOMPActiveSandboxInheritedParent
	}
	return pipelineOMPActiveSandboxManaged
}
