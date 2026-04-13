package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/worker/adapter"
)

var (
	commitIntentPattern  = regexp.MustCompile(`(?i)\bgit\s+commit\b|\bcommit\b`)
	pushIntentPattern    = regexp.MustCompile(`(?i)\bgit\s+push\b|\bpush\b`)
	branchIntentPattern  = regexp.MustCompile(`(?i)\bbranch\b`)
	refsHeadsPattern     = regexp.MustCompile(`refs/heads/([A-Za-z0-9._/-]+)`)
	branchKeywordPattern = regexp.MustCompile(`(?i)\bbranch(?:\s+(?:named|called))?\s+([A-Za-z0-9._/-]*[/-][A-Za-z0-9._/-]+)\b`)
	originBranchPattern  = regexp.MustCompile(`(?i)\borigin(?:/|\s+)([A-Za-z0-9._/-]*[/-][A-Za-z0-9._/-]+)\b`)
	postconditionTimeout = 5 * time.Second
)

type executionBaseline struct {
	GitRepo bool
	HeadSHA string
	Branch  string
}

type taskPostconditions struct {
	CommitRequired bool
	PushRequired   bool
	BranchRequired bool
	Branches       []string
}

type postconditionCheck struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
}

type postconditionReport struct {
	Active   bool                 `json:"active"`
	Checks   []postconditionCheck `json:"checks,omitempty"`
	Branches []string             `json:"branches,omitempty"`
}

func captureExecutionBaseline(workDir string) executionBaseline {
	headSHA, err := gitOutput(workDir, "rev-parse", "HEAD")
	if err != nil {
		return executionBaseline{}
	}
	branch, _ := gitOutput(workDir, "branch", "--show-current")
	return executionBaseline{
		GitRepo: true,
		HeadSHA: strings.TrimSpace(headSHA),
		Branch:  strings.TrimSpace(branch),
	}
}

func detectTaskPostconditions(prompt string) taskPostconditions {
	reqs := taskPostconditions{
		CommitRequired: commitIntentPattern.MatchString(prompt),
		PushRequired:   pushIntentPattern.MatchString(prompt),
		BranchRequired: branchIntentPattern.MatchString(prompt),
	}

	branches := make(map[string]struct{})
	for _, pattern := range []*regexp.Regexp{refsHeadsPattern, branchKeywordPattern, originBranchPattern} {
		matches := pattern.FindAllStringSubmatch(prompt, -1)
		for _, match := range matches {
			if len(match) < 2 {
				continue
			}
			branch := sanitizeBranchName(match[1])
			if branch == "" {
				continue
			}
			branches[branch] = struct{}{}
		}
	}

	reqs.Branches = make([]string, 0, len(branches))
	for branch := range branches {
		reqs.Branches = append(reqs.Branches, branch)
	}
	sort.Strings(reqs.Branches)
	return reqs
}

func verifyExecutionPostconditions(workDir, prompt string, baseline executionBaseline) (adapter.Artifact, error) {
	reqs := detectTaskPostconditions(prompt)
	if !baseline.GitRepo || (!reqs.CommitRequired && !reqs.PushRequired && !reqs.BranchRequired) {
		return adapter.Artifact{}, nil
	}

	report := postconditionReport{
		Active:   true,
		Branches: append([]string(nil), reqs.Branches...),
	}

	currentHead, headErr := gitOutput(workDir, "rev-parse", "HEAD")
	if reqs.CommitRequired {
		if headErr != nil {
			report.Checks = append(report.Checks, postconditionCheck{
				Name:   "commit",
				Status: "failed",
				Detail: headErr.Error(),
			})
		} else if strings.TrimSpace(currentHead) == baseline.HeadSHA {
			report.Checks = append(report.Checks, postconditionCheck{
				Name:   "commit",
				Status: "failed",
				Detail: "HEAD did not advance after execution",
			})
		} else {
			report.Checks = append(report.Checks, postconditionCheck{
				Name:   "commit",
				Status: "passed",
				Detail: strings.TrimSpace(currentHead),
			})
		}
	}

	if reqs.BranchRequired {
		branches := branchesForVerification(reqs, workDir)
		if len(branches) == 0 {
			report.Checks = append(report.Checks, postconditionCheck{
				Name:   "branch",
				Status: "failed",
				Detail: "unable to determine target branch",
			})
		}
		for _, branch := range branches {
			if err := gitRun(workDir, "show-ref", "--verify", "--quiet", "refs/heads/"+branch); err != nil {
				report.Checks = append(report.Checks, postconditionCheck{
					Name:   "branch",
					Status: "failed",
					Detail: fmt.Sprintf("local branch %q not found", branch),
				})
				continue
			}
			report.Checks = append(report.Checks, postconditionCheck{
				Name:   "branch",
				Status: "passed",
				Detail: branch,
			})
		}
	}

	if reqs.PushRequired {
		branches := branchesForVerification(reqs, workDir)
		if len(branches) == 0 {
			report.Checks = append(report.Checks, postconditionCheck{
				Name:   "push",
				Status: "failed",
				Detail: "unable to determine pushed branch",
			})
		}
		for _, branch := range branches {
			if err := gitRun(workDir, "ls-remote", "--exit-code", "--heads", "origin", branch); err != nil {
				report.Checks = append(report.Checks, postconditionCheck{
					Name:   "push",
					Status: "failed",
					Detail: fmt.Sprintf("remote branch %q not found", branch),
				})
				continue
			}
			report.Checks = append(report.Checks, postconditionCheck{
				Name:   "push",
				Status: "passed",
				Detail: branch,
			})
		}
	}

	artifact := adapter.Artifact{
		Name:     "postconditions.json",
		MimeType: "application/json",
		Data:     mustMarshalPostconditionReport(report),
	}

	var failures []string
	for _, check := range report.Checks {
		if check.Status == "failed" {
			failures = append(failures, fmt.Sprintf("%s: %s", check.Name, check.Detail))
		}
	}
	if len(failures) > 0 {
		return artifact, fmt.Errorf("postcondition failed: %s", strings.Join(failures, "; "))
	}
	return artifact, nil
}

func branchesForVerification(reqs taskPostconditions, workDir string) []string {
	if len(reqs.Branches) > 0 {
		return append([]string(nil), reqs.Branches...)
	}
	currentBranch, err := gitOutput(workDir, "branch", "--show-current")
	if err != nil {
		return nil
	}
	currentBranch = sanitizeBranchName(currentBranch)
	if currentBranch == "" || strings.HasPrefix(currentBranch, "worker-") {
		return nil
	}
	return []string{currentBranch}
}

func sanitizeBranchName(raw string) string {
	branch := strings.TrimSpace(raw)
	branch = strings.Trim(branch, "\"'`.,)")
	if branch == "" {
		return ""
	}
	if strings.Contains(branch, "..") || strings.HasPrefix(branch, "-") {
		return ""
	}
	return branch
}

func gitOutput(workDir string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), postconditionTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = workDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func gitRun(workDir string, args ...string) error {
	_, err := gitOutput(workDir, args...)
	return err
}

func mustMarshalPostconditionReport(report postconditionReport) string {
	data, err := json.Marshal(report)
	if err != nil {
		return `{"active":true,"checks":[{"name":"marshal","status":"failed","detail":"report serialization failed"}]}`
	}
	return string(data)
}
