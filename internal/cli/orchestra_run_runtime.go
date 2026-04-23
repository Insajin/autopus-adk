package cli

import "github.com/insajin/autopus-adk/pkg/orchestra"

var (
	orchestraRunLoadConfig      = loadOrchestraConfig
	orchestraRunBuildProviders  = buildProviderConfigs
	orchestraRunBackendFactory  = orchestra.NewSubprocessBackendImpl
	orchestraRunExecutePipeline = orchestra.RunSubprocessPipeline
)
