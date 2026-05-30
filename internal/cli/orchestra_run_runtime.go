package cli

import "github.com/insajin/autopus-adk/pkg/orchestra"

var (
	orchestraRunLoadConfig     = loadOrchestraConfig
	orchestraRunBuildProviders = buildProviderConfigs
	// orchestraRunBackendFactory routes the run pipeline backend through
	// SelectBackend (REQ-003) so it consumes the detected terminal rather than a
	// hardcoded subprocess backend. Kept as a var to preserve the test seam.
	orchestraRunBackendFactory  func(orchestra.OrchestraConfig) orchestra.ExecutionBackend = orchestra.SelectBackend
	orchestraRunExecutePipeline                                                            = orchestra.RunSubprocessPipeline
)
