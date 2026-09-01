package journey

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// provider_app_id is the only field that addresses the app under observation.
// There is no compiled-in fallback, so a pack that omits it must fail by name
// rather than silently observe whatever the harness used to hardcode.
func TestDesktopObservationProviderAppID_DeclaredIdentifierValidates(t *testing.T) {
	t.Parallel()

	pack := validDesktopObservationPack()
	require.NoError(t, Validate(pack, t.TempDir()))
	assert.Equal(t, "co.autopus.desktop", pack.DesktopObservation.ProviderAppID)
}

// The lane is a MUST lane in prelaunch and release-candidate. An identifier we
// do not own must validate exactly as ours does, or the gate stays unreachable
// for every adopting project.
func TestDesktopObservationProviderAppID_ThirdPartyIdentifierValidates(t *testing.T) {
	t.Parallel()

	identifiers := []string{
		"com.apple.finder",
		"com.microsoft.VSCode",
		"org.mozilla.firefox",
		"finder",
		"App_2024",
		"a",
		strings.Repeat("a", 128),
	}
	for _, identifier := range identifiers {
		identifier := identifier
		t.Run(identifier, func(t *testing.T) {
			t.Parallel()
			pack := validDesktopObservationPack()
			pack.DesktopObservation.ProviderAppID = identifier
			require.NoError(t, Validate(pack, t.TempDir()))
		})
	}
}

func TestDesktopObservationProviderAppID_MissingFieldNamesItself(t *testing.T) {
	t.Parallel()

	pack := validDesktopObservationPack()
	pack.DesktopObservation.ProviderAppID = ""

	err := Validate(pack, t.TempDir())
	require.Error(t, err)
	var validationErr *ValidationError
	require.True(t, errors.As(err, &validationErr))
	assert.Equal(t, "qa_journey_desktop_observation_policy_invalid", validationErr.Code)
	assert.Contains(t, validationErr.Message, "provider_app_id")
	assert.Contains(t, validationErr.Message, "required")
}

// The value is interpolated into a single argv element handed to an external
// provider CLI, so anything that could turn into a second argument, a flag, a
// path, or an unrepresentable byte is refused before a request is ever built.
func TestDesktopObservationProviderAppID_UnsafeIdentifierFailsClosed(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		value string
	}{
		{name: "empty", value: ""},
		{name: "whitespace only", value: "   "},
		{name: "tab only", value: "\t"},
		{name: "embedded space", value: "co.autopus desktop"},
		{name: "leading space", value: " co.autopus.desktop"},
		{name: "trailing space", value: "co.autopus.desktop "},
		{name: "double quote", value: `co.autopus."desktop"`},
		{name: "single quote", value: "co.autopus.'desktop'"},
		{name: "semicolon", value: "co.autopus.desktop;id"},
		{name: "backtick", value: "co.autopus.`id`"},
		{name: "command substitution", value: "co.autopus.$(id)"},
		{name: "dollar", value: "$HOME"},
		{name: "ampersand", value: "co.autopus.desktop&"},
		{name: "pipe", value: "co.autopus.desktop|id"},
		{name: "forward slash", value: "../../etc/passwd"},
		{name: "backslash", value: `co\autopus\desktop`},
		{name: "nul", value: "co.autopus\x00.desktop"},
		{name: "newline", value: "co.autopus\ndesktop"},
		{name: "carriage return", value: "co.autopus\rdesktop"},
		{name: "leading hyphen reads as a flag", value: "-force"},
		{name: "leading dot reads as a path", value: ".autopus"},
		{name: "leading underscore", value: "_autopus"},
		{name: "non ascii", value: "코.오토퍼스.데스크탑"},
		{name: "over long", value: strings.Repeat("a", 129)},
	}

	for _, test := range tests {
		test := test
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			pack := validDesktopObservationPack()
			pack.DesktopObservation.ProviderAppID = test.value

			err := Validate(pack, t.TempDir())
			require.Error(t, err)
			var validationErr *ValidationError
			require.True(t, errors.As(err, &validationErr))
			assert.Equal(t, "qa_journey_desktop_observation_policy_invalid", validationErr.Code)
			assert.Contains(t, validationErr.Message, "provider_app_id")
		})
	}
}

// The strict-decode gate must not have widened: adding a known key still leaves
// every unknown key rejected, and a pack file that omits provider_app_id is
// caught by the validator rather than accepted with a zero value.
func TestDesktopObservationProviderAppID_YAMLRoundTripsAndStaysStrict(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "desktop-observe.yaml")
	raw := strings.Replace(validDesktopObservationYAML(), "__EXTRA__", "", 1)
	require.NoError(t, os.WriteFile(path, []byte(raw), 0o600))

	pack, err := LoadFile(path)
	require.NoError(t, err)
	require.NoError(t, Validate(pack, dir))
	assert.Equal(t, "co.autopus.desktop", pack.DesktopObservation.ProviderAppID)

	omitted := strings.Replace(raw, "  provider_app_id: co.autopus.desktop\n", "", 1)
	require.NotEqual(t, raw, omitted)
	omittedPath := filepath.Join(dir, "omitted.yaml")
	require.NoError(t, os.WriteFile(omittedPath, []byte(omitted), 0o600))
	omittedPack, err := LoadFile(omittedPath)
	require.NoError(t, err)
	require.ErrorContains(t, Validate(omittedPack, dir), "provider_app_id")

	unknown := strings.Replace(raw, "  provider_app_id: co.autopus.desktop\n",
		"  provider_app_id: co.autopus.desktop\n  provider_app_ids: [co.autopus.desktop]\n", 1)
	unknownPath := filepath.Join(dir, "unknown.yaml")
	require.NoError(t, os.WriteFile(unknownPath, []byte(unknown), 0o600))
	_, err = LoadFile(unknownPath)
	require.Error(t, err)
}

// REQ-3 makes provider_app_id request-only. Nothing publishes a Pack today, but
// serializing one is the cheapest standing check that app_ref and window_ref
// remain the aliases callers reach for.
func TestDesktopObservationProviderAppID_RefsRemainThePublishableAliases(t *testing.T) {
	t.Parallel()

	body, err := yaml.Marshal(validDesktopObservationPack().DesktopObservation)
	require.NoError(t, err)
	assert.Contains(t, string(body), "app_ref: autopus-desktop")
	assert.Contains(t, string(body), "window_ref: main-window")
}
