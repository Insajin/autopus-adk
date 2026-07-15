package worker

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
)

func providerResultError(provider string, result adapter.TaskResult) error {
	message := strings.TrimSpace(result.Error)
	if message == "" {
		message = strings.TrimSpace(result.Output)
	}
	if message == "" {
		message = "provider result marked as error"
	}
	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = "provider"
	}
	return fmt.Errorf("%s result error: %s", provider, message)
}
