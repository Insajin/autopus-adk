package domainreadiness

// Input and output shapes for CompileCatalog. Split out of types.go to keep
// both files under the 300-line source limit.

type CompileOptions struct {
	ProjectDir string
	Lane       string
}

type CompileSummary struct {
	SchemaVersion string `json:"schema_version"`
	// Valid is the compile-level verdict: the catalog is structurally sound AND
	// every journey_pack_ref it declares resolves to a Journey Pack in the
	// project. Validation.Valid answers only the first half, which is how a
	// catalog full of dangling refs used to certify itself.
	Valid             bool                       `json:"valid"`
	ScenarioCount     int                        `json:"scenario_count"`
	CommandsExecuted  bool                       `json:"commands_executed"`
	SelectedLane      string                     `json:"selected_lane"`
	Validation        CatalogValidationReport    `json:"validation"`
	ScenarioPlans     []ScenarioPlan             `json:"scenario_plans"`
	RejectedScenarios []ScenarioValidationResult `json:"rejected_scenarios,omitempty"`
	JourneyRefGaps    []JourneyRefGap            `json:"journey_ref_gaps,omitempty"`
	CoveredDomains    []string                   `json:"covered_domains"`
	MissingDomains    []string                   `json:"missing_domains,omitempty"`
}

type ScenarioPlan struct {
	ScenarioID       string           `json:"scenario_id"`
	Domain           string           `json:"domain"`
	Owner            string           `json:"owner"`
	OwningRepo       string           `json:"owning_repo"`
	ScenarioMode     ScenarioMode     `json:"scenario_mode"`
	MutationBoundary MutationBoundary `json:"mutation_boundary"`
	Adapter          string           `json:"adapter,omitempty"`
	Command          *CommandShape    `json:"command,omitempty"`
	JourneyRefs      []string         `json:"journey_refs,omitempty"`
	LaneRefs         []string         `json:"lane_refs,omitempty"`
	ArtifactRefs     []string         `json:"artifact_refs,omitempty"`
	AcceptanceRefs   []string         `json:"acceptance_refs,omitempty"`
	SourceNeeds      []string         `json:"source_needs,omitempty"`
	ExpectedEvidence []string         `json:"expected_evidence,omitempty"`
	PassFailOracle   []string         `json:"pass_fail_oracle,omitempty"`
	CanaryRefs       []string         `json:"canary_refs,omitempty"`
	SetupGaps        []string         `json:"setup_gaps,omitempty"`
	RejectReasons    []UnsafeReason   `json:"reject_reasons,omitempty"`
}
