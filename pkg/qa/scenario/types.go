// Package scenario compiles project-authored user scenarios into runner specs.
//
// The harness never invents assertions. A Scenario is the project's own
// declaration of what a user sees; this package only translates it into the
// runner dialect so the declaration becomes executable evidence.
package scenario

// SchemaVersion is the only accepted schema_version value. Unknown keys and
// unknown versions are rejected rather than ignored: a typo that silently
// dropped a screen would turn a scenario into a passing no-op.
const SchemaVersion = "qamesh.scenario.v1"

// ScreenAnnotation is the Playwright annotation type the generated spec pushes
// so the capture producer can label each step with the screen it covers. The
// gui.screen_matrix oracle keys on that label.
const ScreenAnnotation = "autopus-screen"

// ExploreTag marks generated specs as read-only exploration. The gui-explore
// Journey Pack selects specs with `--grep @explore`, so the tag is what makes a
// compiled scenario reachable from the lane.
const ExploreTag = "@explore"

// Scenario is one user-facing journey through a GUI surface.
type Scenario struct {
	SchemaVersion string   `yaml:"schema_version"`
	ID            string   `yaml:"id"`
	Title         string   `yaml:"title"`
	Journey       string   `yaml:"journey"`
	Origin        string   `yaml:"origin,omitempty"`
	AcceptanceRef []string `yaml:"acceptance_refs,omitempty"`
	Screens       []Screen `yaml:"screens"`
	// Path is the file the scenario was loaded from. Set by the loader so
	// generated specs and error messages can cite their source.
	Path string `yaml:"-"`
}

// Screen is one addressable view plus the assertions that must hold on it. Each
// screen compiles to exactly one runner test, which is what lets the screen
// matrix oracle count coverage per screen.
type Screen struct {
	ID    string `yaml:"id"`
	Path  string `yaml:"path"`
	Steps []Step `yaml:"steps"`
}

// Step is a single assertion. Exactly one field may be set.
//
// The vocabulary is deliberately closed and contains no navigation-mutating or
// input action: click, fill, press, and their siblings are not representable.
// That is a safety property, not an omission - a compiled scenario cannot trip
// the gui-explore mutation guard, because the compiler has no way to emit one.
type Step struct {
	ExpectTitle string       `yaml:"expect_title,omitempty"`
	ExpectURL   string       `yaml:"expect_url,omitempty"`
	ExpectText  string       `yaml:"expect_text,omitempty"`
	ExpectRole  *RoleTarget  `yaml:"expect_role,omitempty"`
	ExpectCount *CountTarget `yaml:"expect_count,omitempty"`
}

// RoleTarget addresses an element through the accessibility tree. Only
// role-based addressing exists here: the generated pack declares
// `selector_strategy: role-first`, and a CSS escape hatch would let compiled
// specs contradict the strategy the pack advertises.
type RoleTarget struct {
	Role  string `yaml:"role"`
	Name  string `yaml:"name,omitempty"`
	Exact bool   `yaml:"exact,omitempty"`
}

// CountTarget asserts how many elements of a role exist, which is how a
// scenario states "the catalog lists three products" without naming each one.
type CountTarget struct {
	Role  string `yaml:"role"`
	Name  string `yaml:"name,omitempty"`
	Count int    `yaml:"count"`
}

// Compiled is one emitted spec file.
type Compiled struct {
	ScenarioID string `json:"scenario_id"`
	SourcePath string `json:"source_path"`
	SpecPath   string `json:"spec_path"`
	Screens    int    `json:"screens"`
	Steps      int    `json:"steps"`
	Bytes      int    `json:"bytes"`
}
