package scenario

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func validScenario() Scenario {
	return Scenario{
		SchemaVersion: SchemaVersion,
		ID:            "first-visit",
		Title:         "A visitor lands",
		Journey:       "browser-gui-explore",
		Origin:        "http://127.0.0.1:4173",
		Path:          "first-visit.yaml",
		Screens: []Screen{{
			ID:   "landing",
			Path: "/",
			Steps: []Step{
				{ExpectTitle: "Shop"},
				{ExpectRole: &RoleTarget{Role: "heading", Name: "Catalog", Exact: true}},
				{ExpectCount: &CountTarget{Role: "listitem", Count: 3}},
				{ExpectText: "Total: 42.00"},
				{ExpectURL: "/"},
			},
		}},
	}
}

func compileValid(t *testing.T, s Scenario) string {
	t.Helper()
	body, err := Compile(s, Options{FixtureImport: "../../fixture.cjs"})
	require.NoError(t, err)
	return string(body)
}

// The tag is the only thing that makes a compiled scenario reachable from the
// gui-explore lane, whose command greps for it.
func TestCompileTagsSpecForExploreGrep(t *testing.T) {
	t.Parallel()
	assert.Contains(t, compileValid(t, validScenario()), `test.describe("first-visit `+ExploreTag+`"`)
}

// The annotation is the only link between a declared screen and the capture
// index step the screen_matrix oracle counts.
func TestCompileEmitsScreenAnnotation(t *testing.T) {
	t.Parallel()
	out := compileValid(t, validScenario())
	assert.Contains(t, out, `annotations.push({ type: "`+ScreenAnnotation+`", description: "landing" })`)
}

func TestCompileRendersEveryStepKind(t *testing.T) {
	t.Parallel()
	out := compileValid(t, validScenario())
	for _, want := range []string{
		`await expect(page).toHaveTitle(new RegExp("Shop"));`,
		`await expect(page.getByRole("heading", { name: "Catalog", exact: true }).first()).toBeVisible();`,
		`await expect(page.getByRole("listitem")).toHaveCount(3);`,
		`await expect(page.getByText(new RegExp("Total: 42\\.00")).first()).toBeVisible();`,
		`await expect(page).toHaveURL(ORIGIN + "/");`,
		`await page.goto(ORIGIN + "/");`,
	} {
		assert.Contains(t, out, want)
	}
}

// The closed vocabulary is a safety property: no scenario can compile into a
// forbidden action, so the guard can never be tripped by generated code.
func TestCompileNeverEmitsMutatingAPI(t *testing.T) {
	t.Parallel()
	out := compileValid(t, validScenario())
	for _, forbidden := range []string{
		".click(", ".fill(", ".press(", ".check(", ".uncheck(",
		".selectOption(", ".setInputFiles(", ".dragTo(", ".tap(", ".dblclick(",
	} {
		assert.NotContains(t, out, forbidden, "generated spec must stay read-only")
	}
}

// Scenario text is authored data interpolated into executable source. Anything
// that can close the literal or the comment must be neutralized.
func TestCompileEscapesHostileAuthoredText(t *testing.T) {
	t.Parallel()
	s := validScenario()
	s.Title = "break */ out\nsecond line"
	s.Screens[0].Steps = []Step{
		{ExpectText: "a\"b\\c`d${e}\n</script>"},
		{ExpectRole: &RoleTarget{Role: "heading", Name: `") ; process.exit(1); //`}},
	}
	out := compileValid(t, s)
	lines := strings.Split(out, "\n")

	// A newline in authored prose must not create a second line, and no line may
	// close the block comment early. Either would move authored text into
	// executable position.
	scenarioLines := 0
	for _, line := range lines {
		if strings.HasPrefix(line, "// Scenario:") {
			scenarioLines++
		}
		assert.NotContains(t, line, "*/")
		assert.NotContains(t, line, "\r")
	}
	assert.Equal(t, 1, scenarioLines, "a multi-line title must collapse to one comment line")

	// The load-bearing invariant: every generated line closes every string
	// literal it opens. Hostile text can therefore appear as data - it just
	// cannot terminate the literal it lives in.
	for index, line := range lines {
		assert.Zero(t, unescapedQuotes(line)%2, "line %d has an unterminated literal: %s", index+1, line)
	}
}

// unescapedQuotes counts double quotes that actually delimit a literal, ignoring
// any preceded by a backslash escape.
func unescapedQuotes(line string) int {
	count := 0
	for i := 0; i < len(line); i++ {
		if line[i] == '\\' {
			i++
			continue
		}
		if line[i] == '"' {
			count++
		}
	}
	return count
}

func TestCompileIsDeterministic(t *testing.T) {
	t.Parallel()
	first := compileValid(t, validScenario())
	for range 5 {
		assert.Equal(t, first, compileValid(t, validScenario()))
	}
}

func TestCompileInheritsOriginFromOptions(t *testing.T) {
	t.Parallel()
	s := validScenario()
	s.Origin = ""
	body, err := Compile(s, Options{Origin: "https://staging.example/", FixtureImport: "./f.cjs"})
	require.NoError(t, err)
	assert.Contains(t, string(body), `const ORIGIN = "https://staging.example";`)
}

// Without an origin the spec would navigate nowhere the guard allows, so the
// compiler refuses rather than emitting a relative goto that silently depends
// on a baseURL the project may not set.
func TestCompileRequiresOrigin(t *testing.T) {
	t.Parallel()
	s := validScenario()
	s.Origin = ""
	_, err := Compile(s, Options{FixtureImport: "./f.cjs"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "no origin")
}

func TestCompileRequiresFixtureImport(t *testing.T) {
	t.Parallel()
	_, err := Compile(validScenario(), Options{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "capture fixture")
}

func TestCompileRejectsInvalidScenario(t *testing.T) {
	t.Parallel()
	s := validScenario()
	s.Screens = nil
	_, err := Compile(s, Options{Origin: "http://x.test", FixtureImport: "./f.cjs"})
	require.Error(t, err)
}

func TestScreenMatrixProjectsDeclaredScreens(t *testing.T) {
	t.Parallel()
	s := validScenario()
	rows := ScreenMatrix([]Scenario{s}, "browser-gui-explore")
	require.Len(t, rows, 1)
	assert.Equal(t, "landing", rows[0]["id"])
	assert.Equal(t, "/", rows[0]["path"])
	assert.Equal(t, []string{"goto"}, rows[0]["required_actions"])
	assert.Empty(t, ScreenMatrix([]Scenario{s}, "other-journey"))
}
