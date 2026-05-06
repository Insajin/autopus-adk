package skillevolve

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPromoteCandidate_PreflightsAllTargetsBeforeWriting(t *testing.T) {
	t.Parallel()

	projectDir := t.TempDir()
	firstRel := "autopus-adk/content/skills/testing-strategy.md"
	blockedRel := "autopus-adk/content/skills/blocked.md"
	oldFirst := validSkillContent("testing-strategy") + "old body\n"
	newFirst := validSkillContent("testing-strategy") + "new body\n"
	writeWorkspaceFile(t, projectDir, firstRel, oldFirst)
	require.NoError(t, os.MkdirAll(filepath.Join(projectDir, filepath.FromSlash(blockedRel)), 0o755))

	candidate := promotionReadyCandidate(newFirst)
	candidate.ProposedFiles = append(candidate.ProposedFiles, ProposedFile{
		Path:    blockedRel,
		Content: validSkillContent("blocked") + "blocked body\n",
	})
	candidate.Provenance.AffectedSourceOfTruths = append(candidate.Provenance.AffectedSourceOfTruths, blockedRel)

	result, err := PromoteCandidate(context.Background(), PromotionOptions{
		ProjectDir: projectDir,
		Candidate:  candidate,
		Approval:   HumanApproval{ApprovedBy: "human-reviewer"},
		Apply:      true,
	})

	require.Error(t, err)
	assert.False(t, result.Applied)
	assert.Equal(t, oldFirst, readWorkspaceFiles(t, projectDir, []string{firstRel})[firstRel])
}
