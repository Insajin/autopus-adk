package omp

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/processprobe"
	"github.com/stretchr/testify/require"
)

type installedOMPModelCatalogRunner struct {
	environment []string
	maxOutput   int
	catalog     []byte
}

func (runner *installedOMPModelCatalogRunner) Run(
	ctx context.Context,
	executable string,
	args ...string,
) ([]byte, error) {
	path, err := exec.LookPath(executable)
	if err != nil {
		return nil, err
	}
	command := exec.CommandContext(ctx, path, args...)
	command.Env = runner.environment
	output, err := processprobe.OutputLimited(command, runner.maxOutput)
	if strings.Join(args, " ") == "models --json" {
		runner.catalog = append([]byte(nil), output...)
	}
	return output, err
}

func TestProbeOMPModelCatalog_LiveInstalledSchemaFailsClosedWhenSemanticMetadataIsAbsent(t *testing.T) {
	if os.Getenv("AUTOPUS_OMP_CATALOG_LIVE") != "1" {
		t.Skip("set AUTOPUS_OMP_CATALOG_LIVE=1 to run the installed catalog-only probe")
	}
	if _, err := exec.LookPath("omp"); err != nil {
		t.Skip("installed OMP binary is unavailable")
	}

	var requests atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		requests.Add(1)
	}))
	defer server.Close()
	root := t.TempDir()
	profile := filepath.Join(root, "pi-agent")
	require.NoError(t, os.Mkdir(profile, 0o700))
	require.NoError(t, os.WriteFile(filepath.Join(profile, "models.yml"), []byte(fmt.Sprintf(`providers:
  catalogprobe:
    baseUrl: %s/v1
    auth: none
    api: openai-completions
    models:
      - id: catalog-only
        name: Catalog Only
        reasoning: true
        input: [text]
        contextWindow: 4096
        maxTokens: 128
`, server.URL)), 0o600))

	runner := &installedOMPModelCatalogRunner{
		maxOutput:   128 * 1024,
		environment: isolatedOMPModelCatalogEnvironment(t, root, profile),
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	result := ProbeOMPModelCatalog(ctx, OMPModelCatalogProbeOptions{
		Executable: "omp", Runner: runner, Timeout: 10 * time.Second, MaxOutput: runner.maxOutput,
	})

	row, found := findInstalledOMPModelRow(runner.catalog, "catalogprobe", "catalog-only")
	require.True(t, found)
	topSchema := installedOMPJSONSchema(runner.catalog)
	rowSchema := installedOMPRowSchema(row)
	hasRequiredMetadata := hasInstalledOMPModelField(row, "family") &&
		hasInstalledOMPModelField(row, "capabilities") && hasInstalledOMPModelField(row, "thinking") &&
		(hasInstalledOMPModelField(row, "auth_enabled") || hasInstalledOMPModelField(row, "available") ||
			hasInstalledOMPModelField(row, "keyless"))
	if hasRequiredMetadata {
		require.Equal(t, "ready", result.Status)
		require.Equal(t, "catalog_ready", result.Reason)
	} else {
		require.Equal(t, "blocked", result.Status)
		require.Equal(t, "catalog_metadata_insufficient", result.Reason)
		require.Empty(t, result.Catalog.Models)
	}
	enriched, enrichedReason := NormalizeOMPAvailableCatalog(runner.catalog, runner.maxOutput, []OMPModelCatalogDeclaration{
		{Selector: "catalogprobe/catalog-only", Family: "catalogprobe-family", Capability: "deep_reasoning"},
	})
	require.Equal(t, "catalog_ready", enrichedReason)
	require.Len(t, enriched.Models, 1)
	require.Equal(t, []string{"deep_reasoning"}, enriched.Models[0].Capabilities)
	require.True(t, enriched.Models[0].AuthEnabled)
	require.Equal(t, "omp/17.1.8", result.Version)
	require.Zero(t, requests.Load(), "catalog discovery must not issue a model/provider request")
	t.Logf("catalog-only schema: top_level=%s custom_row=%s required_metadata=%t missing=%s provider_requests=%d",
		strings.Join(topSchema, ","), strings.Join(rowSchema, ","), hasRequiredMetadata,
		strings.Join(missingInstalledOMPSemanticFields(row), ","), requests.Load())
}

func isolatedOMPModelCatalogEnvironment(t *testing.T, root, profile string) []string {
	t.Helper()
	overrides := map[string]string{
		"HOME": filepath.Join(root, "home"), "PI_CODING_AGENT_DIR": profile,
		"XDG_CACHE_HOME": filepath.Join(root, "cache"), "XDG_CONFIG_HOME": filepath.Join(root, "config"),
		"XDG_DATA_HOME": filepath.Join(root, "data"), "XDG_STATE_HOME": filepath.Join(root, "state"),
	}
	for _, directory := range overrides {
		require.NoError(t, os.MkdirAll(directory, 0o700))
	}
	environment := make([]string, 0, len(os.Environ())+len(overrides))
	for _, entry := range os.Environ() {
		key := strings.SplitN(entry, "=", 2)[0]
		if _, replaced := overrides[key]; !replaced {
			environment = append(environment, entry)
		}
	}
	for key, value := range overrides {
		environment = append(environment, key+"="+value)
	}
	return environment
}

func findInstalledOMPModelRow(data []byte, provider, model string) (map[string]json.RawMessage, bool) {
	var payload struct {
		Models []map[string]json.RawMessage `json:"models"`
	}
	if json.Unmarshal(data, &payload) != nil {
		return nil, false
	}
	for _, row := range payload.Models {
		var observedProvider, observedModel string
		if json.Unmarshal(row["provider"], &observedProvider) == nil &&
			json.Unmarshal(row["id"], &observedModel) == nil &&
			observedProvider == provider && observedModel == model {
			return row, true
		}
	}
	return nil, false
}

func hasInstalledOMPModelField(row map[string]json.RawMessage, field string) bool {
	value, ok := row[field]
	return ok && len(value) > 0 && string(value) != "null"
}

func installedOMPJSONSchema(data []byte) []string {
	var object map[string]json.RawMessage
	if json.Unmarshal(data, &object) != nil {
		return nil
	}
	return installedOMPRowSchema(object)
}

func installedOMPRowSchema(row map[string]json.RawMessage) []string {
	result := make([]string, 0, len(row))
	for key, value := range row {
		result = append(result, key+":"+installedOMPJSONType(value))
	}
	sort.Strings(result)
	return result
}

func installedOMPJSONType(value json.RawMessage) string {
	var decoded any
	if json.Unmarshal(value, &decoded) != nil {
		return "invalid"
	}
	switch decoded.(type) {
	case nil:
		return "null"
	case bool:
		return "boolean"
	case float64:
		return "number"
	case string:
		return "string"
	case []any:
		return "array"
	case map[string]any:
		return "object"
	default:
		return "unknown"
	}
}

func missingInstalledOMPSemanticFields(row map[string]json.RawMessage) []string {
	missing := make([]string, 0, 6)
	for _, field := range []string{"family", "capabilities", "thinking", "auth_enabled", "available", "keyless"} {
		if !hasInstalledOMPModelField(row, field) {
			missing = append(missing, field)
		}
	}
	return missing
}
