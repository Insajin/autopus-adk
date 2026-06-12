package orchestra

import "strings"

// debate_sentinel.go generates per-round random sentinels that fence untrusted
// participant output in the Round 2 cross-pollination and judge prompts (REQ-004,
// INV-004). Fencing prevents a participant from injecting a forged "## ..." header
// or role instruction that the next round or the judge would parse as a real
// directive: everything between <sentinel>-BEGIN and <sentinel>-END is data.

const debateSentinelBase = "AUTOPUS_PART_"

// newDebateSentinel returns a sentinel string that is guaranteed not to appear as
// a substring in any of the given participant outputs. On the (vanishingly rare)
// event of a collision the suffix is extended with more random hex until the
// sentinel is absent, which always terminates.
func newDebateSentinel(outputs ...string) string {
	candidate := debateSentinelBase + randomHex()
	for sentinelCollides(candidate, outputs) {
		candidate += randomHex()
	}
	return candidate
}

// sentinelCollides reports whether sentinel appears as a substring in any output.
func sentinelCollides(sentinel string, outputs []string) bool {
	for _, o := range outputs {
		if strings.Contains(o, sentinel) {
			return true
		}
	}
	return false
}

// sentinelForPreviousResults builds a sentinel absent from all Round 2 participant
// outputs.
func sentinelForPreviousResults(results []PreviousResult) string {
	outs := make([]string, len(results))
	for i, r := range results {
		outs[i] = r.Output
	}
	return newDebateSentinel(outs...)
}

// sentinelForJudgeResults builds a sentinel absent from every participant's Round 1
// and Round 2 output supplied to the judge.
func sentinelForJudgeResults(results []JudgeResult) string {
	outs := make([]string, 0, len(results)*2)
	for _, r := range results {
		outs = append(outs, r.Round1, r.Round2)
	}
	return newDebateSentinel(outs...)
}
