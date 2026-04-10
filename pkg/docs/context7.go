package docs

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultContext7BaseURL = "https://context7.com/api/v1"

// Context7Client fetches documentation from the Context7 API.
type Context7Client struct {
	baseURL string
	http    *http.Client
}

// NewContext7Client creates a new Context7Client with the given base URL.
// If baseURL is empty, the default Context7 API URL is used.
func NewContext7Client(baseURL string) *Context7Client {
	if baseURL == "" {
		baseURL = defaultContext7BaseURL
	}
	return &Context7Client{
		baseURL: baseURL,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

// ResolveLibrary resolves a library name to a LibraryInfo using the Context7 API.
// Returns ErrLibraryNotFound if the library cannot be found (HTTP 404).
func (c *Context7Client) ResolveLibrary(name string) (*LibraryInfo, error) {
	endpoint := fmt.Sprintf("%s/libraries?name=%s", c.baseURL, url.QueryEscape(name))

	resp, err := c.http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("resolve library request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("library %q: %w", name, ErrLibraryNotFound)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, wrapContext7APIError("resolve library", resp.StatusCode, body)
	}

	var raw struct {
		ID      string `json:"id"`
		Name    string `json:"name"`
		Version string `json:"version"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("resolve library response parse error: %w", err)
	}

	return &LibraryInfo{
		ID:      raw.ID,
		Name:    raw.Name,
		Version: raw.Version,
	}, nil
}

// GetDocs fetches documentation for a library ID and topic from the Context7 API.
// Returns an error for any non-200 response.
func (c *Context7Client) GetDocs(libraryID, topic string) (*DocContent, error) {
	if libraryID == "" {
		return nil, fmt.Errorf("libraryID must not be empty")
	}

	endpoint := fmt.Sprintf("%s/libraries/%s/docs?topic=%s",
		c.baseURL,
		url.PathEscape(libraryID),
		url.QueryEscape(topic),
	)

	resp, err := c.http.Get(endpoint)
	if err != nil {
		return nil, fmt.Errorf("get docs request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, wrapContext7APIError("get docs", resp.StatusCode, body)
	}

	var raw struct {
		Content string `json:"content"`
		Tokens  int    `json:"tokens"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("get docs response parse error: %w", err)
	}

	return &DocContent{
		Content: raw.Content,
		Tokens:  raw.Tokens,
	}, nil
}

// Fetch implements the DocFetcher interface. It resolves the library name then
// fetches documentation for the given topic, returning a DocResult.
func (c *Context7Client) Fetch(library, topic string) (*DocResult, error) {
	info, err := c.ResolveLibrary(library)
	if err != nil {
		if errors.Is(err, ErrLibraryNotFound) {
			return nil, err
		}
		return nil, fmt.Errorf("fetch: resolve %q: %w", library, err)
	}

	content, err := c.GetDocs(info.ID, topic)
	if err != nil {
		return nil, fmt.Errorf("fetch: get docs for %q: %w", library, err)
	}

	return &DocResult{
		LibraryName: info.Name,
		Package:     info.ID,
		Source:      "context7",
		Content:     content.Content,
		Tokens:      content.Tokens,
	}, nil
}

func wrapContext7APIError(op string, statusCode int, body []byte) error {
	message := strings.TrimSpace(string(body))
	if isContext7QuotaError(statusCode, message) {
		return fmt.Errorf("%s API error %d: %w: %s", op, statusCode, ErrQuotaExceeded, message)
	}
	if statusCode == http.StatusTooManyRequests {
		return fmt.Errorf("%s API error %d: %w: %s", op, statusCode, ErrRateLimited, message)
	}
	return fmt.Errorf("%s API error %d: %s", op, statusCode, message)
}

func isContext7QuotaError(statusCode int, message string) bool {
	if statusCode == http.StatusPaymentRequired {
		return true
	}
	lower := strings.ToLower(message)
	return strings.Contains(lower, "quota") || strings.Contains(lower, "credit")
}
