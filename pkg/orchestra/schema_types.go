package orchestra

// DebaterR1Output is the Round 1 independent analysis output.
type DebaterR1Output struct {
	CurrentState string          `json:"current_state"`
	Ideas        []IdeaOutput    `json:"ideas"`
	Assumptions  []AssumptionOut `json:"assumptions"`
	HMWQuestions []string        `json:"hmw_questions"`
}

// IdeaOutput represents a single idea in debate output.
type IdeaOutput struct {
	Title       string `json:"title"`
	Description string `json:"description"`
	Rationale   string `json:"rationale"`
	Risks       string `json:"risks"`
	Category    string `json:"category"`
}

// AssumptionOut represents a single assumption in debate output.
type AssumptionOut struct {
	Type        string `json:"type"` // value, feasibility, usability
	Description string `json:"description"`
	RiskLevel   string `json:"risk_level"` // high, medium, low
}

// DebaterR2Output is the Round 2 cross-pollination output.
type DebaterR2Output struct {
	Acknowledgments []AckOutput  `json:"acknowledgments"`
	IntegratedIdeas []IdeaOutput `json:"integrated_ideas"`
	Risks           []RiskOutput `json:"risks"`
}

// AckOutput represents an acknowledgment of another debater's point.
type AckOutput struct {
	Source string `json:"source"` // "Debater A", "Debater B"
	Point  string `json:"point"`
	Why    string `json:"why_strong"`
}

// RiskOutput represents a risk identified during debate.
type RiskOutput struct {
	Description string `json:"description"`
	Severity    string `json:"severity"` // high, medium, low
	Mitigation  string `json:"mitigation"`
}

// JudgeOutput is the final judge synthesis output.
type JudgeOutput struct {
	ConsensusAreas []ConsensusArea `json:"consensus_areas"`
	UniqueInsights []UniqueInsight `json:"unique_insights"`
	CrossRisks     []CrossRisk     `json:"cross_risks"`
	TopIdeas       []RankedIdea    `json:"top_ideas"`
	Recommendation string          `json:"recommendation"`
}

// ConsensusArea represents an area where debaters agreed.
type ConsensusArea struct {
	Idea         string   `json:"idea"`
	Participants []string `json:"participants"`
	Significance string   `json:"significance"`
}

// UniqueInsight represents a unique idea from one debater.
type UniqueInsight struct {
	Idea      string `json:"idea"`
	Proposer  string `json:"proposer"`
	WhyMissed string `json:"why_missed"`
}

// CrossRisk represents a risk flagged by multiple debaters.
type CrossRisk struct {
	Risk     string   `json:"risk"`
	Flaggers []string `json:"flaggers"`
	Severity string   `json:"severity"`
}

// RankedIdea represents a scored and ranked idea.
type RankedIdea struct {
	Rank       int     `json:"rank"`
	Title      string  `json:"title"`
	Impact     int     `json:"impact"`
	Confidence int     `json:"confidence"`
	Ease       int     `json:"ease"`
	Score      float64 `json:"score"` // Impact * Confidence * Ease / 100
}

// ReviewerOutput is the SPEC review output.
type ReviewerOutput struct {
	Findings      []Finding                  `json:"findings"`
	Verdict       string                     `json:"verdict"` // PASS, REVISE, REJECT
	Summary       string                     `json:"summary"`
	Checklist     []ReviewerChecklistOutput  `json:"checklist,omitempty"`
	FindingStatus []ReviewerFindingStatusOut `json:"finding_statuses,omitempty"`
}

// Finding represents a single review finding.
type Finding struct {
	Severity    string `json:"severity"`            // critical, major, minor, suggestion
	Category    string `json:"category,omitempty"`  // correctness, completeness, feasibility, style, security
	ScopeRef    string `json:"scope_ref,omitempty"` // REQ-001 or path:line
	Location    string `json:"location"`
	Description string `json:"description"`
	Suggestion  string `json:"suggestion"`
}

// ReviewerChecklistOutput represents a single checklist item result.
type ReviewerChecklistOutput struct {
	ID     string `json:"id"`
	Status string `json:"status"` // PASS, FAIL
	Reason string `json:"reason,omitempty"`
}

// ReviewerFindingStatusOut represents verification status for a prior finding.
type ReviewerFindingStatusOut struct {
	ID     string `json:"id"`
	Status string `json:"status"` // open, resolved, regressed
	Reason string `json:"reason,omitempty"`
}
