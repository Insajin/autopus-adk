package sigmap

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- formatSignature tests ---

func TestFormatSignature_Func(t *testing.T) {
	t.Parallel()

	sig := Signature{
		Name:    "NewAdapter",
		Kind:    "func",
		Params:  "(name string)",
		Returns: "(*Adapter, error)",
		Doc:     "Creates a new adapter.",
	}
	result := formatSignature(&sig)
	assert.Equal(t, "- `func NewAdapter(name string) (*Adapter, error)` — Creates a new adapter.", result)
}

func TestFormatSignature_FuncNoDoc(t *testing.T) {
	t.Parallel()

	sig := Signature{
		Name:    "Stop",
		Kind:    "func",
		Params:  "()",
		Returns: "",
	}
	result := formatSignature(&sig)
	assert.Equal(t, "- `func Stop()`", result)
}

func TestFormatSignature_Method(t *testing.T) {
	t.Parallel()

	sig := Signature{
		Name:     "Start",
		Kind:     "method",
		Receiver: "(a *Adapter)",
		Params:   "(ctx context.Context)",
		Returns:  "error",
		Doc:      "Starts the adapter.",
	}
	result := formatSignature(&sig)
	assert.Equal(t, "- `func (a *Adapter) Start(ctx context.Context) error` — Starts the adapter.", result)
}

func TestFormatSignature_GenericFunc(t *testing.T) {
	t.Parallel()

	sig := Signature{
		Name:       "Map",
		Kind:       "func",
		TypeParams: "[T any]",
		Params:     "(items []T)",
		Returns:    "([]T)",
		Doc:        "Maps over items.",
	}
	result := formatSignature(&sig)
	assert.Equal(t, "- `func Map[T any](items []T) ([]T)` — Maps over items.", result)
}

func TestFormatSignature_TypeStruct(t *testing.T) {
	t.Parallel()

	sig := Signature{
		Name: "Config",
		Kind: "type",
		Doc:  "Config holds harness settings.",
	}
	result := formatSignature(&sig)
	assert.Equal(t, "- `type Config struct` — Config holds harness settings.", result)
}

func TestFormatSignature_GenericType(t *testing.T) {
	t.Parallel()

	sig := Signature{
		Name:       "Stack",
		Kind:       "type",
		TypeParams: "[T any]",
		Doc:        "Stack is a LIFO structure.",
	}
	result := formatSignature(&sig)
	assert.Equal(t, "- `type Stack[T any] struct` — Stack is a LIFO structure.", result)
}

func TestFormatSignature_Interface(t *testing.T) {
	t.Parallel()

	sig := Signature{
		Name: "PlatformAdapter",
		Kind: "interface",
		Doc:  "PlatformAdapter defines the platform contract.",
	}
	result := formatSignature(&sig)
	assert.Equal(t, "- `interface PlatformAdapter` — PlatformAdapter defines the platform contract.", result)
}

func TestFormatSignature_InterfaceNoDoc(t *testing.T) {
	t.Parallel()

	sig := Signature{
		Name: "Runner",
		Kind: "interface",
	}
	result := formatSignature(&sig)
	assert.Equal(t, "- `interface Runner`", result)
}

// --- renderPackage tests ---

func TestRenderPackage(t *testing.T) {
	t.Parallel()

	pkg := &Package{
		Path: "pkg/adapter",
		Signatures: []Signature{
			{Name: "NewAdapter", Kind: "func", Params: "(name string)", Returns: "(*Adapter, error)", Doc: "Creates a new adapter."},
			{Name: "Start", Kind: "method", Receiver: "(a *Adapter)", Params: "(ctx context.Context)", Returns: "error", Doc: "Starts the adapter."},
		},
	}
	result := renderPackage(pkg)
	assert.Contains(t, result, "## pkg/adapter")
	assert.Contains(t, result, "func NewAdapter")
	assert.Contains(t, result, "func (a *Adapter) Start")
}

func TestRenderPackage_Empty(t *testing.T) {
	t.Parallel()

	pkg := &Package{Path: "pkg/empty"}
	result := renderPackage(pkg)
	assert.Contains(t, result, "## pkg/empty")
}

// --- countLines tests ---

func TestCountLines(t *testing.T) {
	t.Parallel()

	assert.Equal(t, 1, countLines("hello"))
	assert.Equal(t, 2, countLines("hello\nworld"))
	assert.Equal(t, 3, countLines("a\nb\nc"))
	assert.Equal(t, 0, countLines(""))
}

// --- filterByFanIn tests ---

func TestFilterByFanIn_UnderLimit(t *testing.T) {
	t.Parallel()

	pkgs := []Package{
		{Path: "pkg/a", FanIn: 5, Depth: 1, Signatures: []Signature{{Name: "A", Kind: "func"}}},
		{Path: "pkg/b", FanIn: 2, Depth: 1, Signatures: []Signature{{Name: "B", Kind: "func"}}},
	}
	result := filterByFanIn(pkgs, 1000)
	assert.Len(t, result, 2)
}

func TestFilterByFanIn_SortByFanInDesc(t *testing.T) {
	t.Parallel()

	// Create packages with enough signatures to exceed budget
	makeSigs := func(n int) []Signature {
		sigs := make([]Signature, n)
		for i := range sigs {
			sigs[i] = Signature{Name: "F", Kind: "func", Params: "()", Returns: ""}
		}
		return sigs
	}

	pkgs := []Package{
		{Path: "pkg/low", FanIn: 1, Depth: 1, Signatures: makeSigs(50)},
		{Path: "pkg/high", FanIn: 10, Depth: 1, Signatures: makeSigs(50)},
	}
	// Budget allows only one package (each pkg section ~103 lines including separator)
	result := filterByFanIn(pkgs, 110)
	require.Len(t, result, 1)
	assert.Equal(t, "pkg/high", result[0].Path)
}

func TestFilterByFanIn_TiebreakerDepth(t *testing.T) {
	t.Parallel()

	makeSigs := func(n int) []Signature {
		sigs := make([]Signature, n)
		for i := range sigs {
			sigs[i] = Signature{Name: "F", Kind: "func"}
		}
		return sigs
	}

	pkgs := []Package{
		{Path: "pkg/a/b/c", FanIn: 5, Depth: 3, Signatures: makeSigs(50)},
		{Path: "pkg/a", FanIn: 5, Depth: 1, Signatures: makeSigs(50)},
	}
	// Budget allows only one package (each pkg section ~103 lines including separator)
	result := filterByFanIn(pkgs, 110)
	require.Len(t, result, 1)
	assert.Equal(t, "pkg/a", result[0].Path)
}

// --- Render tests ---

func TestRender_Header(t *testing.T) {
	t.Parallel()

	sm := &SignatureMap{
		ModulePath: "github.com/example/app",
		Packages: []Package{
			{Path: "pkg/adapter", Signatures: []Signature{
				{Name: "New", Kind: "func", Doc: "Creates new."},
			}},
		},
	}
	result := Render(sm)
	assert.True(t, strings.HasPrefix(result, "# Signature Map\n"))
	assert.Contains(t, result, "> Auto-generated by `auto setup`. Do not edit manually.")
	assert.Contains(t, result, "Module: `github.com/example/app`")
	assert.Contains(t, result, "## pkg/adapter")
}

func TestRender_WithinLimit(t *testing.T) {
	t.Parallel()

	sm := &SignatureMap{
		ModulePath: "github.com/example/app",
		Packages: []Package{
			{Path: "pkg/a", Signatures: []Signature{{Name: "A", Kind: "func"}}},
		},
	}
	result := Render(sm)
	assert.NotContains(t, result, "Filtered:")
	assert.LessOrEqual(t, countLines(result), MaxLines)
}

func TestRender_ExceedsLimit_AddsFooter(t *testing.T) {
	t.Parallel()

	// Generate enough signatures to exceed 500 lines across many packages
	makePkg := func(path string, fanIn, depth, sigCount int) Package {
		sigs := make([]Signature, sigCount)
		for i := range sigs {
			sigs[i] = Signature{Name: "Func", Kind: "func", Params: "()", Returns: ""}
		}
		return Package{Path: path, FanIn: fanIn, Depth: depth, Signatures: sigs}
	}

	var pkgs []Package
	for i := 0; i < 20; i++ {
		pkgs = append(pkgs, makePkg("pkg/x"+string(rune('a'+i)), 1, 1, 30))
	}
	sm := &SignatureMap{ModulePath: "github.com/example/app", Packages: pkgs}

	result := Render(sm)
	assert.LessOrEqual(t, countLines(result), MaxLines)
	assert.Contains(t, result, "Filtered:")
	assert.Contains(t, result, "packages omitted (low fan-in)")
}

func TestRender_EmptyPackages(t *testing.T) {
	t.Parallel()

	sm := &SignatureMap{ModulePath: "github.com/example/app"}
	result := Render(sm)
	assert.Contains(t, result, "# Signature Map")
	assert.Contains(t, result, "Module: `github.com/example/app`")
}
