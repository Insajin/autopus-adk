package cli

import (
	"context"
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
