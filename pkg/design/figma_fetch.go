package design

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const DefaultFigmaAPIBaseURL = "https://api.figma.com"

type FigmaFetchOptions struct {
	Token      string
	APIBaseURL string
	MaxRefs    int
	Depth      int
	HTTPClient *http.Client
}

type FigmaFetchReport struct {
	Version     int              `json:"version"`
	GeneratedAt string           `json:"generated_at"`
	FigmaRefs   []FigmaRef       `json:"figma_refs,omitempty"`
	Nodes       []FigmaNodeFetch `json:"nodes,omitempty"`
	SetupGaps   []string         `json:"setup_gaps,omitempty"`
}

type FigmaNodeFetch struct {
	SourcePath        string `json:"source_path"`
	URLHash           string `json:"url_hash"`
	FileKey           string `json:"file_key"`
	NodeID            string `json:"node_id,omitempty"`
	Endpoint          string `json:"endpoint,omitempty"`
	Status            string `json:"status"`
	StatusCode        int    `json:"status_code,omitempty"`
	FileName          string `json:"file_name,omitempty"`
	LastModified      string `json:"last_modified,omitempty"`
	NodeName          string `json:"node_name,omitempty"`
	NodeType          string `json:"node_type,omitempty"`
	ComponentCount    int    `json:"component_count,omitempty"`
	ComponentSetCount int    `json:"component_set_count,omitempty"`
	StyleCount        int    `json:"style_count,omitempty"`
	Error             string `json:"error,omitempty"`
}

func FetchFigmaNodes(ctx context.Context, root string, opts FigmaFetchOptions) (FigmaFetchReport, error) {
	maxRefs := opts.MaxRefs
	if maxRefs <= 0 {
		maxRefs = 30
	}
	audit, err := AuditFigma(root, maxRefs)
	if err != nil {
		return FigmaFetchReport{}, err
	}
	report := FigmaFetchReport{
		Version:     1,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		FigmaRefs:   audit.FigmaRefs,
		SetupGaps:   append([]string{}, audit.SetupGaps...),
	}
	token := resolveFigmaToken(opts.Token)
	if token == "" {
		report.SetupGaps = appendMissing(report.SetupGaps, "figma_token_missing")
		return report, nil
	}
	client := opts.HTTPClient
	if client == nil {
		client = http.DefaultClient
	}
	baseURL := strings.TrimRight(opts.APIBaseURL, "/")
	if baseURL == "" {
		baseURL = DefaultFigmaAPIBaseURL
	}
	depth := opts.Depth
	if depth <= 0 {
		depth = 1
	}
	for _, ref := range audit.FigmaRefs {
		fetch := fetchFigmaNode(ctx, client, baseURL, token, ref, depth)
		report.Nodes = append(report.Nodes, fetch)
	}
	return report, nil
}

func fetchFigmaNode(ctx context.Context, client *http.Client, baseURL, token string, ref FigmaRef, depth int) FigmaNodeFetch {
	out := FigmaNodeFetch{
		SourcePath: ref.SourcePath,
		URLHash:    ref.URLHash,
		FileKey:    ref.FileKey,
		NodeID:     ref.NodeID,
		Status:     "skipped",
	}
	if ref.FileKey == "" {
		out.Error = "file_key_missing"
		return out
	}
	if ref.NodeID == "" {
		out.Error = "node_id_missing"
		return out
	}
	endpoint := fmt.Sprintf("/v1/files/%s/nodes?ids=%s&depth=%d", url.PathEscape(ref.FileKey), url.QueryEscape(ref.NodeID), depth)
	out.Endpoint = endpoint
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, baseURL+endpoint, nil)
	if err != nil {
		out.Status = "error"
		out.Error = err.Error()
		return out
	}
	req.Header.Set("X-Figma-Token", token)
	resp, err := client.Do(req)
	if err != nil {
		out.Status = "error"
		out.Error = err.Error()
		return out
	}
	defer resp.Body.Close()
	out.StatusCode = resp.StatusCode
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		out.Status = "error"
		out.Error = readFigmaError(resp.Body)
		return out
	}
	var payload figmaNodesPayload
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&payload); err != nil {
		out.Status = "error"
		out.Error = err.Error()
		return out
	}
	node := payload.Nodes[ref.NodeID]
	if node == nil {
		out.Status = "missing"
		out.Error = "node_not_found"
		return out
	}
	out.Status = "fetched"
	out.FileName = payload.Name
	out.LastModified = payload.LastModified
	out.NodeName = node.Document.Name
	out.NodeType = node.Document.Type
	out.ComponentCount = len(node.Components)
	out.ComponentSetCount = len(node.ComponentSets)
	out.StyleCount = len(node.Styles)
	return out
}

type figmaNodesPayload struct {
	Name         string                       `json:"name"`
	LastModified string                       `json:"lastModified"`
	Nodes        map[string]*figmaNodePayload `json:"nodes"`
}

type figmaNodePayload struct {
	Document      figmaDocumentNode      `json:"document"`
	Components    map[string]interface{} `json:"components"`
	ComponentSets map[string]interface{} `json:"componentSets"`
	Styles        map[string]interface{} `json:"styles"`
}

type figmaDocumentNode struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

func readFigmaError(body io.Reader) string {
	data, err := io.ReadAll(io.LimitReader(body, 4096))
	if err != nil || len(data) == 0 {
		return "figma_api_error"
	}
	return strings.TrimSpace(string(data))
}

func resolveFigmaToken(explicit string) string {
	if strings.TrimSpace(explicit) != "" {
		return strings.TrimSpace(explicit)
	}
	for _, key := range []string{"FIGMA_ACCESS_TOKEN", "FIGMA_TOKEN"} {
		if value := strings.TrimSpace(os.Getenv(key)); value != "" {
			return value
		}
	}
	return ""
}

func (r FigmaFetchReport) Markdown() string {
	var sb strings.Builder
	sb.WriteString("## Figma Node Fetch\n")
	if len(r.Nodes) == 0 {
		sb.WriteString("- Nodes: none\n")
	}
	for _, node := range r.Nodes {
		fmt.Fprintf(&sb, "- %s %s: %s", node.FileKey, node.NodeID, node.Status)
		if node.NodeName != "" {
			fmt.Fprintf(&sb, " (%s, %s)", node.NodeName, node.NodeType)
		}
		if node.Error != "" {
			fmt.Fprintf(&sb, " [%s]", node.Error)
		}
		sb.WriteString("\n")
	}
	if len(r.SetupGaps) > 0 {
		sb.WriteString("- Setup gaps:\n")
		for _, gap := range r.SetupGaps {
			fmt.Fprintf(&sb, "  - %s\n", gap)
		}
	}
	return sb.String()
}

func (r FigmaFetchReport) JSON() ([]byte, error) {
	data, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
