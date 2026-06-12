package setup

import (
	"testing"
)

// TestDetectFrameworkVersionNode covers the Node package.json path through
// detectFrameworkVersion for several framework aliases.
func TestDetectFrameworkVersionNode(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "package.json", `{
		"dependencies": {"next": "14.2.0", "react": "18.3.1"},
		"devDependencies": {"svelte": "4.2.0"}
	}`)

	cases := map[string]string{
		"nextjs": "14.2.0",
		"react":  "18.3.1",
		"svelte": "4.2.0",
		"vue":    "",
	}
	for framework, want := range cases {
		if got := detectFrameworkVersion(dir, framework); got != want {
			t.Fatalf("detectFrameworkVersion(%s) = %q, want %q", framework, got, want)
		}
	}
}

// TestDetectFrameworkVersionGoModules covers the go.mod resolution branch.
func TestDetectFrameworkVersionGoModules(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "go.mod", "module x\n\nrequire (\n\tgithub.com/gin-gonic/gin v1.9.1\n\tgithub.com/labstack/echo v4.11.0\n)\n")

	if got := detectFrameworkVersion(dir, "gin"); got != "v1.9.1" {
		t.Fatalf("gin version = %q, want v1.9.1", got)
	}
	if got := detectFrameworkVersion(dir, "echo"); got != "v4.11.0" {
		t.Fatalf("echo version = %q, want v4.11.0", got)
	}
	if got := detectFrameworkVersion(dir, "chi"); got != "" {
		t.Fatalf("chi version = %q, want empty", got)
	}
}

// TestDetectPythonDependencyVersion covers requirements.txt parsing including
// the operator-trim path in extractDependencyVersion.
func TestDetectPythonDependencyVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "requirements.txt", "fastapi==0.110.0\nuvicorn>=0.29\n")

	if got := detectFrameworkVersion(dir, "fastapi"); got != "0.110.0" {
		t.Fatalf("fastapi version = %q, want 0.110.0", got)
	}
	if got := detectFrameworkVersion(dir, "django"); got != "" {
		t.Fatalf("django version = %q, want empty (absent)", got)
	}
}

// TestDetectCargoDependencyVersion covers the previously-uncovered Cargo.toml
// parser for the axum framework.
func TestDetectCargoDependencyVersion(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "Cargo.toml", "[dependencies]\naxum = \"0.7.5\"\nserde = { version = \"1.0\" }\n")

	if got := detectFrameworkVersion(dir, "axum"); got != "0.7.5" {
		t.Fatalf("axum version = %q, want 0.7.5", got)
	}
	// Direct call: absent dependency yields empty.
	if got := detectCargoDependencyVersion(dir, "tokio"); got != "" {
		t.Fatalf("tokio version = %q, want empty", got)
	}
	// Missing Cargo.toml yields empty.
	if got := detectCargoDependencyVersion(t.TempDir(), "axum"); got != "" {
		t.Fatalf("missing Cargo.toml = %q, want empty", got)
	}
}

// TestDetectFrameworkVersionUnknown asserts the default empty branch.
func TestDetectFrameworkVersionUnknown(t *testing.T) {
	if got := detectFrameworkVersion(t.TempDir(), "cobol-web"); got != "" {
		t.Fatalf("unknown framework = %q, want empty", got)
	}
}
