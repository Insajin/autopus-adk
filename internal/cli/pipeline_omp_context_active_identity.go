package cli

import (
	"strings"

	ompadapter "github.com/insajin/autopus-adk/pkg/adapter/omp"
)

func pipelineOMPActiveImplementationDigest() string {
	bridge := ompadapter.ExpectedOMPContextBridgeSourceIdentity()
	route := ompadapter.ExpectedOMPNativePipelineRouteSourceIdentity()
	return pipelineOMPActiveHash([]byte(strings.Join([]string{
		pipelineOMPActiveRPCIdentity, pipelineOMPActivePolicyIdentity, bridge.TargetPath, bridge.SHA256,
		route.TargetPath, route.SHA256,
	}, "\x00")))
}
