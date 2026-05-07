package docs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestScraper_FetchGoDocs verifies that Go package docs are fetched from pkg.go.dev.
// Given: a mock pkg.go.dev server returning package documentation
// When: FetchGoDocs is called with a package name
// Then: documentation content is returned with non-empty body
func TestScraper_FetchGoDocs(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<html><body><section id="pkg-overview">Package cobra is a CLI library.</section></body></html>`))
	}))
	defer srv.Close()

	scraper := NewScraper(WithGoDocsBaseURL(srv.URL))
	result, err := scraper.FetchGoDocs("github.com/spf13/cobra")

	require.NoError(t, err)
	assert.NotEmpty(t, result.Content)
	assert.Equal(t, "github.com/spf13/cobra", result.Package)
	assert.NotEmpty(t, result.SourceRef)
	assert.False(t, result.CheckedAt.IsZero())
}

func TestScraper_FetchGoDocsPreservesProxyVersionEvidence(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if strings.HasSuffix(r.URL.Path, "/@latest") {
			_, _ = w.Write([]byte(`{"Version":"v1.10.0"}`))
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(`<html><body><section id="pkg-overview">Package gin is a web framework.</section></body></html>`))
	}))
	defer srv.Close()

	scraper := NewScraper(WithGoDocsBaseURL(srv.URL), WithGoProxyBaseURL(srv.URL))
	result, err := scraper.FetchGoDocs("github.com/gin-gonic/gin")

	require.NoError(t, err)
	assert.Equal(t, "v1.10.0", result.Version)
	assert.Contains(t, result.SourceRef, "/@latest")
}

// TestScraper_FetchNpmDocs verifies that npm package docs are fetched via registry JSON API.
// Given: a mock npm registry server returning package metadata
// When: FetchNpmDocs is called with a package name
// Then: package description and documentation are returned
func TestScraper_FetchNpmDocs(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"express","description":"Fast web framework","readme":"# Express\nMinimal web framework.","dist-tags":{"latest":"5.1.0"}}`))
	}))
	defer srv.Close()

	scraper := NewScraper(WithNpmRegistryURL(srv.URL))
	result, err := scraper.FetchNpmDocs("express")

	require.NoError(t, err)
	assert.NotEmpty(t, result.Content)
	assert.Equal(t, "express", result.Package)
	assert.Contains(t, result.Content, "Express")
	assert.Equal(t, "5.1.0", result.Version)
	assert.False(t, result.CheckedAt.IsZero())
}

// TestScraper_FetchPyPIDocs verifies that PyPI package docs are fetched from the JSON API.
// Given: a mock PyPI server returning package info
// When: FetchPyPIDocs is called with a package name
// Then: package summary and documentation are returned
func TestScraper_FetchPyPIDocs(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"info":{"name":"requests","version":"2.32.0","summary":"HTTP library for Python","description":"# Requests\nSimple HTTP library."}}`))
	}))
	defer srv.Close()

	scraper := NewScraper(WithPyPIBaseURL(srv.URL))
	result, err := scraper.FetchPyPIDocs("requests")

	require.NoError(t, err)
	assert.NotEmpty(t, result.Content)
	assert.Equal(t, "requests", result.Package)
	assert.Equal(t, "2.32.0", result.Version)
}
