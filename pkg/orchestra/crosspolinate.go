package orchestra

import (
	"fmt"
)

// ProviderResult holds a single provider's output from one round.
type ProviderResult struct {
	Provider string // real provider name
	Output   string // raw output text
}

// CrossPollinateBuilder anonymizes provider outputs and builds
// cross-pollination prompts for the next debate round.
type CrossPollinateBuilder struct {
	identityMap map[string]string // alias -> real name
	reverseMap  map[string]string // real name -> alias
}

// NewCrossPollinateBuilder creates a builder with the given provider names.
// Aliases are assigned in order: "Debater A", "Debater B", "Debater C", etc.
func NewCrossPollinateBuilder(providerNames []string) *CrossPollinateBuilder {
	im := make(map[string]string, len(providerNames))
	rm := make(map[string]string, len(providerNames))
	for i, name := range providerNames {
		alias := fmt.Sprintf("Debater %c", 'A'+rune(i))
		im[alias] = name
		rm[name] = alias
	}
	return &CrossPollinateBuilder{identityMap: im, reverseMap: rm}
}

// Anonymize converts provider results to anonymized results.
// ICE scores are stripped and outputs are capped to the Round 2 prompt budget.
func (cpb *CrossPollinateBuilder) Anonymize(results []ProviderResult) []PreviousResult {
	cleanedOutputs := make([]string, len(results))
	for i, r := range results {
		cleanedOutputs[i] = stripICEScores(r.Output)
	}
	cappedOutputs := capPromptSections(cleanedOutputs, rebuttalPromptTotalTokens, rebuttalPromptPerParticipant)

	out := make([]PreviousResult, 0, len(results))
	for i, r := range results {
		alias, ok := cpb.reverseMap[r.Provider]
		if !ok {
			alias = r.Provider // fallback: use original name
		}
		out = append(out, PreviousResult{
			Alias:  alias,
			Output: cappedOutputs[i],
		})
	}
	return out
}

// AnonymizeForJudge converts multi-round results to judge-ready format while
// bounding per-participant context for the final judge prompt.
func (cpb *CrossPollinateBuilder) AnonymizeForJudge(round1, round2 []ProviderResult) []JudgeResult {
	r1Map := make(map[string]string, len(round1))
	for _, r := range round1 {
		r1Map[r.Provider] = stripICEScores(r.Output)
	}
	r2Map := make(map[string]string, len(round2))
	for _, r := range round2 {
		r2Map[r.Provider] = stripICEScores(r.Output)
	}

	out := make([]JudgeResult, 0, len(cpb.identityMap))
	for alias, realName := range cpb.identityMap {
		out = append(out, JudgeResult{
			Alias:  alias,
			Round1: r1Map[realName],
			Round2: r2Map[realName],
		})
	}
	return capJudgeResults(out)
}

// IdentityMap returns alias -> real provider name mapping for de-anonymization.
func (cpb *CrossPollinateBuilder) IdentityMap() map[string]string {
	cp := make(map[string]string, len(cpb.identityMap))
	for k, v := range cpb.identityMap {
		cp[k] = v
	}
	return cp
}

// stripICEScores is defined in interactive_detect.go and reused here.
// It removes self-assigned ICE scoring sections to prevent confidence cascade.
