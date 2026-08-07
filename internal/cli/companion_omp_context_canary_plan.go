package cli

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

type companionOMPContextCanaryPlanOptions struct {
	projectDir            string
	inputOutput           string
	challengeDigest       string
	producerRepository    string
	producerWorkflowRef   string
	candidateRepository   string
	sourceCommit          string
	sourceTree            string
	target                string
	autoVersion           string
	provider              string
	model                 string
	policyID              string
	oraclePolicyDigest    string
	ompVersion            string
	ompExecutableSHA256   string
	releaseLineageKeyID   string
	releaseLineageHandoff string
	minimumRollbackFloor  uint64
}

func newCompanionOMPContextCanaryPlanCmd() *cobra.Command {
	var options companionOMPContextCanaryPlanOptions
	command := &cobra.Command{
		Use:          "omp-context-canary-plan",
		Short:        "Build deterministic release canary input and its static policy",
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(command *cobra.Command, _ []string) error {
			return runCompanionOMPContextCanaryPlan(command, options)
		},
	}
	flags := command.Flags()
	flags.StringVar(&options.projectDir, "project-dir", "", "Canonical release project root")
	flags.StringVar(&options.inputOutput, "input-output", "", "New private observe-session JSONL file")
	flags.StringVar(&options.challengeDigest, "challenge-digest", "", "Canonical canary challenge digest")
	flags.StringVar(&options.producerRepository, "producer-repository", "", "Evidence producer repository")
	flags.StringVar(&options.producerWorkflowRef, "producer-workflow-ref", "", "Evidence producer workflow reference")
	flags.StringVar(&options.candidateRepository, "candidate-repository", "", "Candidate repository")
	flags.StringVar(&options.sourceCommit, "source-commit", "", "Candidate source commit")
	flags.StringVar(&options.sourceTree, "source-tree", "", "Candidate source tree")
	flags.StringVar(&options.target, "target", "", "Release target")
	flags.StringVar(&options.autoVersion, "auto-version", "", "Candidate auto version")
	flags.StringVar(&options.provider, "provider", "", "Provider identity")
	flags.StringVar(&options.model, "model", "", "Provider model identity")
	flags.StringVar(&options.policyID, "policy-id", "", "Active policy identity")
	flags.StringVar(&options.oraclePolicyDigest, "oracle-policy-digest", "", "Quality oracle digest")
	flags.StringVar(&options.ompVersion, "omp-version", "", "Pinned OMP version")
	flags.StringVar(&options.ompExecutableSHA256, "omp-executable-sha256", "", "Pinned OMP executable digest")
	flags.StringVar(&options.releaseLineageKeyID, "release-lineage-key-id", "", "Release lineage key identity")
	flags.StringVar(&options.releaseLineageHandoff, "release-lineage-handoff", "", "Release lineage handoff")
	flags.Uint64Var(&options.minimumRollbackFloor, "minimum-rollback-floor", 0, "Minimum release rollback floor")
	for _, name := range []string{
		"project-dir", "input-output", "challenge-digest", "producer-repository", "producer-workflow-ref",
		"candidate-repository", "source-commit", "source-tree", "target", "auto-version", "provider", "model",
		"policy-id", "oracle-policy-digest", "omp-version", "omp-executable-sha256",
		"release-lineage-key-id", "release-lineage-handoff", "minimum-rollback-floor",
	} {
		_ = command.MarkFlagRequired(name)
	}
	return command
}

func runCompanionOMPContextCanaryPlan(command *cobra.Command, options companionOMPContextCanaryPlanOptions) error {
	commands, tasks, err := workflowContextObserveSessionCanaryPlan(options.challengeDigest)
	if err != nil {
		return err
	}
	observeOptions := workflowContextObserveSessionOptions{ProjectDir: options.projectDir}
	if err := populateWorkflowContextObserveSessionPolicy(&observeOptions); err != nil {
		return err
	}
	policyDigest, err := promptlayer.OMPContextPromotionPolicyDigestV1(observeOptions.PromotionPolicy)
	if err != nil {
		return errors.New("derive release canary policy digest")
	}
	providerScope, modelScopeDigest, err := pipelineOMPActiveModelScope(
		workflowContextObserveSessionPhaseModels(options.provider + "/" + options.model),
	)
	if err != nil || providerScope != options.provider {
		return errors.New("derive release canary model scope")
	}
	taskBytes, err := json.Marshal(tasks)
	if err != nil {
		return errors.New("encode release canary task manifest")
	}
	cohortDigest := workflowContextRuntimeHash(string(taskBytes))
	policyBytes, err := promptlayer.MarshalOMPContextPromotionStaticPolicyV3(
		promptlayer.OMPContextPromotionStaticPolicyV3{
			SchemaVersion:      promptlayer.OMPContextPromotionRuntimeSchemaV3,
			ProducerRepository: options.producerRepository, ProducerWorkflowRef: options.producerWorkflowRef,
			CandidateRepository: options.candidateRepository, SourceCommit: options.sourceCommit,
			SourceTree: options.sourceTree, Target: options.target, AutoVersion: options.autoVersion,
			PolicyID: options.policyID, PolicyDigest: policyDigest, OMPVersion: options.ompVersion,
			OMPExecutableSHA256:          options.ompExecutableSHA256,
			PipelineImplementationDigest: pipelineOMPActiveImplementationDigest(), Provider: options.provider,
			ModelScopeDigest: modelScopeDigest, CohortManifestDigest: cohortDigest, OrderSeed: cohortDigest,
			OraclePolicyDigest: options.oraclePolicyDigest, ReleaseLineageKeyID: options.releaseLineageKeyID,
			ReleaseLineageHandoff: options.releaseLineageHandoff, MinimumRollbackFloor: options.minimumRollbackFloor,
		},
	)
	if err != nil {
		return err
	}
	var input bytes.Buffer
	encoder := json.NewEncoder(&input)
	encoder.SetEscapeHTML(false)
	for _, value := range commands {
		if err := encoder.Encode(value); err != nil {
			return errors.New("encode release canary input")
		}
	}
	if err := writeNewPrivateOMPContextPromotionAttestation(options.inputOutput, input.Bytes()); err != nil {
		return fmt.Errorf("write release canary input: %w", err)
	}
	if _, err := fmt.Fprintln(command.OutOrStdout(), base64.RawURLEncoding.EncodeToString(policyBytes)); err != nil {
		return errors.New("write release canary static policy")
	}
	return nil
}

func workflowContextObserveSessionCanaryPlan(challenge string) (
	[]workflowContextObserveSessionCommand,
	[]promptlayer.OMPContextPromotionTaskV1,
	error,
) {
	if !validPipelineOMPActiveHash(challenge) {
		return nil, nil, errors.New("release canary challenge digest is invalid")
	}
	commands := []workflowContextObserveSessionCommand{{
		SchemaVersion: workflowContextObserveSessionCommandSchema,
		Type:          "handshake", ChallengeDigest: challenge,
	}}
	tasks := make([]promptlayer.OMPContextPromotionTaskV1, 0, 20)
	sequence := 0
	for taskIndex := range 20 {
		taskID := workflowContextRuntimeHash(challenge + ":task:" + strconv.Itoa(taskIndex))
		prompt := fmt.Sprintf(
			"Return exactly EVALUATION-%02d-%d on one line. Do not call tools, execute commands, or add any other text.",
			taskIndex, taskIndex%7,
		)
		order := "AB"
		if taskIndex%2 == 1 {
			order = "BA"
		}
		tasks = append(tasks, promptlayer.OMPContextPromotionTaskV1{TaskIDDigest: taskID, Order: order})
		for pairIndex, variant := range order {
			sequence++
			commands = append(commands, workflowContextObserveSessionCommand{
				SchemaVersion: workflowContextObserveSessionCommandSchema, Type: "call",
				Sequence: sequence, PairSequence: pairIndex + 1, TaskIDDigest: taskID,
				Variant: string(variant), Prompt: prompt,
			})
		}
	}
	commands = append(commands, workflowContextObserveSessionCommand{
		SchemaVersion: workflowContextObserveSessionCommandSchema,
		Type:          "shutdown",
	})
	return commands, tasks, nil
}
