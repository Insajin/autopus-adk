package cli

import (
	"strings"

	"github.com/insajin/autopus-adk/pkg/orchestra"
)

func shouldInlineStructuredReviewSchema(backend orchestra.ExecutionBackend, provider orchestra.ProviderConfig) bool {
	if backend != nil && backend.Name() == "pane" {
		return true
	}
	return strings.TrimSpace(provider.SchemaFlag) == ""
}
