package docs

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultGoDocsBaseURL  = "https://pkg.go.dev"
	defaultNpmRegistryURL = "https://registry.npmjs.org"
	defaultPyPIBaseURL    = "https://pypi.org"
)

// Scraper fetches documentation from public package registries.
type Scraper struct {
	goDocsBaseURL  string
	npmRegistryURL string
	pypiBaseURL    string
	httpClient     *http.Client
}

// ScraperOption configures a Scraper instance.
type ScraperOption func(*Scraper)

// WithGoDocsBaseURL overrides the pkg.go.dev base URL (useful for tests).
func WithGoDocsBaseURL(url string) ScraperOption {
	return func(s *Scraper) { s.goDocsBaseURL = url }
}

// WithNpmRegistryURL overrides the npm registry base URL (useful for tests).
func WithNpmRegistryURL(url string) ScraperOption {
	return func(s *Scraper) { s.npmRegistryURL = url }
}

// WithPyPIBaseURL overrides the PyPI base URL (useful for tests).
func WithPyPIBaseURL(url string) ScraperOption {
	return func(s *Scraper) { s.pypiBaseURL = url }
}

// NewScraper creates a Scraper with default settings, applying any provided options.
func NewScraper(opts ...ScraperOption) *Scraper {
	s := &Scraper{
		goDocsBaseURL:  defaultGoDocsBaseURL,
		npmRegistryURL: defaultNpmRegistryURL,
		pypiBaseURL:    defaultPyPIBaseURL,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// FetchGoDocs fetches Go package documentation from pkg.go.dev.
// It extracts text from the <section id="pkg-overview"> element.
func (s *Scraper) FetchGoDocs(pkg string) (*DocResult, error) {
	url := fmt.Sprintf("%s/%s", s.goDocsBaseURL, pkg)
	body, err := s.get(url)
	if err != nil {
		return nil, fmt.Errorf("go docs fetch: %w", err)
	}

	content := extractGoSection(body)
	if content == "" {
		content = body // fallback: return raw body if section not found
	}

	return &DocResult{
		LibraryName: pkg,
		Package:     pkg,
		Source:      "scraper",
		Content:     content,
		Tokens:      len(content) / 4,
	}, nil
}

// extractGoSection extracts text content from <section id="pkg-overview">...</section>.
func extractGoSection(html string) string {
	const openTag = `<section id="pkg-overview">`
	const closeTag = `</section>`

	start := strings.Index(html, openTag)
	if start == -1 {
		return ""
	}
	start += len(openTag)

	end := strings.Index(html[start:], closeTag)
	if end == -1 {
		return ""
	}

	raw := html[start : start+end]
	return stripTags(raw)
}

// stripTags removes HTML tags from a string, leaving only text content.
func stripTags(s string) string {
	var b strings.Builder
	inTag := false
	for _, ch := range s {
		switch {
		case ch == '<':
			inTag = true
		case ch == '>':
			inTag = false
		case !inTag:
			b.WriteRune(ch)
		}
	}
	return strings.TrimSpace(b.String())
}

// FetchNpmDocs fetches npm package documentation from the registry JSON API.
// It uses the readme field, falling back to description if readme is empty.
func (s *Scraper) FetchNpmDocs(pkg string) (*DocResult, error) {
	url := fmt.Sprintf("%s/%s", s.npmRegistryURL, pkg)
	body, err := s.get(url)
	if err != nil {
		return nil, fmt.Errorf("npm docs fetch: %w", err)
	}

	var payload struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		Readme      string `json:"readme"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil, fmt.Errorf("npm docs parse: %w", err)
	}

	content := payload.Readme
	if content == "" {
		content = payload.Description
	}

	return &DocResult{
		LibraryName: pkg,
		Package:     pkg,
		Source:      "scraper",
		Content:     content,
		Tokens:      len(content) / 4,
	}, nil
}

// FetchPyPIDocs fetches Python package documentation from the PyPI JSON API.
// It uses the info.description field, falling back to info.summary.
func (s *Scraper) FetchPyPIDocs(pkg string) (*DocResult, error) {
	url := fmt.Sprintf("%s/pypi/%s/json", s.pypiBaseURL, pkg)
	body, err := s.get(url)
	if err != nil {
		return nil, fmt.Errorf("pypi docs fetch: %w", err)
	}

	var payload struct {
		Info struct {
			Name        string `json:"name"`
			Summary     string `json:"summary"`
			Description string `json:"description"`
		} `json:"info"`
	}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		return nil, fmt.Errorf("pypi docs parse: %w", err)
	}

	content := payload.Info.Description
	if content == "" {
		content = payload.Info.Summary
	}

	return &DocResult{
		LibraryName: pkg,
		Package:     pkg,
		Source:      "scraper",
		Content:     content,
		Tokens:      len(content) / 4,
	}, nil
}

// Fetch implements DocFetcher. It auto-detects the language/registry from the library name.
// Libraries with "/" or starting with "github.com" are treated as Go packages.
// Libraries with no "/" and no "." are treated as npm packages.
// Otherwise, PyPI is tried.
func (s *Scraper) Fetch(library, _ string) (*DocResult, error) {
	if strings.Contains(library, "/") || strings.HasPrefix(library, "github.com") {
		return s.FetchGoDocs(library)
	}
	if !strings.Contains(library, ".") {
		return s.FetchNpmDocs(library)
	}
	return s.FetchPyPIDocs(library)
}

// get performs an HTTP GET and returns the response body as a string.
func (s *Scraper) get(url string) (string, error) {
	resp, err := s.httpClient.Get(url) //nolint:noctx
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status %d for %s", resp.StatusCode, url)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}
