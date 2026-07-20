package run

import (
	"context"
	"errors"
	"time"

	"github.com/insajin/autopus-adk/pkg/qa/desktopobserve"
)

const maxDesktopObservationDuration = 2 * time.Second

type desktopProviderClient interface {
	Handshake(context.Context) (desktopobserve.ProviderIdentity, error)
	Capabilities(context.Context) ([]desktopobserve.Operation, error)
	Permissions(context.Context) (desktopobserve.PermissionResult, error)
	ListApps(context.Context) ([]desktopobserve.AppSummary, error)
	ListWindows(context.Context, string) ([]desktopobserve.WindowSummary, error)
	GetState(context.Context, string, string) (desktopobserve.SemanticProjection, error)
}

type desktopProviderResolver interface {
	ResolveLocal(context.Context) (desktopProviderClient, error)
	LookPath(context.Context, string) (string, error)
	ResolveOrca(context.Context, string) (string, error)
	NewOrcaClient(context.Context, string) (desktopProviderClient, error)
}

type DesktopObservationRunRequest struct {
	RuntimeProvider desktopobserve.RuntimeProvider
	Operations      []desktopobserve.Operation
	AppRef          string
	WindowRef       string
	Policy          desktopobserve.OraclePolicy
	Redactor        desktopobserve.Redactor
}

type desktopObservationRunner struct {
	local    desktopProviderClient
	orca     desktopProviderClient
	resolver desktopProviderResolver
	ledgers  map[desktopobserve.RuntimeProvider]*desktopobserve.StateLedger
	timeout  time.Duration
}

func newDesktopObservationRunner(local, orca desktopProviderClient) *desktopObservationRunner {
	return &desktopObservationRunner{
		local:   local,
		orca:    orca,
		timeout: maxDesktopObservationDuration,
		ledgers: map[desktopobserve.RuntimeProvider]*desktopobserve.StateLedger{
			desktopobserve.RuntimeProviderLocal: desktopobserve.NewStateLedger(),
			desktopobserve.RuntimeProviderOrca:  desktopobserve.NewStateLedger(),
		},
	}
}

func (runner *desktopObservationRunner) operationContext(parent context.Context) (context.Context, context.CancelFunc) {
	timeout := maxDesktopObservationDuration
	if runner != nil && runner.timeout > 0 && runner.timeout < timeout {
		timeout = runner.timeout
	}
	return context.WithTimeout(parent, timeout)
}

func desktopBoundedError(ctx context.Context, err error) error {
	if contextErr := ctx.Err(); contextErr != nil {
		return contextErr
	}
	return err
}

func newDesktopObservationRunnerWithResolver(resolver desktopProviderResolver) *desktopObservationRunner {
	runner := newDesktopObservationRunner(nil, nil)
	runner.resolver = resolver
	return runner
}

func (runner *desktopObservationRunner) resolveClient(
	ctx context.Context,
	provider desktopobserve.RuntimeProvider,
) (desktopProviderClient, error) {
	if runner == nil {
		return nil, errors.New("desktop observation runner is unavailable")
	}
	if runner.resolver == nil {
		if provider == desktopobserve.RuntimeProviderLocal {
			return runner.local, nil
		}
		return runner.orca, nil
	}
	if provider == desktopobserve.RuntimeProviderLocal {
		return runner.resolver.ResolveLocal(ctx)
	}
	path, err := runner.resolver.LookPath(ctx, "orca")
	if err != nil {
		return nil, err
	}
	resolved, err := runner.resolver.ResolveOrca(ctx, path)
	if err != nil {
		return nil, err
	}
	return runner.resolver.NewOrcaClient(ctx, resolved)
}

func desktopExecutionOperations() []desktopobserve.Operation {
	return []desktopobserve.Operation{
		desktopobserve.OperationCapabilities,
		desktopobserve.OperationPermissions,
		desktopobserve.OperationListApps,
		desktopobserve.OperationListWindows,
		desktopobserve.OperationGetState,
	}
}

func validDesktopProvider(provider desktopobserve.RuntimeProvider) error {
	switch provider {
	case "":
		return desktopobserve.ErrRuntimeProviderRequired
	case desktopobserve.RuntimeProviderLocal, desktopobserve.RuntimeProviderOrca:
		return nil
	default:
		return desktopobserve.ErrRuntimeProviderInvalid
	}
}

func validDesktopOperations(operations []desktopobserve.Operation) bool {
	expected := desktopExecutionOperations()
	if len(operations) != len(expected) {
		return false
	}
	for index := range expected {
		if operations[index] != expected[index] || !desktopobserve.IsReadOnlyOperation(operations[index]) {
			return false
		}
	}
	return true
}

func expectedDesktopIdentity(provider desktopobserve.RuntimeProvider) desktopobserve.ProviderIdentity {
	name := "autopus-desktop-local"
	if provider == desktopobserve.RuntimeProviderOrca {
		name = "orca-computer-use-macos"
	}
	return desktopobserve.ProviderIdentity{
		Name: name, Version: "0.0.0", ProtocolVersion: desktopobserve.ProtocolVersion,
	}
}

func validDesktopIdentity(identity desktopobserve.ProviderIdentity, provider desktopobserve.RuntimeProvider) bool {
	if identity.Name != expectedDesktopIdentity(provider).Name ||
		identity.ProtocolVersion != desktopobserve.ProtocolVersion {
		return false
	}
	receipt := desktopSuccessReceipt(identity, desktopProviderScope(identity), desktopSupportedCapabilities())
	receipt.Redaction.Status = desktopobserve.RedactionNotRequired
	receipt.Quarantine.Status = desktopobserve.QuarantineEmpty
	return receipt.Validate() == nil
}

func desktopProviderScope(identity desktopobserve.ProviderIdentity) desktopobserve.ReceiptScope {
	return desktopobserve.ReceiptScope{Kind: desktopobserve.ScopeProvider, PublicRef: identity.Name}
}

func desktopWindowScope(windowRef string) desktopobserve.ReceiptScope {
	return desktopobserve.ReceiptScope{Kind: desktopobserve.ScopeWindow, PublicRef: windowRef}
}

func desktopProviderPublicRef(provider desktopobserve.RuntimeProvider) string {
	if provider == desktopobserve.RuntimeProviderOrca {
		return "provider-orca"
	}
	return "provider-local"
}
