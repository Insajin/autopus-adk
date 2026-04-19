// Package issue provides domain types and logic for the auto issue reporter.
package issue

// IssueContext holds environment and error context for a report.
type IssueContext struct {
	ErrorMessage string
	Command      string
	ExitCode     int
	OS           string
	GoVersion    string
	AutoVersion  string
	Platform     string
	ConfigYAML   string // sanitized autopus.yaml content
	Telemetry    string // recent pipeline run summary
}

// IssueReport is a complete issue report ready for formatting.
type IssueReport struct {
	Title   string
	Context IssueContext
	Hash    string // xxhash hex string for dedup
	Labels  []string
	Repo    string // target GitHub repo (owner/repo)
}

// SubmitResult holds the outcome of a GitHub issue submission.
type SubmitResult struct {
	IssueURL     string
	IssueNumber  int
	WasDuplicate bool
	Action       string // "created", "commented", "skipped"
}

// Config holds issue reporter configuration from autopus.yaml.
type Config struct {
	Repo             string   `yaml:"repo"`
	Labels           []string `yaml:"labels"`
	AutoSubmit       bool     `yaml:"auto_submit"`
	RateLimitMinutes int      `yaml:"rate_limit_minutes"`
}
