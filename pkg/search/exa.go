// Package search는 외부 검색 및 해시 기능을 제공한다.
package search

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const defaultExaBaseURL = "https://api.exa.ai"

// SearchResult는 검색 결과이다.
type SearchResult struct {
	Title   string // 결과 제목
	URL     string // 결과 URL
	Snippet string // 결과 요약
}

// ExaClient는 Exa 검색 API 클라이언트이다.
type ExaClient struct {
	apiKey  string
	baseURL string
	http    *http.Client
}

// ExaOption은 ExaClient 옵션 함수이다.
type ExaOption func(*ExaClient)

// WithExaBaseURL는 Exa API 기본 URL을 설정한다.
func WithExaBaseURL(url string) ExaOption {
	return func(c *ExaClient) {
		c.baseURL = url
	}
}

// NewExaClient는 API 키로 Exa 클라이언트를 생성한다.
func NewExaClient(apiKey string, opts ...ExaOption) *ExaClient {
	c := &ExaClient{
		apiKey:  apiKey,
		baseURL: defaultExaBaseURL,
		http:    &http.Client{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// NewExaClientFromEnv는 환경 변수 EXA_API_KEY로 클라이언트를 생성한다.
func NewExaClientFromEnv(opts ...ExaOption) *ExaClient {
	return NewExaClient(os.Getenv("EXA_API_KEY"), opts...)
}

// Search는 Exa API로 검색을 수행한다.
func (c *ExaClient) Search(query string, numResults int) ([]SearchResult, error) {
	payload := map[string]interface{}{
		"query":      query,
		"numResults": numResults,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("요청 직렬화 실패: %w", err)
	}

	req, err := http.NewRequest("POST", c.baseURL+"/search", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("요청 생성 실패: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("API 요청 실패: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API 오류 %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Snippet string `json:"snippet"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("응답 파싱 실패: %w", err)
	}

	var results []SearchResult
	for _, r := range result.Results {
		results = append(results, SearchResult{
			Title:   r.Title,
			URL:     r.URL,
			Snippet: r.Snippet,
		})
	}

	return results, nil
}
