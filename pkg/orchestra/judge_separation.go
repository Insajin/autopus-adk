package orchestra

import (
	"fmt"
	"strings"
)

// JudgeSeparationEvidence records whether a required judge is independent from
// every participant model family.
type JudgeSeparationEvidence struct {
	Required            bool     `json:"required"`
	ParticipantFamilies []string `json:"participant_families"`
	JudgeProvider       string   `json:"judge_provider"`
	JudgeModelFamily    string   `json:"judge_model_family"`
	Separated           bool     `json:"separated"`
	Reason              string   `json:"reason,omitempty"`
}

func evaluateJudgeFamilySeparation(
	providers []ProviderConfig,
	judge ProviderConfig,
	required bool,
) *JudgeSeparationEvidence {
	evidence := &JudgeSeparationEvidence{
		Required: required, JudgeProvider: judge.Name,
		JudgeModelFamily: normalizeModelFamily(judge.ModelFamily),
	}
	unknownParticipant := false
	for _, provider := range providers {
		family := normalizeModelFamily(provider.ModelFamily)
		if family == "" {
			unknownParticipant = true
			continue
		}
		evidence.ParticipantFamilies = appendUniqueName(evidence.ParticipantFamilies, family)
	}

	switch {
	case !required:
		evidence.Separated = evidence.JudgeModelFamily != "" && !unknownParticipant &&
			!containsProviderName(evidence.ParticipantFamilies, evidence.JudgeModelFamily)
		evidence.Reason = "policy_not_required"
	case evidence.JudgeModelFamily == "":
		evidence.Reason = "unknown_judge_model_family"
	case unknownParticipant:
		evidence.Reason = "unknown_participant_model_family"
	case containsProviderName(evidence.ParticipantFamilies, evidence.JudgeModelFamily):
		evidence.Reason = "same_model_family"
	default:
		evidence.Separated = true
		evidence.Reason = "different_model_family"
	}
	return evidence
}

func normalizeModelFamily(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func judgeSeparationError(evidence *JudgeSeparationEvidence) error {
	if evidence == nil || !evidence.Required || evidence.Separated {
		return nil
	}
	return fmt.Errorf("orchestra: required judge model-family separation failed: %s", evidence.Reason)
}
