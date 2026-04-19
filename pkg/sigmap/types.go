// Package sigmap provides type definitions for the Signature Map system.
// It models the exported API surface of a Go module as a structured inventory.
package sigmap

// Signature represents a single exported symbol in a Go package.
type Signature struct {
	Name       string // e.g. "NewServer"
	Kind       string // "func", "method", "type", "interface"
	Receiver   string // e.g. "(s *Server)" — empty for non-methods
	Params     string // e.g. "(addr string, port int)"
	Returns    string // e.g. "(error)"
	TypeParams string // e.g. "[T any, U comparable]" — for generics
	Doc        string // GoDoc first sentence
}

// Package groups signatures by package path.
type Package struct {
	Path       string // e.g. "pkg/adapter"
	Name       string // e.g. "adapter"
	FanIn      int    // number of files importing this package
	Depth      int    // path depth (number of "/" in path)
	Signatures []Signature
}

// SignatureMap is the complete API inventory of a Go module.
type SignatureMap struct {
	ModulePath string // go.mod module path
	Packages   []Package
	Warnings   []string // parse error warnings
}
