package search

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const defaultContext7BaseURL = "https://context7.com/api/v1"

// Context7Client는 Context7 문서 API 클라이언트이다.
type Context7Client struct {
	baseURL string
	http    *http.Client
}

// Context7Option은 Context7Client 옵션 함수이다.
type Context7Option func(*Context7Client)

// WithContext7BaseURL은 Context7 API 기본 URL을 설정한다.
func WithContext7BaseURL(u string) Context7Option {
	return func(c *Context7Client) {
		c.baseURL = u
	}
}

// NewContext7Client는 Context7 클라이언트를 생성한다.
func NewContext7Client(opts ...Context7Option) *Context7Client {
	c := &Context7Client{
		baseURL: defaultContext7BaseURL,
		http:    &http.Client{},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// ResolveLibrary는 라이브러리명으로 Context7 라이브러리 ID를 조회한다.
func (c *Context7Client) ResolveLibrary(name string) (string, error) {
	endpoint := fmt.Sprintf("%s/libraries?name=%s", c.baseURL, url.QueryEscape(name))

	resp, err := c.http.Get(endpoint)
	if err != nil {
		return "", fmt.Errorf("라이브러리 조회 실패: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API 오류 %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("응답 파싱 실패: %w", err)
	}

	return result.ID, nil
}

// GetDocs는 라이브러리 ID와 주제로 문서를 조회한다.
func (c *Context7Client) GetDocs(libraryID string, topic string) (string, error) {
	if libraryID == "" {
		return "", fmt.Errorf("라이브러리 ID가 비어있습니다")
	}

	endpoint := fmt.Sprintf("%s/libraries/%s/docs?topic=%s",
		c.baseURL,
		url.PathEscape(libraryID),
		url.QueryEscape(topic),
	)

	resp, err := c.http.Get(endpoint)
	if err != nil {
		return "", fmt.Errorf("문서 조회 실패: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("API 오류 %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Content string `json:"content"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("응답 파싱 실패: %w", err)
	}

	return result.Content, nil
}
