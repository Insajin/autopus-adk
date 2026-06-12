package design

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// figmaServer returns an httptest server responding with the given status and body.
func figmaServer(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestFetchFigmaNodes_NonSuccessStatusReadsError(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "DESIGN.md", "Figma: https://figma.com/design/abc123/Product?node-id=1-2")
	server := figmaServer(t, http.StatusForbidden, "Invalid token")
	defer server.Close()

	report, err := FetchFigmaNodes(context.Background(), root, FigmaFetchOptions{
		Token: "bad", APIBaseURL: server.URL, HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("FetchFigmaNodes: %v", err)
	}
	if len(report.Nodes) != 1 {
		t.Fatalf("nodes = %d, want 1", len(report.Nodes))
	}
	n := report.Nodes[0]
	if n.Status != "error" || n.StatusCode != http.StatusForbidden {
		t.Errorf("status=%q code=%d, want error/403", n.Status, n.StatusCode)
	}
	if n.Error != "Invalid token" {
		t.Errorf("error body = %q, want 'Invalid token'", n.Error)
	}
}

func TestFetchFigmaNodes_NodeMissingInPayload(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "DESIGN.md", "Figma: https://figma.com/design/abc123/Product?node-id=1-2")
	// Payload omits the requested node id "1:2".
	server := figmaServer(t, http.StatusOK, `{"name":"Product","nodes":{}}`)
	defer server.Close()

	report, err := FetchFigmaNodes(context.Background(), root, FigmaFetchOptions{
		Token: "t", APIBaseURL: server.URL, HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("FetchFigmaNodes: %v", err)
	}
	n := report.Nodes[0]
	if n.Status != "missing" || n.Error != "node_not_found" {
		t.Errorf("status=%q error=%q, want missing/node_not_found", n.Status, n.Error)
	}
}

func TestFetchFigmaNodes_DecodeError(t *testing.T) {
	root := t.TempDir()
	writeFile(t, root, "DESIGN.md", "Figma: https://figma.com/design/abc123/Product?node-id=1-2")
	server := figmaServer(t, http.StatusOK, "{not-json")
	defer server.Close()

	report, err := FetchFigmaNodes(context.Background(), root, FigmaFetchOptions{
		Token: "t", APIBaseURL: server.URL, HTTPClient: server.Client(),
	})
	if err != nil {
		t.Fatalf("FetchFigmaNodes: %v", err)
	}
	if report.Nodes[0].Status != "error" || report.Nodes[0].Error == "" {
		t.Errorf("expected decode error status, got %+v", report.Nodes[0])
	}
}

func TestReadFigmaError_EmptyBodyFallback(t *testing.T) {
	if got := readFigmaError(strings.NewReader("")); got != "figma_api_error" {
		t.Errorf("empty body = %q, want figma_api_error", got)
	}
	if got := readFigmaError(strings.NewReader("  boom  ")); got != "boom" {
		t.Errorf("trimmed body = %q, want boom", got)
	}
}

func TestFigmaFetchReport_MarkdownEmptyAndPopulated(t *testing.T) {
	empty := FigmaFetchReport{}
	md := empty.Markdown()
	if !strings.Contains(md, "## Figma Node Fetch") || !strings.Contains(md, "Nodes: none") {
		t.Errorf("empty markdown missing sections: %q", md)
	}

	report := FigmaFetchReport{
		Nodes: []FigmaNodeFetch{
			{FileKey: "abc", NodeID: "1:2", Status: "fetched", NodeName: "Hero", NodeType: "FRAME"},
			{FileKey: "abc", NodeID: "3:4", Status: "error", Error: "boom"},
		},
		SetupGaps: []string{"figma_token_missing"},
	}
	md = report.Markdown()
	if !strings.Contains(md, "abc 1:2: fetched (Hero, FRAME)") {
		t.Errorf("markdown missing fetched node line: %q", md)
	}
	if !strings.Contains(md, "abc 3:4: error [boom]") {
		t.Errorf("markdown missing error node line: %q", md)
	}
	if !strings.Contains(md, "Setup gaps:") || !strings.Contains(md, "figma_token_missing") {
		t.Errorf("markdown missing setup gaps: %q", md)
	}
}

func TestFigmaFetchReport_JSON(t *testing.T) {
	report := FigmaFetchReport{
		Version: 1,
		Nodes:   []FigmaNodeFetch{{FileKey: "abc", NodeID: "1:2", Status: "fetched"}},
	}
	data, err := report.JSON()
	if err != nil {
		t.Fatalf("JSON: %v", err)
	}
	if len(data) == 0 || data[len(data)-1] != '\n' {
		t.Error("JSON output must end with newline")
	}
	var round FigmaFetchReport
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("round-trip unmarshal: %v", err)
	}
	if round.Version != 1 || len(round.Nodes) != 1 || round.Nodes[0].FileKey != "abc" {
		t.Errorf("round-trip mismatch: %+v", round)
	}
}
