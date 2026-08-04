package cli

import (
	"context"
	"errors"
	"fmt"
	"regexp"
)

var installedOMPVersionPattern = regexp.MustCompile(`^omp/[0-9]+\.[0-9]+\.[0-9]+$`)

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-003: installed OMP identity and loopback authorization gate the live context runtime.
// @AX:REASON [AUTO]: this is the external integration boundary between observed OMP capabilities and supervisor admission.
func RunWorkflowContextInstalledCanary(
	ctx context.Context,
	supervisor *WorkflowContextRuntimeSupervisor,
	request WorkflowContextRuntimeRequest,
	dispatch WorkflowContextDispatchFunc,
) (WorkflowContextRuntimeReceipt, error) {
	if supervisor == nil {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("OMP context canary supervisor is required")
	}
	if !installedOMPVersionPattern.MatchString(request.Capabilities.Version) {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("installed OMP identity was not observed")
	}
	if !request.Capabilities.AuthNoneLoopback {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("installed OMP auth:none loopback was not proved")
	}
	return supervisor.Run(ctx, request, dispatch)
}

// RunWorkflowContextInstalledManagedCanary admits only through a private live RPC driver.
// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-OMP-004: public managed canary boundary for installed identity and private RPC admission.
// @AX:REASON [AUTO]: live callers depend on identity verification completing before the supervisor binds and runs the managed driver.
func RunWorkflowContextInstalledManagedCanary(
	ctx context.Context,
	supervisor *WorkflowContextRuntimeSupervisor,
	request WorkflowContextRuntimeRequest,
	driver *WorkflowContextManagedRPCDriver,
) (receipt WorkflowContextRuntimeReceipt, resultErr error) {
	if driver == nil {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("OMP context managed RPC driver is required")
	}
	handedOff := false
	defer func() {
		if handedOff {
			return
		}
		maintenance, cancel := workflowContextMaintenanceContext()
		defer cancel()
		resultErr = errors.Join(resultErr, driver.Cleanup(maintenance))
	}()
	if supervisor == nil {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("OMP context managed canary supervisor is required")
	}
	if !installedOMPVersionPattern.MatchString(request.Capabilities.Version) {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("installed OMP identity was not observed")
	}
	if !request.Capabilities.AuthNoneLoopback {
		return WorkflowContextRuntimeReceipt{}, fmt.Errorf("installed OMP auth:none loopback was not proved")
	}
	if err := driver.verifyInstalledIdentity(ctx, request.Capabilities.Version); err != nil {
		return WorkflowContextRuntimeReceipt{}, err
	}
	request.Driver = driver
	handedOff = true
	return supervisor.RunManaged(ctx, request)
}
