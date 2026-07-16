package cli

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCollectJSONVisualEvidenceAdoptsOnlyFinalRetryArtifacts(t *testing.T) {
	t.Parallel()

	input := []byte(`{
		"suites":[{"specs":[{"id":"spec-home","tests":[{
			"id":"test-home","projectName":"chromium","results":[
				{"id":"result-first","retry":0,"status":"failed","attachments":[
					{"name":"actual","contentType":"image/png","path":"first-actual.png"}
				]},
				{"id":"result-final","retry":1,"status":"failed","attachments":[
					{"name":"actual","contentType":"image/png","path":"final-actual.png"}
				]}
			]
		}]}]}]
	}`)

	evidence := collectVisualEvidence(input)
	require.Len(t, evidence.Artifacts, 1)
	assert.Equal(t, "final-actual.png", evidence.Artifacts[0].Path)
	encoded, err := json.Marshal(evidence.Artifacts[0])
	require.NoError(t, err)
	assert.Contains(t, string(encoded), `"result_id":"result-final"`)
	assert.Contains(t, string(encoded), `"retry":1`)
	assert.Equal(t, []string{"chromium"}, evidence.Projects)
}
