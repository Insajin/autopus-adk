package journey

type Pack struct {
	ID                  string              `yaml:"id" json:"id"`
	Title               string              `yaml:"title" json:"title"`
	Surface             string              `yaml:"surface" json:"surface"`
	Lanes               []string            `yaml:"lanes" json:"lanes"`
	Adapter             AdapterRef          `yaml:"adapter" json:"adapter"`
	Command             Command             `yaml:"command" json:"command"`
	Checks              []Check             `yaml:"checks" json:"checks"`
	Artifacts           []Artifact          `yaml:"artifacts" json:"artifacts"`
	SourceRefs          SourceRefs          `yaml:"source_refs" json:"source_refs"`
	ProfileRequirements ProfileRequirements `yaml:"profile_requirements" json:"profile_requirements"`
	PassFailAuthority   string              `yaml:"pass_fail_authority,omitempty" json:"pass_fail_authority,omitempty"`
	InputSource         string              `yaml:"source,omitempty" json:"input_source,omitempty"`
	Source              string              `yaml:"-" json:"source"`
}

type AdapterRef struct {
	ID string `yaml:"id" json:"id"`
}

type Command struct {
	Run          string   `yaml:"run,omitempty" json:"run,omitempty"`
	Argv         []string `yaml:"argv,omitempty" json:"argv,omitempty"`
	CWD          string   `yaml:"cwd,omitempty" json:"cwd,omitempty"`
	Timeout      string   `yaml:"timeout,omitempty" json:"timeout,omitempty"`
	EnvAllowlist []string `yaml:"env_allowlist,omitempty" json:"env_allowlist,omitempty"`
}

type Check struct {
	ID       string         `yaml:"id" json:"id"`
	Type     string         `yaml:"type" json:"type"`
	Expected map[string]any `yaml:"expected,omitempty" json:"expected,omitempty"`
}

type Artifact struct {
	Kind string `yaml:"kind,omitempty" json:"kind,omitempty"`
	Path string `yaml:"path,omitempty" json:"path,omitempty"`
	Root string `yaml:"root,omitempty" json:"root,omitempty"`
}

type SourceRefs struct {
	SourceSpec       string   `yaml:"source_spec,omitempty" json:"source_spec,omitempty"`
	AcceptanceRefs   []string `yaml:"acceptance_refs,omitempty" json:"acceptance_refs,omitempty"`
	OwnedPaths       []string `yaml:"owned_paths,omitempty" json:"owned_paths,omitempty"`
	DoNotModifyPaths []string `yaml:"do_not_modify_paths,omitempty" json:"do_not_modify_paths,omitempty"`
}

type ProfileRequirements struct {
	Capabilities []string `yaml:"capabilities,omitempty" json:"capabilities,omitempty"`
}

type ValidationError struct {
	Code    string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
