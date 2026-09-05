package telemetry

import (
	"fmt"
	"time"
)

// phaseGraph is the dependency DAG over a run's phases, indexed by position.
type phaseGraph struct {
	phases []PhaseRecord
	deps   [][]int
	issues []string
	// Longest-path memo: total duration, node count on that path, chosen
	// predecessor (-1 for a root), and visit state (0 new, 1 open, 2 done).
	best  []time.Duration
	count []int
	prev  []int
	state []int8
}

// criticalPath returns the longest wall-clock path through the phase DAG and
// its duration. Parallel siblings are never summed: each phase contributes
// its own span once, along the single chain of dependencies that ends
// latest. Unknown dependency names, self-dependencies, and cycles are
// reported as issues and their edges ignored rather than aborting.
func criticalPath(phases []PhaseRecord) ([]string, time.Duration, []string) {
	if len(phases) == 0 {
		return []string{}, 0, nil
	}
	graph := newPhaseGraph(phases)
	for i := range phases {
		graph.visit(i)
	}
	end := 0
	for i := 1; i < len(phases); i++ {
		if graph.better(i, end) {
			end = i
		}
	}
	names := make([]string, graph.count[end])
	for node, i := end, len(names)-1; node >= 0; node, i = graph.prev[node], i-1 {
		names[i] = phases[node].Name
	}
	return names, graph.best[end], graph.issues
}

func newPhaseGraph(phases []PhaseRecord) *phaseGraph {
	graph := &phaseGraph{
		phases: phases,
		deps:   make([][]int, len(phases)),
		best:   make([]time.Duration, len(phases)),
		count:  make([]int, len(phases)),
		prev:   make([]int, len(phases)),
		state:  make([]int8, len(phases)),
	}
	for i, phase := range phases {
		graph.prev[i] = -1
		for _, name := range phase.DependsOn {
			target := resolvePhaseRef(phases, i, name)
			switch {
			case target < 0:
				graph.issues = append(graph.issues, fmt.Sprintf("phase %q depends on unknown phase %q", phase.Name, name))
			case target == i:
				graph.issues = append(graph.issues, fmt.Sprintf("phase %q depends on itself", phase.Name))
			default:
				graph.deps[i] = append(graph.deps[i], target)
			}
		}
	}
	return graph
}

// visit computes the longest path ending at node i. Ties between dependency
// chains prefer the longer chain, then the dependency listed first, so the
// result is deterministic for zero-duration phases.
func (g *phaseGraph) visit(i int) {
	if g.state[i] != 0 {
		return
	}
	g.state[i] = 1
	choice := -1
	for _, dep := range g.deps[i] {
		if g.state[dep] == 1 {
			g.issues = append(g.issues, fmt.Sprintf("dependency cycle between phases %q and %q ignored", g.phases[i].Name, g.phases[dep].Name))
			continue
		}
		g.visit(dep)
		if choice < 0 || g.better(dep, choice) {
			choice = dep
		}
	}
	g.best[i] = phaseSpan(g.phases[i])
	g.count[i] = 1
	if choice >= 0 {
		g.best[i] += g.best[choice]
		g.count[i] += g.count[choice]
		g.prev[i] = choice
	}
	g.state[i] = 2
}

// better reports whether the finished path ending at a beats the one at b:
// longer duration wins, then more phases, then the earlier-recorded phase.
func (g *phaseGraph) better(a, b int) bool {
	if g.best[a] != g.best[b] {
		return g.best[a] > g.best[b]
	}
	return g.count[a] > g.count[b]
}

// resolvePhaseRef maps a dependency name to a phase index: the nearest
// earlier phase with that name (retries repeat names), else the first later
// one, else -1.
func resolvePhaseRef(phases []PhaseRecord, from int, name string) int {
	for j := from - 1; j >= 0; j-- {
		if phases[j].Name == name {
			return j
		}
	}
	for j := from + 1; j < len(phases); j++ {
		if phases[j].Name == name {
			return j
		}
	}
	return -1
}

// phaseSpan is a phase's own wall-clock span: end minus start when both are
// recorded, otherwise the accumulated Duration.
func phaseSpan(phase PhaseRecord) time.Duration {
	if !phase.StartTime.IsZero() && !phase.EndTime.IsZero() && phase.EndTime.After(phase.StartTime) {
		return phase.EndTime.Sub(phase.StartTime)
	}
	if phase.Duration > 0 {
		return phase.Duration
	}
	return 0
}
