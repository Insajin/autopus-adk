package orchestra

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResponseFailuresFromInteractivePanes_TimedOutResponseBecomesFailure(t *testing.T) {
	t.Parallel()

	panes := []paneInfo{{
		provider: ProviderConfig{Name: "gemini", Binary: "agy"},
		paneID:   "pane-1",
	}}
	responses := []ProviderResponse{{
		Provider: "gemini",
		Output:   "shell quote>",
		TimedOut: true,
		Duration: time.Second,
	}}

	failures := responseFailuresFromInteractivePanes(panes, responses, 45, nil)
	require.Len(t, failures, 1)
	assert.Equal(t, "gemini", failures[0].Name)
	assert.Equal(t, "timeout", failures[0].FailureClass)
	assert.Contains(t, failures[0].Error, "timeout")
}
