package orchestra

import "fmt"

// ReviewJudgeParticipant is an anonymized reviewer response supplied to the final judge.
type ReviewJudgeParticipant struct {
	Alias  string
	Output string
}

// ReviewJudgeData contains the complete input for a typed SPEC review judgment.
type ReviewJudgeData struct {
	SpecID        string
	Mode          string
	Participants  []ReviewJudgeParticipant
	PriorFindings string
	Checklist     string
	SchemaJSON    string
}

// AnonymizeReviewParticipants replaces only participant headers with stable input-order aliases.
// Reviewer bodies are passed verbatim; the judge prompt must ignore self-identification inside them.
func AnonymizeReviewParticipants(responses []ProviderResponse) ([]ReviewJudgeParticipant, map[string]string) {
	participants := make([]ReviewJudgeParticipant, 0, len(responses))
	identities := make(map[string]string, len(responses))
	for i, response := range responses {
		alias := fmt.Sprintf("Reviewer %c", 'A'+rune(i))
		participants = append(participants, ReviewJudgeParticipant{
			Alias:  alias,
			Output: response.Output,
		})
		identities[alias] = response.Provider
	}
	return participants, identities
}
