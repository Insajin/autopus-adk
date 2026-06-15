package regen

import (
	"testing"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

// validWebPack returns a fully-formed gui-explore pack that passes
// journey.Validate (GUI policy satisfied). Used to verify that the AI-authority
// guard, not journey.Validate, is what rejects a web AI-authority pack.
func validWebPack(id string) journey.Pack {
	return journey.Pack{
		ID:      id,
		Title:   "Web GUI explore lane",
		Surface: "web",
		Lanes:   []string{"gui-explore"},
		Adapter: journey.AdapterRef{ID: "gui-explore"},
		Command: journey.Command{Argv: []string{"npm", "exec", "playwright", "test"}, CWD: ".", Timeout: "120s"},
		Checks:  []journey.Check{{ID: id, Type: "gui_exploration"}},
		GUI: journey.GUIPolicy{
			AllowedOrigins:   []string{"http://127.0.0.1:1420"},
			ForbiddenActions: []string{"mutation", "payment"},
			SelectorStrategy: "role-first",
			NetworkPolicy:    journey.GUINetworkPolicy{Mode: "local-only"},
		},
		SourceRefs: standardSourceRefs("SPEC-QAMESH-003", "AC-QAMESH3-004"),
	}
}

// validDesktopPack returns a node-script desktop pack that passes journey.Validate.
func validDesktopPack(id string) journey.Pack {
	return journey.Pack{
		ID:         id,
		Title:      "Desktop native lane",
		Surface:    "desktop",
		Lanes:      []string{"desktop-native"},
		Adapter:    journey.AdapterRef{ID: "node-script"},
		Command:    journey.Command{Argv: []string{"npm", "run", "build"}, CWD: ".", Timeout: defaultTimeout},
		Checks:     deterministicChecks(id),
		SourceRefs: standardSourceRefs("SPEC-QAMESH-005", "AC-QAMESH2-005"),
	}
}

// mustValidate fails the test if the pack does not pass journey.Validate.
func mustValidate(t *testing.T, pack journey.Pack, projectDir string) {
	t.Helper()
	if err := journey.Validate(pack, projectDir); err != nil {
		t.Fatalf("expected pack %q to pass journey.Validate, got: %v", pack.ID, err)
	}
}

// accepted builds a non-excluded SynthesizedPack for diff fixtures.
func accepted(surface string, pack journey.Pack) SynthesizedPack {
	return SynthesizedPack{Pack: pack, Surface: surface}
}
