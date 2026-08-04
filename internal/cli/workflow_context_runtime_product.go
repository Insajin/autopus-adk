package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

// WorkflowContextProductSessionInput is the process-private authoritative input
// for one user-facing managed OMP context session.
type WorkflowContextProductSessionInput struct {
	ProjectDir       string
	Command          string
	DeliveryCommand  string
	SpecDir          string
	SpecID           string
	TaskID           string
	Phase            string
	SessionID        string
	OriginalTask     string
	DecisionDelta    string
	FrozenFindingIDs []string
	OwnershipPaths   []string
	ForbiddenPaths   []string
}

type WorkflowContextManagedDriverFactory func(
	WorkflowContextManagedRPCOptions,
) (WorkflowContextManagedProcessDriver, error)

type workflowContextProductIdentityVerifier interface {
	verifyInstalledIdentity(context.Context, string) error
}

// WorkflowContextProductRuntimeInputs contains runtime evidence and process
// dependencies that cannot be derived from a product request.
type WorkflowContextProductRuntimeInputs struct {
	Capabilities     WorkflowContextCapabilities
	Promotion        promptlayer.OMPContextPromotionEvidenceV1
	History          []promptlayer.OMPContextHistoryRow
	Overlay          WorkflowContextOverlayController
	Supervisor       *WorkflowContextRuntimeSupervisor
	ReceiptWriter    *WorkflowContextReceiptWriter
	DriverOptions    WorkflowContextManagedRPCOptions
	NewManagedDriver WorkflowContextManagedDriverFactory
	ProviderOutput   func(string) error
	ProviderUsage    func(WorkflowContextProviderUsage) error
}

// RunWorkflowContextProductSession assembles canonical product authority and
// enters the managed supervisor boundary without consulting an installed canary.
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: authoritative product entrypoint shared by the CLI and managed-session integrations.
// @AX:REASON [AUTO]: multiple callers depend on policy, binding, canonical-source, and driver construction remaining one fail-closed boundary.
// @AX:WARN [AUTO]: product authority assembly has cyclomatic complexity 18.
// @AX:REASON [AUTO]: gocyclo reports 18 across policy, installed identity, delivery, binding, driver, and supervisor admission.
func RunWorkflowContextProductSession(
	ctx context.Context,
	input WorkflowContextProductSessionInput,
	runtime WorkflowContextProductRuntimeInputs,
) (WorkflowContextRuntimeReceipt, error) {
	input, err := canonicalWorkflowContextProductInput(input)
	if err != nil {
		return WorkflowContextRuntimeReceipt{}, err
	}
	if runtime.Supervisor == nil || runtime.Overlay == nil || runtime.NewManagedDriver == nil {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("workflow context product session: authoritative runtime inputs are unavailable")
	}
	if !installedOMPVersionPattern.MatchString(runtime.Capabilities.Version) {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("workflow context product session: installed OMP identity was not observed")
	}
	cfg, err := loadHarnessConfigForDir(input.ProjectDir, globalFlags{})
	if err != nil {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("workflow context product session: load harness config: %w", err)
	}
	policy, selected, err := workflowContextPolicyFromConfig(cfg)
	if err != nil {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("workflow context product session: load OMP context policy: %w", err)
	}
	if !selected || policy.HistoryMode != config.OMPContextHistoryActive {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("workflow context product session: selected active OMP context policy is required")
	}

	options := promptlayer.ContextDeliveryOptions{
		Root: input.ProjectDir, Command: input.DeliveryCommand, SpecDir: input.SpecDir,
	}
	delivery, err := promptlayer.BuildContextDelivery(options)
	if err != nil {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("workflow context product session: build context delivery: %w", err)
	}
	if err := promptlayer.VerifyContextDeliveryForOptions(options, delivery); err != nil {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("workflow context product session: verify context delivery: %w", err)
	}
	ephemeral := workflowContextProductEphemeral(input)
	binding := promptlayer.OMPContextBindingInput{
		WorkspaceID: cfg.ProjectName, SpecID: input.SpecID, TaskID: input.TaskID,
		Phase: input.Phase, SessionID: input.SessionID, DeliveryOptions: options,
		Delivery: delivery, Ephemeral: ephemeral, History: cloneWorkflowContextProductHistory(runtime.History),
	}
	if _, err := promptlayer.BuildOMPContextBinding(binding); err != nil {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("workflow context product session: build context binding: %w", err)
	}

	driverOptions := runtime.DriverOptions
	driverOptions.ProjectDir = input.ProjectDir
	driverOptions.Prompts = []string{input.OriginalTask, input.DecisionDelta}
	driver, err := runtime.NewManagedDriver(driverOptions)
	if err != nil {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("workflow context product session: construct managed driver: %w", err)
	}
	if driver == nil {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("workflow context product session: managed driver factory returned nil")
	}
	verifier, ok := driver.(workflowContextProductIdentityVerifier)
	if !ok {
		return WorkflowContextRuntimeReceipt{}, rejectWorkflowContextProductDriver(
			driver, errors.New("workflow context product session: managed driver identity verifier is unavailable"),
		)
	}
	if err := verifier.verifyInstalledIdentity(ctx, runtime.Capabilities.Version); err != nil {
		return WorkflowContextRuntimeReceipt{}, rejectWorkflowContextProductDriver(driver, err)
	}

	request := WorkflowContextRuntimeRequest{
		Policy: policy, Capabilities: runtime.Capabilities, Binding: binding,
		RootClass: policy.RuntimeRootPolicy, Driver: driver, Overlay: runtime.Overlay,
		CanonicalSource: workflowContextProductCanonicalSource{options: options, ephemeral: ephemeral},
		Promotion:       runtime.Promotion, ReceiptWriter: runtime.ReceiptWriter,
		ProviderOutput: runtime.ProviderOutput, ProviderUsage: runtime.ProviderUsage,
	}
	return runtime.Supervisor.RunManaged(ctx, request)
}

func canonicalWorkflowContextProductInput(
	input WorkflowContextProductSessionInput,
) (WorkflowContextProductSessionInput, error) {
	projectDir := strings.TrimSpace(input.ProjectDir)
	if projectDir == "" {
		return input, fmt.Errorf("workflow context product session: project directory is required")
	}
	absolute, err := filepath.Abs(projectDir)
	if err != nil {
		return input, fmt.Errorf("workflow context product session: resolve project directory: %w", err)
	}
	canonical := filepath.Clean(absolute)
	info, err := os.Lstat(canonical)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return input, fmt.Errorf("workflow context product session: project directory is invalid")
	}
	configInfo, err := os.Lstat(filepath.Join(canonical, "autopus.yaml"))
	if err != nil || !configInfo.Mode().IsRegular() || configInfo.Mode()&os.ModeSymlink != 0 {
		return input, fmt.Errorf("workflow context product session: project autopus.yaml is required")
	}
	input.ProjectDir = filepath.Clean(canonical)
	input.Command = strings.ToLower(strings.TrimSpace(input.Command))
	input.DeliveryCommand = strings.ToLower(strings.TrimSpace(input.DeliveryCommand))
	if input.DeliveryCommand == "" {
		input.DeliveryCommand = input.Command
	}
	if input.Command != "go" || !validWorkflowContextDeliveryCommand(input.DeliveryCommand) {
		return input, fmt.Errorf("workflow context product session: original or delivery command is invalid")
	}
	input.SpecID = strings.TrimSpace(input.SpecID)
	prompts := []string{input.OriginalTask, input.DecisionDelta}
	if err := validateWorkflowContextManagedProductPrompts(prompts); err != nil {
		return input, fmt.Errorf("workflow context product session: %w", err)
	}
	if err := validateWorkflowContextProductPromptAuthority(input); err != nil {
		return input, err
	}
	input.SpecDir, err = canonicalWorkflowContextProductSpecDir(input)
	if err != nil {
		return input, err
	}
	return input, nil
}

func validWorkflowContextDeliveryCommand(command string) bool {
	switch command {
	case "plan", "go", "test", "review":
		return true
	default:
		return false
	}
}

func validateWorkflowContextProductPromptAuthority(input WorkflowContextProductSessionInput) error {
	fields := strings.Fields(input.OriginalTask)
	if len(fields) >= 3 && fields[0] == "/auto" {
		if fields[1] != input.Command {
			return fmt.Errorf("workflow context product session: prompt command does not match canonical command")
		}
		if fields[2] != input.SpecID {
			return fmt.Errorf("workflow context product session: prompt SPEC does not match canonical SPEC")
		}
		return nil
	}
	if len(fields) < 2 || fields[0] != "/auto-"+input.Command {
		return fmt.Errorf("workflow context product session: prompt command does not match canonical command")
	}
	if fields[1] != input.SpecID {
		return fmt.Errorf("workflow context product session: prompt SPEC does not match canonical SPEC")
	}
	return nil
}

// @AX:WARN [AUTO]: canonical SPEC resolution has nine fail-closed if branches.
// @AX:REASON [AUTO]: lexical and canonical containment, symlink refusal, SPEC identity, and spec.md identity must be checked together.
func canonicalWorkflowContextProductSpecDir(input WorkflowContextProductSessionInput) (string, error) {
	raw := strings.TrimSpace(input.SpecDir)
	if raw == "" || input.SpecID == "" {
		return "", fmt.Errorf("workflow context product session: canonical SPEC directory is required")
	}
	candidate := filepath.FromSlash(raw)
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(input.ProjectDir, candidate)
	}
	absolute, err := filepath.Abs(candidate)
	if err != nil {
		return "", fmt.Errorf("workflow context product session: resolve SPEC directory: %w", err)
	}
	lexical := filepath.Clean(absolute)
	relative, err := filepath.Rel(input.ProjectDir, lexical)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("workflow context product session: SPEC directory must be inside the project")
	}
	canonical, err := filepath.EvalSymlinks(lexical)
	if err != nil {
		return "", fmt.Errorf("workflow context product session: canonical SPEC directory is unavailable")
	}
	canonicalProject, err := filepath.EvalSymlinks(input.ProjectDir)
	if err != nil {
		return "", fmt.Errorf("workflow context product session: canonical project directory is unavailable")
	}
	canonicalRelative, err := filepath.Rel(canonicalProject, canonical)
	if err != nil || canonicalRelative == "." || canonicalRelative == ".." || strings.HasPrefix(canonicalRelative, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("workflow context product session: SPEC directory must be inside the project")
	}
	info, err := os.Lstat(lexical)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || filepath.Base(canonical) != input.SpecID {
		return "", fmt.Errorf("workflow context product session: SPEC directory identity is invalid")
	}
	specInfo, err := os.Lstat(filepath.Join(lexical, "spec.md"))
	if err != nil || !specInfo.Mode().IsRegular() || specInfo.Mode()&os.ModeSymlink != 0 {
		return "", fmt.Errorf("workflow context product session: canonical SPEC spec.md is required")
	}
	return filepath.ToSlash(filepath.Clean(relative)), nil
}

func rejectWorkflowContextProductDriver(driver WorkflowContextManagedProcessDriver, cause error) error {
	maintenance, cancel := workflowContextMaintenanceContext()
	defer cancel()
	return errors.Join(cause, driver.Cleanup(maintenance))
}

func workflowContextProductEphemeral(input WorkflowContextProductSessionInput) promptlayer.OMPContextEphemeral {
	return promptlayer.OMPContextEphemeral{
		OriginalTask: input.OriginalTask, DecisionDelta: input.DecisionDelta,
		FrozenFindingIDs: append([]string(nil), input.FrozenFindingIDs...),
		OwnershipPaths:   append([]string(nil), input.OwnershipPaths...),
		ForbiddenPaths:   append([]string(nil), input.ForbiddenPaths...),
	}
}

func cloneWorkflowContextProductHistory(rows []promptlayer.OMPContextHistoryRow) []promptlayer.OMPContextHistoryRow {
	return append([]promptlayer.OMPContextHistoryRow(nil), rows...)
}

type workflowContextProductCanonicalSource struct {
	options   promptlayer.ContextDeliveryOptions
	ephemeral promptlayer.OMPContextEphemeral
}

func (source workflowContextProductCanonicalSource) Rebuild(
	_ context.Context,
	options promptlayer.ContextDeliveryOptions,
) (promptlayer.ContextDeliveryResult, promptlayer.OMPContextEphemeral, error) {
	if !reflect.DeepEqual(source.options, options) {
		return promptlayer.ContextDeliveryResult{}, promptlayer.OMPContextEphemeral{},
			fmt.Errorf("authoritative OMP context options changed")
	}
	delivery, err := promptlayer.BuildContextDelivery(source.options)
	if err != nil {
		return promptlayer.ContextDeliveryResult{}, promptlayer.OMPContextEphemeral{}, err
	}
	if err := promptlayer.VerifyContextDeliveryForOptions(source.options, delivery); err != nil {
		return promptlayer.ContextDeliveryResult{}, promptlayer.OMPContextEphemeral{}, err
	}
	ephemeral := source.ephemeral
	ephemeral.FrozenFindingIDs = append([]string(nil), source.ephemeral.FrozenFindingIDs...)
	ephemeral.OwnershipPaths = append([]string(nil), source.ephemeral.OwnershipPaths...)
	ephemeral.ForbiddenPaths = append([]string(nil), source.ephemeral.ForbiddenPaths...)
	return delivery, ephemeral, nil
}
