package rulecond_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/rulecond"
)

// stateKeyShape is the REQ-STICKYRULE-FIRE-02 filename contract: a hostile
// session_id cannot reach a path component because the name is a digest by
// construction rather than a validated identifier.
var stateKeyShape = regexp.MustCompile(`^[0-9a-f]{64}$`)

// hostileSessionIDs are the S3 identifiers, including the degenerate ones.
func hostileSessionIDs() []struct {
	name string
	id   string
} {
	return []struct {
		name string
		id   string
	}{
		{name: "ordinary", id: "abc123"},
		{name: "relative traversal", id: "../../etc/passwd"},
		{name: "absolute path", id: "/absolute/path"},
		{name: "empty", id: ""},
		{name: "percent encoded traversal", id: "..%2F..%2Fetc"},
		{name: "very long", id: strings.Repeat("A", 100000)},
	}
}

// filesUnder returns every regular file path under dir, relative to dir.
func filesUnder(t *testing.T, dir string) []string {
	t.Helper()
	var out []string
	require.NoError(t, filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	}))
	return out
}

// TestStickyStateKey_IsAlwaysADigest is the S3 oracle for INV-105: every
// identifier, hostile or degenerate, produces a fixed-shape filename.
func TestStickyStateKey_IsAlwaysADigest(t *testing.T) {
	t.Parallel()

	for _, tt := range hostileSessionIDs() {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			key := rulecond.StickyStateKey(tt.id)
			assert.Regexp(t, stateKeyShape, key,
				"the state filename must be a lowercase hex SHA-256 digest")
			assert.Equal(t, key, filepath.Base(key),
				"the derived name must carry no path component")
		})
	}

	assert.NotEqual(t,
		rulecond.StickyStateKey("alpha"), rulecond.StickyStateKey("beta"),
		"distinct sessions must not share a state file")
	assert.Equal(t,
		rulecond.StickyStateKey("alpha"), rulecond.StickyStateKey("alpha"),
		"the derivation must be deterministic")
}

// TestStickyFire_HostileSessionIDStaysInsideTheStateDirectory is the S3
// end-to-end oracle: every write lands under the state directory and no state
// file records the raw identifier.
func TestStickyFire_HostileSessionIDStaysInsideTheStateDirectory(t *testing.T) {
	for _, tt := range hostileSessionIDs() {
		t.Run(tt.name, func(t *testing.T) {
			f := newStickyFixture(t)
			f.writeShippedStickyPair()
			before := filesUnder(t, f.root)

			_, stderr := f.submitRaw(stickyPayloadFull(tt.id, "p", "/t.jsonl", "/c"), stickyDefaultCadence)
			assert.Empty(t, stderr)

			stateRel := filepath.ToSlash(filepath.FromSlash(rulecond.StickyStateDirRelPath)) + "/"
			var created []string
			for _, path := range filesUnder(t, f.root) {
				if !contains(before, path) {
					created = append(created, path)
				}
			}
			require.NotEmpty(t, created, "the counter must be persisted")
			for _, path := range created {
				assert.True(t, strings.HasPrefix(path, stateRel),
					"%q was created outside the state directory", path)
				assert.Regexp(t, stateKeyShape, filepath.Base(path))
			}

			raw, err := os.ReadFile(filepath.Join(f.stateDir(), rulecond.StickyStateKey(tt.id)))
			require.NoError(t, err)
			assert.Equal(t, "1", strings.TrimSpace(string(raw)),
				"the first prompt of a session records index 1")
			if tt.id != "" {
				assert.NotContains(t, string(raw), tt.id,
					"the raw session_id must never reach disk")
			}
		})
	}
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}

// TestStickyFire_PromptAndIdentityNeverLeave is the S5 oracle for
// REQ-STICKYRULE-FIRE-04: rule names and injectable bodies are the only things
// the runtime is allowed to emit or retain.
func TestStickyFire_PromptAndIdentityNeverLeave(t *testing.T) {
	f := newStickyFixture(t)
	f.writeShippedStickyPair()

	const (
		session    = "S-alpha"
		prompt     = "deploy with AWS_SECRET_ACCESS_KEY=" + decoySecret
		transcript = "/Users/example/.claude/projects/x/transcript.jsonl"
		cwd        = "/Users/example/private-repo"
	)

	stdout, stderr := f.submitRaw(stickyPayloadFull(session, prompt, transcript, cwd), stickyDefaultCadence)
	assert.Empty(t, stderr)

	ctx := stickyContext(t, stdout)
	assert.Contains(t, ctx, injectableBody(t, ruleLanguagePolicy))
	assert.Contains(t, ctx, injectableBody(t, ruleObjectiveReasoning))

	stateRaw, err := os.ReadFile(filepath.Join(f.stateDir(), rulecond.StickyStateKey(session)))
	require.NoError(t, err)

	for _, forbidden := range []string{decoySecret, "deploy with", session, "/Users/example"} {
		assert.NotContains(t, stdout, forbidden,
			"stdout must not echo %q", forbidden)
		assert.NotContains(t, string(stateRaw), forbidden,
			"the state file must not retain %q", forbidden)
	}
}
