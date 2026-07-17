package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/insajin/autopus-adk/internal/cli/tui"
	"github.com/insajin/autopus-adk/pkg/config"
	"github.com/insajin/autopus-adk/pkg/learn"
	"github.com/insajin/autopus-adk/pkg/memindex"
)

const evidenceFreshnessMaxAge = 30 * 24 * time.Hour

type freshnessCheckResult struct {
	id       string
	present  bool
	hasWarn  bool
	severity string
	status   string
	ageDays  float64
	detail   string
}

func checkFreshness(dir string) ([]freshnessCheckResult, error) {
	var results []freshnessCheckResult

	// 1. learnings
	learnStore, err := learn.NewStore(dir)
	if err == nil {
		learnPath := filepath.Join(dir, ".autopus", "learnings", "pipeline.jsonl")
		if _, err := os.Stat(learnPath); err == nil {
			entries, _, err := learnStore.ReadTolerant()
			var latestTime time.Time
			if err == nil && len(entries) > 0 {
				for _, e := range entries {
					if e.Timestamp.After(latestTime) {
						latestTime = e.Timestamp
					}
				}
			}
			
			res := freshnessCheckResult{
				id: "doctor.evidence.learnings",
			}
			if latestTime.IsZero() {
				res.hasWarn = true
				res.severity = "warning"
				res.status = "warn"
				res.detail = "learnings evidence has never been recorded; run 'auto learn record' to update"
			} else {
				age := time.Since(latestTime)
				res.ageDays = age.Hours() / 24.0
				if age > evidenceFreshnessMaxAge {
					res.hasWarn = true
					res.severity = "warning"
					res.status = "warn"
					res.detail = fmt.Sprintf("learnings evidence is %.1f day(s) old (exceeds 30d); run 'auto learn record' to update", res.ageDays)
				} else {
					res.severity = "info"
					res.status = "pass"
					res.detail = fmt.Sprintf("learnings evidence is fresh (%.1f day(s) old)", res.ageDays)
				}
			}
			res.present = true
			results = append(results, res)
		}
	}

	// 2. canary
	canaryMdPath := filepath.Join(dir, ".autopus", "project", "canary.md")
	if _, err := os.Stat(canaryMdPath); err == nil {
		latestJsonPath := filepath.Join(dir, ".autopus", "canary", "latest.json")
		res := freshnessCheckResult{
			id: "doctor.evidence.canary",
		}
		if _, err := os.Stat(latestJsonPath); err == nil {
			data, err := os.ReadFile(latestJsonPath)
			var canaryData struct {
				Timestamp string `json:"timestamp"`
			}
			if err == nil && json.Unmarshal(data, &canaryData) == nil && canaryData.Timestamp != "" {
				t, err := time.Parse(time.RFC3339, canaryData.Timestamp)
				if err == nil {
					age := time.Since(t)
					res.ageDays = age.Hours() / 24.0
					if age > evidenceFreshnessMaxAge {
						res.hasWarn = true
						res.severity = "warning"
						res.status = "warn"
						res.detail = fmt.Sprintf("canary evidence is %.1f day(s) old (exceeds 30d); run 'auto canary' to update", res.ageDays)
					} else {
						res.severity = "info"
						res.status = "pass"
						res.detail = fmt.Sprintf("canary evidence is fresh (%.1f day(s) old)", res.ageDays)
					}
				} else {
					res.hasWarn = true
					res.severity = "warning"
					res.status = "warn"
					res.detail = "canary evidence receipt has invalid timestamp; run 'auto canary' to update"
				}
			} else {
				res.hasWarn = true
				res.severity = "warning"
				res.status = "warn"
				res.detail = "canary evidence receipt is unreadable; run 'auto canary' to update"
			}
		} else {
			res.hasWarn = true
			res.severity = "warning"
			res.status = "warn"
			res.detail = "canary evidence has never been run; run 'auto canary' to update"
		}
		res.present = true
		results = append(results, res)
	}

	// 3. memindex
	indexPath := memindex.DefaultIndexPath(dir)
	if fi, err := os.Stat(indexPath); err == nil {
		res := freshnessCheckResult{
			id:      "doctor.evidence.memindex",
			present: true,
		}
		mtime := fi.ModTime()
		age := time.Since(mtime)
		res.ageDays = age.Hours() / 24.0
		if age > evidenceFreshnessMaxAge {
			res.hasWarn = true
			res.severity = "warning"
			res.status = "warn"
			res.detail = fmt.Sprintf("memindex evidence is %.1f day(s) old (exceeds 30d); run 'auto mem rebuild' to update", res.ageDays)
		} else {
			res.severity = "info"
			res.status = "pass"
			res.detail = fmt.Sprintf("memindex evidence is fresh (%.1f day(s) old)", res.ageDays)
		}
		results = append(results, res)
	}

	return results, nil
}

func renderEvidenceFreshnessText(out io.Writer, dir string, cfg *config.HarnessConfig) {
	results, err := checkFreshness(dir)
	if err != nil || len(results) == 0 {
		return
	}

	tui.SectionHeader(out, "Evidence Freshness")
	for _, res := range results {
		if res.hasWarn {
			tui.Warn(out, fmt.Sprintf("%s: %s", getLoopName(res.id), res.detail))
		} else {
			tui.OK(out, fmt.Sprintf("%s: %s", getLoopName(res.id), res.detail))
		}
	}
}

func getLoopName(id string) string {
	switch id {
	case "doctor.evidence.learnings":
		return "learnings"
	case "doctor.evidence.canary":
		return "canary"
	case "doctor.evidence.memindex":
		return "memindex"
	default:
		return "evidence"
	}
}

func (r *doctorJSONReport) collectEvidenceFreshnessChecks(dir string, cfg *config.HarnessConfig) {
	results, err := checkFreshness(dir)
	if err != nil {
		return
	}

	for _, res := range results {
		r.checks = append(r.checks, jsonCheck{
			ID:       res.id,
			Severity: res.severity,
			Status:   res.status,
			Detail:   res.detail,
		})
	}
}
