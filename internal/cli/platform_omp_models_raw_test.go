package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/adapter/omp"
)

func TestPlatformOMPModelsNativeCatalogDegradesToSafeDisplayOnlyRows(t *testing.T) {
	runner := &ompCLIFakeRunner{catalog: ompCLINativeCatalogJSON()}
	root := t.TempDir()
	deps := normalizeOMPPlatformDependencies(ompPlatformDependencies{
		newRunner: func() omp.OMPModelCatalogRunner { return runner },
	})

	text := executeOMPSubcommand(t, newPlatformOMPModelsCmd(&root, deps))
	assert.Contains(t, text, "Status: degraded (catalog_metadata_insufficient)")
	assert.Contains(t, text, "openai-codex/gpt-5.4-mini")
	assert.Contains(t, text, "semantic_metadata=unavailable")
	assert.Contains(t, text, "Routing/profile apply: blocked")
	assert.NotContains(t, text, "$2.50")

	encoded := executeOMPSubcommand(t, newPlatformOMPModelsCmd(&root, deps), "--json")
	var envelope ompCLIJSONEnvelope
	require.NoError(t, json.Unmarshal([]byte(encoded), &envelope))
	assert.Equal(t, jsonStatusWarn, envelope.Status)
	var payload ompCatalogPayload
	require.NoError(t, json.Unmarshal(envelope.Data, &payload))
	assert.Equal(t, "degraded", payload.Status)
	assert.Equal(t, ompRawCatalogFallbackReason, payload.Reason)
	assert.False(t, payload.StrictRoutingReady)
	require.Len(t, payload.Models, 2)
	assert.Equal(t, "anthropic/claude-haiku-4-5", payload.Models[0].Selector)
	assert.Equal(t, "openai-codex/gpt-5.4-mini", payload.Models[1].Selector)
	assert.Equal(t, int64(272000), payload.Models[1].ContextWindow)
	assert.Equal(t, []string{"image", "text"}, payload.Models[1].Input)
	assert.Empty(t, payload.Models[1].Family)
	assert.Empty(t, payload.Models[1].Capabilities)
	assert.Empty(t, payload.Models[1].Thinking)
	assert.Equal(t, "unknown", payload.Models[1].AuthAvailability)
	assert.NotContains(t, strings.ToLower(encoded), "cache_read")
}

func TestNormalizeOMPRawDisplayCatalogRejectsUntrustedShapes(t *testing.T) {
	valid := `{"models":[{"provider":"p","id":"m","selector":"p/m","name":"Model M","contextWindow":200000,"reasoning":true,"thinking":["low","high"],"input":["text"]}]}`
	tests := map[string]string{
		"duplicate key":       `{"models":[],"models":[]}`,
		"trailing":            valid + ` {}`,
		"credential field":    `{"models":[{"provider":"p","id":"m","selector":"p/m","name":"Model M","contextWindow":200000,"reasoning":true,"thinking":["high"],"input":["text"],"access_token":"sentinel"}]}`,
		"duplicate selector":  `{"models":[{"provider":"p","id":"m","selector":"p/m","name":"One","contextWindow":1,"reasoning":true,"thinking":["high"],"input":["text"]},{"provider":"p","id":"m","selector":"p/m","name":"Two","contextWindow":1,"reasoning":true,"thinking":["high"],"input":["text"]}]}`,
		"unsafe display name": `{"models":[{"provider":"p","id":"m","selector":"p/m","name":"bad\u001bname","contextWindow":1,"reasoning":true,"thinking":["high"],"input":["text"]}]}`,
		"fractional context":  `{"models":[{"provider":"p","id":"m","selector":"p/m","name":"Model M","contextWindow":1.5,"reasoning":true,"thinking":["high"],"input":["text"]}]}`,
		"unknown input":       `{"models":[{"provider":"p","id":"m","selector":"p/m","name":"Model M","contextWindow":1,"reasoning":true,"thinking":["high"],"input":["audio"]}]}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			models, reason := normalizeOMPRawDisplayCatalog([]byte(body), 1<<20)
			assert.Equal(t, "catalog_invalid", reason)
			assert.Empty(t, models)
		})
	}
	models, reason := normalizeOMPRawDisplayCatalog([]byte(valid), 1<<20)
	assert.Equal(t, "catalog_ready", reason)
	require.Len(t, models, 1)
	assert.Equal(t, "p/m", models[0].Selector)
}

func TestPlatformOMPModelsInvalidCatalogDoesNotUseNativeFallback(t *testing.T) {
	runner := &ompCLIFakeRunner{catalog: []byte(`{"models":[]} trailing`)}
	root := t.TempDir()
	deps := normalizeOMPPlatformDependencies(ompPlatformDependencies{
		newRunner: func() omp.OMPModelCatalogRunner { return runner },
	})
	cmd := newPlatformOMPModelsCmd(&root, deps)
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--json"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err := cmd.Execute()
	require.Error(t, err)
	assert.Equal(t, 1, countOMPModelProbeCalls(runner.calls))
	var envelope struct {
		Status jsonEnvelopeStatus `json:"status"`
		Data   ompCatalogPayload  `json:"data"`
	}
	require.NoError(t, json.Unmarshal(output.Bytes(), &envelope))
	assert.Equal(t, jsonStatusError, envelope.Status)
	assert.Equal(t, "catalog_invalid", envelope.Data.Reason)
	assert.Empty(t, envelope.Data.Models)
}

func TestPlatformOMPProfileInitKeepsNativeDisplayCatalogFailClosed(t *testing.T) {
	runner := &ompCLIFakeRunner{catalog: ompCLINativeCatalogJSON()}
	root := t.TempDir()
	deps := normalizeOMPPlatformDependencies(ompPlatformDependencies{
		newRunner: func() omp.OMPModelCatalogRunner { return runner },
	})
	cmd := newPlatformOMPProfileInitCmd(&root, deps)
	cmd.SetArgs([]string{"--name", "balanced"})
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	err := cmd.Execute()
	require.Error(t, err)
	assert.Contains(t, err.Error(), ompRawCatalogFallbackReason)
	assert.Equal(t, 1, countOMPModelProbeCalls(runner.calls))
}

func countOMPModelProbeCalls(calls []string) int {
	count := 0
	for _, call := range calls {
		if strings.HasSuffix(call, " models --json --no-extensions") {
			count++
		}
	}
	return count
}

func ompCLINativeCatalogJSON() []byte {
	return []byte(`{"models":[
		{"provider":"openai-codex","id":"gpt-5.4-mini","selector":"openai-codex/gpt-5.4-mini","name":"GPT-5.4 mini","contextWindow":272000,"reasoning":true,"thinking":["minimal","low","medium","high","xhigh"],"input":["text","image"],"cost":{"input":2.5,"output":15,"cacheRead":0.25,"cacheWrite":0},"ownedBy":"openai-codex"},
		{"provider":"anthropic","id":"claude-haiku-4-5","selector":"anthropic/claude-haiku-4-5","name":"Claude Haiku 4.5","contextWindow":200000,"reasoning":true,"thinking":null,"input":["text","image"],"cost":{"input":1,"output":5,"cacheRead":0.1,"cacheWrite":1.25},"ownedBy":"anthropic"}
	]}`)
}
