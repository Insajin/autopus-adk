// Package spec는 SPEC 문서 관리 및 EARS 요구사항 파싱을 제공한다.
// EARS: Easy Approach to Requirements Syntax (Mavin et al., 2009, IEEE RE Conference)
package spec

import "io/fs"

// EARSType은 EARS 패턴 유형이다.
type EARSType string

const (
	EARSUbiquitous  EARSType = "Ubiquitous"  // 시스템은 항상 SHALL
	EARSEventDriven EARSType = "EventDriven" // WHEN...THEN
	EARSStateDriven EARSType = "StateDriven" // WHERE...THEN
	EARSUnwanted    EARSType = "Unwanted"    // IF...THEN (비정상 상황)
	EARSOptional    EARSType = "Optional"    // WHEN...IF...THEN (복합)
)

// Requirement는 단일 요구사항이다.
type Requirement struct {
	ID          string   // 요구사항 ID (예: REQ-001)
	Type        EARSType // EARS 유형
	Description string   // 요구사항 설명
	TracesTo    string   // 추적 대상 (예: SPEC-001, UC-001)
}

// GherkinStep is a single step in a Gherkin scenario (Given/When/Then).
type GherkinStep struct {
	Keyword string // "Given", "When", "Then", "And", "But"
	Text    string // step description
}

// Criterion은 인수 기준이다.
type Criterion struct {
	ID          string        // 기준 ID
	Description string        // 기준 설명
	TracesTo    string        // 추적 대상
	Priority    string        // "Must", "Should", "Nice" (default: "Must")
	Steps       []GherkinStep // Gherkin steps (Given/When/Then)
}

// SpecDocument는 SPEC 문서이다.
type SpecDocument struct {
	ID                 string        // SPEC ID (예: SPEC-AUTH-001)
	Title              string        // 제목
	Version            string        // 버전
	Status             string        // 상태 (draft, review, approved, done)
	Requirements       []Requirement // 요구사항 목록
	AcceptanceCriteria []Criterion   // 인수 기준 목록
	RawContent         string        // spec.md 원문 (review prompt에 전문 포함용)
}

// ValidationError는 검증 오류이다.
type ValidationError struct {
	Field   string // 오류 필드
	Message string // 오류 메시지
	Level   string // 오류 수준: error, warning
}

func (e ValidationError) Error() string {
	return e.Message
}

// ReviewVerdict is the outcome of a multi-provider SPEC review.
type ReviewVerdict string

const (
	VerdictPass   ReviewVerdict = "PASS"
	VerdictRevise ReviewVerdict = "REVISE"
	VerdictReject ReviewVerdict = "REJECT"
)

// FindingStatus represents the lifecycle state of a review finding.
type FindingStatus string

const (
	FindingStatusOpen       FindingStatus = "open"
	FindingStatusResolved   FindingStatus = "resolved"
	FindingStatusRegressed  FindingStatus = "regressed"
	FindingStatusDeferred   FindingStatus = "deferred"
	FindingStatusOutOfScope FindingStatus = "out_of_scope"
)

// FindingCategory classifies the domain of a review finding.
type FindingCategory string

const (
	FindingCategoryCorrectness  FindingCategory = "correctness"
	FindingCategoryCompleteness FindingCategory = "completeness"
	FindingCategoryFeasibility  FindingCategory = "feasibility"
	FindingCategoryStyle        FindingCategory = "style"
	FindingCategorySecurity     FindingCategory = "security"
)

// ReviewMode determines the review phase behavior.
type ReviewMode string

const (
	ReviewModeDiscover ReviewMode = "discover"
	ReviewModeVerify   ReviewMode = "verify"
)

// ReviewPromptOptions configures mode-aware review prompt generation.
type ReviewPromptOptions struct {
	Mode               ReviewMode      // discover or verify
	PriorFindings      []ReviewFinding // unresolved findings from previous round (verify mode)
	StaticFindings     []ReviewFinding // pre-seeded findings from static analysis (discover mode)
	SpecDir            string          // path to spec directory for loading auxiliary docs
	PassCriteria       string          // overrides default verdict decision rules in prompt
	DocContextMaxLines int             // max lines per auxiliary doc (0 = use default 200)
	checklistFS        fs.FS
	checklistDiskPaths []string
}

// ChecklistStatus represents the status of a checklist item.
type ChecklistStatus string

const (
	ChecklistStatusPass ChecklistStatus = "PASS"
	ChecklistStatusFail ChecklistStatus = "FAIL"
)

// ChecklistOutcome is a structured checklist result emitted by a provider.
type ChecklistOutcome struct {
	ID       string          // checklist item ID (for example, Q-CORR-01)
	Status   ChecklistStatus // PASS or FAIL
	Reason   string          // optional reason, required for FAIL in reviewer guidance
	Provider string          // provider that emitted the outcome
	Revision int             // revision iteration (0 = first review)
}

// ReviewFinding is a single issue found during review.
type ReviewFinding struct {
	Provider     string          // provider that found the issue
	Severity     string          // critical, major, minor, suggestion
	Description  string          // finding description
	ID           string          // revision-agnostic ID: F-001, F-002, ...
	Status       FindingStatus   // open, resolved, regressed, deferred, out_of_scope
	Category     FindingCategory // correctness, completeness, feasibility, style, security
	ScopeRef     string          // requirement ID or normalized file path
	FirstSeenRev int             // revision when first discovered
	LastSeenRev  int             // most recent revision evaluated
	EscapeHatch  bool            // true if added via critical/security escape hatch in verify mode
}

// ReviewResult is the aggregated result of a multi-provider review.
type ReviewResult struct {
	SpecID            string             // target SPEC ID
	Verdict           ReviewVerdict      // final verdict
	Findings          []ReviewFinding    // all findings from all providers
	ChecklistOutcomes []ChecklistOutcome // structured checklist outcomes from provider responses
	Responses         []string           // raw provider responses
	Revision          int                // revision iteration (0 = first review)
}
