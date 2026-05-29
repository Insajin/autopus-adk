package design

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExtractFigmaRefsSanitizesQueryAndHashes(t *testing.T) {
	t.Parallel()

	refs := ExtractFigmaRefs("DESIGN.md", "See https://www.figma.com/design/abc123/Product?node-id=1-2#frag")
	require.Len(t, refs, 1)
	assert.Equal(t, "DESIGN.md", refs[0].SourcePath)
	assert.Equal(t, "design", refs[0].Kind)
	assert.Equal(t, "https://www.figma.com/design/abc123/Product", refs[0].URL)
	assert.Equal(t, "abc123", refs[0].FileKey)
	assert.Equal(t, "1:2", refs[0].NodeID)
	assert.Len(t, refs[0].URLHash, 16)
}

func TestAuditFigmaDetectsRefsAndCodeConnect(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "DESIGN.md", "Figma: https://figma.com/file/abc123/Library")
	writeFile(t, root, "src/components/Button.figma.tsx", "export default {}")
	writeFile(t, root, "package.json", `{"devDependencies":{"@figma/code-connect":"latest"}}`)

	audit, err := AuditFigma(root, 10)
	require.NoError(t, err)
	require.Len(t, audit.FigmaRefs, 1)
	assert.Equal(t, "detected", audit.CodeConnect.Status)
	assert.Equal(t, filepath.ToSlash("src/components/Button.figma.tsx"), audit.CodeConnect.MappingRefs[0].Path)
	assert.Contains(t, audit.Markdown(), "Code Connect: detected")
}

func TestAuditFigmaReportsSetupGapsWithoutNetwork(t *testing.T) {
	t.Parallel()

	audit, err := AuditFigma(t.TempDir(), 10)
	require.NoError(t, err)
	assert.Empty(t, audit.FigmaRefs)
	assert.Equal(t, "missing", audit.CodeConnect.Status)
	assert.Contains(t, audit.SetupGaps, "figma_reference_missing")
	assert.Contains(t, audit.SetupGaps, "code_connect_mapping_missing")
}

func TestFetchFigmaNodesGetsCompactNodeMetadata(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "DESIGN.md", "Figma: https://figma.com/design/abc123/Product?node-id=1-2")
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/v1/files/abc123/nodes", r.URL.Path)
		assert.Equal(t, "1:2", r.URL.Query().Get("ids"))
		assert.Equal(t, "1", r.URL.Query().Get("depth"))
		assert.Equal(t, "test-token", r.Header.Get("X-Figma-Token"))
		fmt.Fprint(w, `{"name":"Product","lastModified":"2026-01-02T03:04:05Z","nodes":{"1:2":{"document":{"name":"Hero","type":"FRAME"},"components":{"1:3":{}},"componentSets":{},"styles":{"S:1":{}}}}}`)
	}))
	defer server.Close()

	report, err := FetchFigmaNodes(context.Background(), root, FigmaFetchOptions{
		Token:      "test-token",
		APIBaseURL: server.URL,
		HTTPClient: server.Client(),
	})
	require.NoError(t, err)
	require.Len(t, report.Nodes, 1)
	assert.Equal(t, "fetched", report.Nodes[0].Status)
	assert.Equal(t, "Hero", report.Nodes[0].NodeName)
	assert.Equal(t, "FRAME", report.Nodes[0].NodeType)
	assert.Equal(t, 1, report.Nodes[0].ComponentCount)
	assert.Empty(t, report.Nodes[0].Error)
}

func TestFetchFigmaNodesReportsMissingTokenWithoutNetwork(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeFile(t, root, "DESIGN.md", "Figma: https://figma.com/design/abc123/Product?node-id=1-2")
	report, err := FetchFigmaNodes(context.Background(), root, FigmaFetchOptions{})
	require.NoError(t, err)
	assert.Empty(t, report.Nodes)
	assert.Contains(t, report.SetupGaps, "figma_token_missing")
}
