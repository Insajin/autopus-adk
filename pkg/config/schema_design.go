package config

// DesignConf controls optional DESIGN.md discovery and prompt injection.
type DesignConf struct {
	Enabled         bool     `yaml:"enabled"`
	Paths           []string `yaml:"paths,omitempty"`
	MaxContextLines int      `yaml:"max_context_lines,omitempty"`
	InjectOnReview  bool     `yaml:"inject_on_review"`
	InjectOnVerify  bool     `yaml:"inject_on_verify"`
	ExternalImports bool     `yaml:"external_imports"`
	UIFileGlobs     []string `yaml:"ui_globs,omitempty"`
}
