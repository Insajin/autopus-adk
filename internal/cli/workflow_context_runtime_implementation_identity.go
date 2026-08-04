package cli

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/insajin/autopus-adk/pkg/adapter/omp"
	"github.com/spf13/cobra"
)

const workflowContextImplementationIdentitySchemaV1 = "workflow-context-runtime-implementation/v1"

type workflowContextImplementationIdentityV1 struct {
	SchemaVersion                string `json:"schema_version"`
	PipelineImplementationDigest string `json:"pipeline_implementation_digest"`
	RPCIdentity                  string `json:"rpc_identity"`
	PolicyIdentity               string `json:"policy_identity"`
	BridgeTarget                 string `json:"bridge_target"`
	BridgeSHA256                 string `json:"bridge_sha256"`
	RouteTarget                  string `json:"route_target"`
	RouteSHA256                  string `json:"route_sha256"`
}

func newWorkflowContextImplementationIdentityCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:           "implementation-identity",
		Short:         "Print the body-free managed OMP implementation identity",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if strings.ToLower(strings.TrimSpace(format)) != "json" {
				return errors.New("workflow context-runtime implementation-identity requires --format json")
			}
			bridge := omp.ExpectedOMPContextBridgeSourceIdentity()
			route := omp.ExpectedOMPNativePipelineRouteSourceIdentity()
			return json.NewEncoder(cmd.OutOrStdout()).Encode(workflowContextImplementationIdentityV1{
				SchemaVersion:                workflowContextImplementationIdentitySchemaV1,
				PipelineImplementationDigest: pipelineOMPActiveImplementationDigest(),
				RPCIdentity:                  pipelineOMPActiveRPCIdentity, PolicyIdentity: pipelineOMPActivePolicyIdentity,
				BridgeTarget: bridge.TargetPath, BridgeSHA256: bridge.SHA256,
				RouteTarget: route.TargetPath, RouteSHA256: route.SHA256,
			})
		},
	}
	cmd.Flags().StringVar(&format, "format", "json", "Output format (json)")
	return cmd
}
