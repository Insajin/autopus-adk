//go:build integration

package issue_test

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/issue"
)

func TestAddComment_Success(t *testing.T) {
	t.Parallel()

	runner := &mockRunner{
		runFn: func(name string, args ...string) ([]byte, error) {
			return []byte(""), nil
		},
	}
	submitter := issue.NewSubmitter(runner)

	err := submitter.AddComment("org/repo", 42, "comment body")
	require.NoError(t, err)
}

func TestAddComment_Error(t *testing.T) {
	t.Parallel()

	runner := &mockRunner{
		runFn: func(name string, args ...string) ([]byte, error) {
			return nil, errors.New("gh: API error 404")
		},
	}
	submitter := issue.NewSubmitter(runner)

	err := submitter.AddComment("org/repo", 99, "body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "99")
}

func TestSubmit_NoDuplicate(t *testing.T) {
	t.Parallel()

	runner := &mockRunner{
		lookPathFn: func(name string) (string, error) {
			return "/usr/bin/gh", nil
		},
		runFn: func(name string, args ...string) ([]byte, error) {
			for _, a := range args {
				switch a {
				case "status":
					return []byte("Logged in"), nil
				case "list":
					return []byte("[]"), nil
				case "create":
					return []byte("https://github.com/org/repo/issues/1"), nil
				}
			}
			return []byte("https://github.com/org/repo/issues/1"), nil
		},
	}
	submitter := issue.NewSubmitter(runner)

	report := issue.IssueReport{
		Title:  "Test issue",
		Hash:   "abc123",
		Labels: []string{"auto-report"},
		Repo:   "org/repo",
	}
	result, err := submitter.Submit(report, "issue body")
	require.NoError(t, err)
	assert.False(t, result.WasDuplicate)
}

func TestSubmit_WithDuplicate(t *testing.T) {
	t.Parallel()

	type ghIssue struct {
		Number int    `json:"number"`
		URL    string `json:"url"`
	}
	dupPayload, _ := json.Marshal([]ghIssue{{Number: 55, URL: "https://github.com/org/repo/issues/55"}})

	runner := &mockRunner{
		lookPathFn: func(name string) (string, error) {
			return "/usr/bin/gh", nil
		},
		runFn: func(name string, args ...string) ([]byte, error) {
			for _, a := range args {
				switch a {
				case "status":
					return []byte("Logged in"), nil
				case "list":
					return dupPayload, nil
				case "comment":
					return []byte(""), nil
				}
			}
			return []byte(""), nil
		},
	}
	submitter := issue.NewSubmitter(runner)

	report := issue.IssueReport{
		Title:  "Duplicate issue",
		Hash:   "deadbeef",
		Labels: []string{"auto-report"},
		Repo:   "org/repo",
	}
	result, err := submitter.Submit(report, "duplicate body")
	require.NoError(t, err)
	assert.True(t, result.WasDuplicate)
	assert.Equal(t, "commented", result.Action)
	assert.Equal(t, "https://github.com/org/repo/issues/55", result.IssueURL)
	assert.Equal(t, 55, result.IssueNumber)
}

func TestSubmit_CheckGHFails(t *testing.T) {
	t.Parallel()

	runner := &mockRunner{
		lookPathFn: func(name string) (string, error) {
			return "", errors.New("not found")
		},
	}
	submitter := issue.NewSubmitter(runner)

	report := issue.IssueReport{Repo: "org/repo", Hash: "abc"}
	_, err := submitter.Submit(report, "body")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "gh CLI")
}
