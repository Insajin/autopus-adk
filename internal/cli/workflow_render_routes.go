package cli

import "fmt"

// routeEmbed bundles the embedded source paths and display paths for a single
// workflow route so render can be parameterized by --route (T18, REQ-012).
type routeEmbed struct {
	schemaEmbed     string
	contractEmbed   string
	jsEmbed         string
	contractDisplay string
	schemaDisplay   string
}

// workflowRouteEmbeds maps a route name to its embedded + display paths. Route A
// is the always-on opt-in route; route_team is the deterministic team route.
var workflowRouteEmbeds = map[string]routeEmbed{
	"route_a": {
		schemaEmbed:     "workflows/route_a.schema.json",
		contractEmbed:   "workflows/route_a.md",
		jsEmbed:         "claude/workflows/route_a.workflow.js.tmpl",
		contractDisplay: "content/workflows/route_a.md",
		schemaDisplay:   "content/workflows/route_a.schema.json",
	},
	"route_team": {
		schemaEmbed:     "workflows/route_team.schema.json",
		contractEmbed:   "workflows/route_team.md",
		jsEmbed:         "claude/workflows/route_team.workflow.js.tmpl",
		contractDisplay: "content/workflows/route_team.md",
		schemaDisplay:   "content/workflows/route_team.schema.json",
	},
}

// normalizeRouteName maps the user-facing --route value to the canonical route
// key. "team" and "a" are accepted shorthands for the full names.
func normalizeRouteName(route string) string {
	switch route {
	case "team", "route_team":
		return "route_team"
	case "a", "route_a", "":
		return "route_a"
	default:
		return route
	}
}

// selectRouteEmbed resolves a (possibly shorthand) route name to its routeEmbed,
// failing closed on an unknown route.
func selectRouteEmbed(route string) (routeEmbed, string, error) {
	key := normalizeRouteName(route)
	re, ok := workflowRouteEmbeds[key]
	if !ok {
		return routeEmbed{}, "", fmt.Errorf("unknown workflow route %q (want route_a or route_team)", route)
	}
	return re, key, nil
}
