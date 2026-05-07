package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/docs"
)

// docsCacheDir returns the cache directory from env or a default under the user cache dir.
func docsCacheDir() string {
	if dir := os.Getenv("AUTO_DOCS_CACHE_DIR"); dir != "" {
		return dir
	}
	base, err := os.UserCacheDir()
	if err != nil {
		base = os.TempDir()
	}
	return filepath.Join(base, "autopus-adk", "docs")
}

// docsCacheAdapter wraps docs.Cache to implement docs.FetcherCache (DocResult-based).
type docsCacheAdapter struct {
	cache *docs.Cache
}

func (a *docsCacheAdapter) Get(key string) (*docs.DocResult, error) {
	entry, err := a.cache.Get(key)
	if err != nil || entry == nil {
		return nil, err
	}
	return &docs.DocResult{
		LibraryName: entry.LibraryID,
		Source:      "cache",
		Version:     entry.Version,
		SourceRef:   entry.SourceRef,
		CheckedAt:   entry.CachedAt,
		Content:     entry.Content,
		Tokens:      entry.Tokens,
	}, nil
}

func (a *docsCacheAdapter) Set(key string, result *docs.DocResult) error {
	return a.cache.Set(key, &docs.CacheEntry{
		LibraryID: result.LibraryName,
		Version:   result.Version,
		SourceRef: result.SourceRef,
		Content:   result.Content,
		Tokens:    result.Tokens,
	})
}

// newDocsCmd creates the top-level `auto docs` command with fetch and cache subcommands.
func newDocsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "docs",
		Short: "Documentation utilities",
	}
	cmd.AddCommand(newDocsFetchCmd())
	cmd.AddCommand(newDocsCacheCmd())
	return cmd
}

// newDocsFetchCmd creates the `auto docs fetch` subcommand.
func newDocsFetchCmd() *cobra.Command {
	var topic string
	var format string

	cmd := &cobra.Command{
		Use:   "fetch [library...]",
		Short: "Fetch library documentation",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Build fetcher with Context7, Scraper, and disk cache
			c7 := docs.NewContext7Client("")
			scraper := docs.NewScraper()
			cache := &docsCacheAdapter{
				cache: docs.NewCache(docsCacheDir(), 24*time.Hour),
			}
			fetcher := docs.NewFetcher(c7, scraper, cache)

			// Auto-detect libraries if no args provided
			if len(args) == 0 {
				return docsAutoDetect(cmd, fetcher, topic, format)
			}

			results, err := fetcher.FetchMultiple(args, topic)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no documentation found")
				return nil
			}

			return docsOutput(cmd, results, format)
		},
	}

	cmd.Flags().StringVar(&topic, "topic", "", "Documentation topic to focus on")
	cmd.Flags().StringVar(&format, "format", "text", "Output format: text or prompt")
	return cmd
}

// docsAutoDetect detects libraries from the project directory and fetches their docs.
func docsAutoDetect(cmd *cobra.Command, fetcher *docs.Fetcher, topic, format string) error {
	projectDir := os.Getenv("AUTO_DOCS_PROJECT_DIR")
	if projectDir == "" {
		var err error
		projectDir, err = os.Getwd()
		if err != nil {
			return err
		}
	}

	libraries := detectLibrariesFromDir(projectDir)
	if len(libraries) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no libraries detected")
		return nil
	}

	results, err := fetcher.FetchMultiple(libraries, topic)
	if err != nil {
		return err
	}
	if len(results) == 0 {
		fmt.Fprintln(cmd.OutOrStdout(), "no documentation found")
		return nil
	}
	return docsOutput(cmd, results, format)
}

// detectLibrariesFromDir checks for known manifest files and extracts library names.
func detectLibrariesFromDir(dir string) []string {
	if libs, err := docs.DetectFromGoMod(filepath.Join(dir, "go.mod")); err == nil && len(libs) > 0 {
		return docs.FilterStdLib("go", libs)
	}
	if libs, err := docs.DetectFromPackageJSON(filepath.Join(dir, "package.json")); err == nil && len(libs) > 0 {
		return libs
	}
	if libs, err := docs.DetectFromPyProjectToml(filepath.Join(dir, "pyproject.toml")); err == nil && len(libs) > 0 {
		return libs
	}
	return nil
}

// docsOutput writes fetched results to cmd output in the requested format.
func docsOutput(cmd *cobra.Command, results []*docs.DocResult, format string) error {
	if format == "prompt" {
		out, err := docs.FormatPromptInjection(results)
		if err != nil {
			return err
		}
		fmt.Fprint(cmd.OutOrStdout(), out)
		return nil
	}
	// Default text format
	for _, r := range results {
		fmt.Fprintf(cmd.OutOrStdout(), "### %s\n%s\n\n", r.LibraryName, r.Content)
	}
	return nil
}
