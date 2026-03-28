package content_test

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/content"
)

// hasPattern reports whether exclusions contain a given pattern.
func hasPattern(exclusions []content.FileSizeExclusion, pattern string) bool {
	for _, e := range exclusions {
		if e.Pattern == pattern {
			return true
		}
	}
	return false
}

// TestCommonExclusionsAlwaysPresent verifies that doc and config patterns are always included.
func TestCommonExclusionsAlwaysPresent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		stack     string
		framework string
	}{
		{"go", ""},
		{"typescript", "nextjs"},
		{"python", "django"},
		{"rust", "axum"},
		{"", ""},
		{"unknown", "unknown"},
	}

	commonPatterns := []string{
		"*.md", "*.txt", "*.rst",
		"*.yaml", "*.yml", "*.json", "*.toml",
	}

	for _, tc := range cases {
		exclusions := content.FileSizeExclusions(tc.stack, tc.framework)
		for _, p := range commonPatterns {
			if !hasPattern(exclusions, p) {
				t.Errorf("stack=%q framework=%q: expected common pattern %q to be present", tc.stack, tc.framework, p)
			}
		}
	}
}

// TestGoStackExclusions verifies Go-specific patterns are returned for the go stack.
func TestGoStackExclusions(t *testing.T) {
	t.Parallel()

	exclusions := content.FileSizeExclusions("go", "")

	goPatterns := []string{
		"*_generated.go",
		"*.pb.go",
		"*_gen.go",
		"*_string.go",
		"mock_*.go",
		"go.sum",
	}

	for _, p := range goPatterns {
		if !hasPattern(exclusions, p) {
			t.Errorf("go stack: expected pattern %q to be present", p)
		}
	}
}

// TestTypescriptStackExclusions verifies TypeScript-specific patterns are returned.
func TestTypescriptStackExclusions(t *testing.T) {
	t.Parallel()

	exclusions := content.FileSizeExclusions("typescript", "")

	tsPatterns := []string{
		"*.generated.ts",
		"*.d.ts",
		"*.min.js",
		"*.min.css",
		"dist/**",
		"node_modules/**",
		"package-lock.json",
	}

	for _, p := range tsPatterns {
		if !hasPattern(exclusions, p) {
			t.Errorf("typescript stack: expected pattern %q to be present", p)
		}
	}
}

// TestPythonStackExclusions verifies Python-specific patterns are returned.
func TestPythonStackExclusions(t *testing.T) {
	t.Parallel()

	exclusions := content.FileSizeExclusions("python", "")

	pyPatterns := []string{
		"*_pb2.py",
		"*_pb2_grpc.py",
		"*.pyc",
		"__pycache__/**",
		"poetry.lock",
		"Pipfile.lock",
	}

	for _, p := range pyPatterns {
		if !hasPattern(exclusions, p) {
			t.Errorf("python stack: expected pattern %q to be present", p)
		}
	}
}

// TestRustStackExclusions verifies Rust-specific patterns are returned.
func TestRustStackExclusions(t *testing.T) {
	t.Parallel()

	exclusions := content.FileSizeExclusions("rust", "")

	rustPatterns := []string{
		"*.generated.rs",
		"build.rs",
		"Cargo.lock",
	}

	for _, p := range rustPatterns {
		if !hasPattern(exclusions, p) {
			t.Errorf("rust stack: expected pattern %q to be present", p)
		}
	}
}

// TestFrameworkPatternsAppended verifies framework-specific patterns are included.
func TestFrameworkPatternsAppended(t *testing.T) {
	t.Parallel()

	cases := []struct {
		stack     string
		framework string
		patterns  []string
	}{
		{"typescript", "nextjs", []string{".next/**", "next-env.d.ts"}},
		{"typescript", "nuxtjs", []string{".nuxt/**", ".output/**"}},
		{"python", "django", []string{"*/migrations/*.py"}},
		{"typescript", "react", []string{"build/**"}},
		{"typescript", "vue", []string{"dist/**"}},
		{"typescript", "svelte", []string{".svelte-kit/**"}},
		{"typescript", "nestjs", []string{"dist/**"}},
	}

	for _, tc := range cases {
		exclusions := content.FileSizeExclusions(tc.stack, tc.framework)
		for _, p := range tc.patterns {
			if !hasPattern(exclusions, p) {
				t.Errorf("stack=%q framework=%q: expected framework pattern %q to be present", tc.stack, tc.framework, p)
			}
		}
	}
}

// TestUnknownStackReturnsOnlyCommon verifies unknown stacks return only common patterns.
func TestUnknownStackReturnsOnlyCommon(t *testing.T) {
	t.Parallel()

	exclusions := content.FileSizeExclusions("cobol", "")

	// Must contain common patterns
	if !hasPattern(exclusions, "*.md") {
		t.Error("unknown stack: expected *.md to be present")
	}

	// Must NOT contain Go/TS/Python/Rust patterns
	stackSpecific := []string{
		"*_generated.go", "*.pb.go", "go.sum",
		"*.d.ts", "node_modules/**",
		"*_pb2.py", "poetry.lock",
		"Cargo.lock",
	}
	for _, p := range stackSpecific {
		if hasPattern(exclusions, p) {
			t.Errorf("unknown stack: expected pattern %q to NOT be present", p)
		}
	}
}

// TestEmptyStackAndFramework verifies that empty inputs return only common patterns.
func TestEmptyStackAndFramework(t *testing.T) {
	t.Parallel()

	exclusions := content.FileSizeExclusions("", "")

	commonPatterns := []string{"*.md", "*.yaml", "*.json"}
	for _, p := range commonPatterns {
		if !hasPattern(exclusions, p) {
			t.Errorf("empty stack: expected common pattern %q to be present", p)
		}
	}

	// No stack-specific patterns expected
	if hasPattern(exclusions, "go.sum") || hasPattern(exclusions, "*.d.ts") {
		t.Error("empty stack: stack-specific patterns should not be present")
	}
}

// TestNoExtraFrameworkExclusions verifies frameworks like gin/axum add no extra patterns.
func TestNoExtraFrameworkExclusions(t *testing.T) {
	t.Parallel()

	noExtraFrameworks := []string{"gin", "echo", "chi", "fastapi", "flask", "axum"}

	for _, fw := range noExtraFrameworks {
		goExclusions := content.FileSizeExclusions("go", fw)
		tsExclusions := content.FileSizeExclusions("typescript", fw)
		goBase := content.FileSizeExclusions("go", "")
		tsBase := content.FileSizeExclusions("typescript", "")

		if len(goExclusions) != len(goBase) {
			t.Errorf("framework=%q with go: expected no extra exclusions (got %d, want %d)", fw, len(goExclusions), len(goBase))
		}
		if len(tsExclusions) != len(tsBase) {
			t.Errorf("framework=%q with typescript: expected no extra exclusions (got %d, want %d)", fw, len(tsExclusions), len(tsBase))
		}
	}
}
