package scaffold

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

func desktopReadmeYAMLBlock(t *testing.T, body string) string {
	t.Helper()
	fences := regexp.MustCompile("(?s)```yaml\n(.*?)```").FindAllStringSubmatch(body, -1)
	for _, match := range fences {
		if strings.Contains(match[1], "desktop_observation:") {
			return match[1]
		}
	}
	t.Fatalf("no desktop_observation yaml block in the README")
	return ""
}

// The README example must be installable as written. A documented pack that does
// not validate is worse than no documentation: it sends the reader to debug the
// harness instead of filling in their own values.
func TestDesktopObservationReadme_ExampleValidatesAsAPack(t *testing.T) {
	t.Parallel()

	body := captureReadmeBody(projectSignals{HasDesktopGUI: true})
	block := desktopReadmeYAMLBlock(t, body)

	var pack journey.Pack
	require.NoError(t, yaml.Unmarshal([]byte(block), &pack))
	require.NoError(t, journey.Validate(pack, t.TempDir()))

	assert.Equal(t, "desktop-accessibility-observe", pack.Adapter.ID)
	assert.Equal(t, []string{"desktop-native"}, pack.Lanes)
	assert.NotEmpty(t, pack.DesktopObservation.ProviderAppID)
	assert.Len(t, pack.DesktopObservation.RequiredLandmarks, 3,
		"the example must show a deeper landmark, which is the point of selective publication")
}

// A project with no desktop signal must not be handed desktop instructions; the
// README is already long and the section would be noise.
func TestDesktopObservationReadme_OnlyForDesktopProjects(t *testing.T) {
	t.Parallel()

	assert.NotContains(t, captureReadmeBody(projectSignals{}),
		"Desktop accessibility observation")
	assert.Contains(t, captureReadmeBody(projectSignals{HasTauriRust: true}),
		"Desktop accessibility observation")
}

// The example is the only place a project learns this schema, so it must state
// the two facts that are not guessable: which field is request-only, and that
// only declared landmarks are published.
func TestDesktopObservationReadme_StatesTheNonObviousContract(t *testing.T) {
	t.Parallel()

	body := captureReadmeBody(projectSignals{HasDesktopGUI: true})
	for _, token := range []string{
		"provider_app_id",
		"Request-only",
		"only",
		"declared landmarks are published",
		"read-only",
		"declared_landmark_not_found",
		"orca computer list-windows",
		"--runtime-provider orca",
	} {
		assert.Contains(t, body, token)
	}
}
