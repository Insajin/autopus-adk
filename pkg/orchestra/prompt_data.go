package orchestra

// PromptData holds the template data for orchestra prompt rendering.
type PromptData struct {
	// Project context (shared across all roles).
	ProjectName    string
	ProjectSummary string
	TechStack      string
	Components     []string
	MustReadFiles  []string
	RelevantPaths  []RelevantPath
	TargetModule   string
	MaxTurns       int

	// Topic is the debate/review subject.
	Topic string

	// Schema control: "prompt" embeds schema JSON inline, "" omits it.
	SchemaMethod string
	SchemaJSON   string

	// Round 2 fields.
	Round           int
	PreviousRound   int
	PreviousResults []PreviousResult

	// Judge fields.
	AllResults []JudgeResult

	// Reviewer fields.
	SpecContent string
	CodeContext string
}

// RelevantPath describes a code path relevant to the topic.
type RelevantPath struct {
	Path        string
	Description string
}

// PreviousResult holds an anonymized participant's output from a prior round.
type PreviousResult struct {
	Alias  string
	Output string
}

// JudgeResult holds all rounds of a single participant for judge evaluation.
type JudgeResult struct {
	Alias  string
	Round1 string
	Round2 string
}
