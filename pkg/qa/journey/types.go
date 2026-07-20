package journey

type Pack struct {
	ID                  string                   `yaml:"id" json:"id"`
	Title               string                   `yaml:"title" json:"title"`
	Surface             string                   `yaml:"surface" json:"surface"`
	Lanes               []string                 `yaml:"lanes" json:"lanes"`
	Adapter             AdapterRef               `yaml:"adapter" json:"adapter"`
	Command             Command                  `yaml:"command" json:"command"`
	Checks              []Check                  `yaml:"checks" json:"checks"`
	Artifacts           []Artifact               `yaml:"artifacts" json:"artifacts"`
	SourceRefs          SourceRefs               `yaml:"source_refs" json:"source_refs"`
	ProfileRequirements ProfileRequirements      `yaml:"profile_requirements" json:"profile_requirements"`
	GUI                 GUIPolicy                `yaml:"gui,omitempty" json:"gui,omitempty"`
	Mobile              MobilePolicy             `yaml:"mobile,omitempty" json:"mobile,omitempty"`
	DesktopObservation  DesktopObservationPolicy `yaml:"desktop_observation,omitempty" json:"desktop_observation,omitempty"`
	PassFailAuthority   string                   `yaml:"pass_fail_authority,omitempty" json:"pass_fail_authority,omitempty"`
	InputSource         string                   `yaml:"source,omitempty" json:"input_source,omitempty"`
	Source              string                   `yaml:"-" json:"source"`
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

type GUIPolicy struct {
	AllowedOrigins    []string             `yaml:"allowed_origins,omitempty" json:"allowed_origins,omitempty"`
	ForbiddenActions  []string             `yaml:"forbidden_actions,omitempty" json:"forbidden_actions,omitempty"`
	SelectorStrategy  string               `yaml:"selector_strategy,omitempty" json:"selector_strategy,omitempty"`
	NetworkPolicy     GUINetworkPolicy     `yaml:"network_policy,omitempty" json:"network_policy,omitempty"`
	ArtifactRetention GUIArtifactRetention `yaml:"artifact_retention,omitempty" json:"artifact_retention,omitempty"`
	ScreenMatrix      []GUIScreenMatrixRow `yaml:"screen_matrix,omitempty" json:"screen_matrix,omitempty"`
}

type GUIScreenMatrixRow struct {
	ID              string   `yaml:"id,omitempty" json:"id,omitempty"`
	Path            string   `yaml:"path,omitempty" json:"path,omitempty"`
	RequiredActions []string `yaml:"required_actions,omitempty" json:"required_actions,omitempty"`
}

type GUINetworkPolicy struct {
	Mode          string `yaml:"mode,omitempty" json:"mode,omitempty"`
	RetainHeaders bool   `yaml:"retain_headers,omitempty" json:"retain_headers,omitempty"`
	RetainBodies  bool   `yaml:"retain_bodies,omitempty" json:"retain_bodies,omitempty"`
}

type GUIArtifactRetention struct {
	PublishRaw bool `yaml:"publish_raw,omitempty" json:"publish_raw,omitempty"`
}

type MobilePolicy struct {
	FlowPath          string   `yaml:"flow_path,omitempty" json:"flow_path,omitempty"`
	DeviceTarget      string   `yaml:"device_target,omitempty" json:"device_target,omitempty"`
	AppArtifactDigest string   `yaml:"app_artifact_digest,omitempty" json:"app_artifact_digest,omitempty"`
	ForbiddenActions  []string `yaml:"forbidden_actions,omitempty" json:"forbidden_actions,omitempty"`
}

type DesktopObservationPolicy struct {
	Platform          string                       `yaml:"platform" json:"platform"`
	Operations        []string                     `yaml:"operations" json:"operations"`
	AppRef            string                       `yaml:"app_ref" json:"app_ref"`
	WindowRef         string                       `yaml:"window_ref" json:"window_ref"`
	RequiredLandmarks []DesktopObservationLandmark `yaml:"required_landmarks" json:"required_landmarks"`
}

type DesktopObservationLandmark struct {
	Role          string `yaml:"role" json:"role"`
	Name          string `yaml:"name" json:"name"`
	RequiredState string `yaml:"required_state" json:"required_state"`
}

type ValidationError struct {
	Code    string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}
