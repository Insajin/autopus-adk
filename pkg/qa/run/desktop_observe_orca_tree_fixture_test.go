package run

import (
	"fmt"
	"strings"
)

// SPEC-QAMESH-013: the shared Orca fixtures render the real capture in
// testdata/desktop_trees/autopus.txt, not a synthetic UI.
//
// The fixture they replace was a 14-line invention that no shipped application
// produced, and validOrcaSnapshot demanded it line for line. Because the fixture
// and the validator were written to each other, the lane looked green here and
// refused every real app - including ours.
//
// The identity below is therefore the measured one: a genuine bundle id, and an
// app name and window title that both contain a space. The old fixture used
// "Autopus" for the window title, which no Autopus Desktop window is titled and
// which hid the fact that REQ-7 has to carry a space through a pipeline whose
// published refs forbid one.
const (
	orcaFixtureAppID       = "co.autopus.desktop"
	orcaFixtureAppName     = "Autopus Desktop"
	orcaFixtureWindowTitle = "Autopus Desktop"

	// orcaFixtureNodeCount is the rendered tree's node count, reported as
	// snapshot.elementCount. Fixture data, not a bound: validOrcaSnapshot no
	// longer pins the value, only the field's presence.
	orcaFixtureNodeCount = 8

	// orcaFixtureFocusedID is the element the capture reports as focused. It is
	// inside the body, so the window projects as focused.
	orcaFixtureFocusedID = 2

	// orcaFixtureButtonName is the one addition to the captured tree: a labelled
	// control inside the web content, declared by the fixture pack as a third
	// landmark below the window.
	//
	// The capture was taken on a splash screen, so it contains no labelled
	// control whose state can change - and without one, nothing proves that REQ-4
	// publishes a declared node BELOW the window rather than only the two
	// canonical landmarks. The rendering is the real one: Orca emits role words
	// then the accessibility label, and inlines a state marker as "(disabled)" or
	// "(expanded)".
	orcaFixtureButtonName = "Run details"

	// These two strings are observed content the pack never declares. They exist
	// so REQ-4 has something real to refuse to publish: the document title is
	// in-flight product copy and the status line is user-visible progress text.
	orcaFixtureDocumentTitle = "Autopus Desktop — AI가 회사를 운영합니다"
	orcaFixtureStatusText    = "데스크탑 셸을 깨우는 중 Autopus를 준비하는 중입니다."
)

// orcaTreeFixture parameterizes the shared Orca accessibility tree.
//
// The zero value renders the capture verbatim except for the pid, which follows
// orcaTestPID so the tree agrees with the fixture's window binding. Every field
// exists because some test proves the decoder fails closed when the provider
// answers about something other than what the pack declared - there is no field
// here that only makes the happy path prettier.
type orcaTreeFixture struct {
	AppID       string
	PID         int
	AppName     string
	WindowTitle string
	// FocusedID is the element id the trailing focus line names. An id absent
	// from the body is legal wire shape rather than an error, because Orca
	// reports focus for the session and not for the window it rendered; it
	// projects as an unfocused window.
	FocusedID int
	// ButtonState is the state marker the renderer inlines into the declared
	// button's role phrase, e.g. "disabled" or "expanded". Empty renders no
	// marker, which is how Orca reports an ordinary enabled, collapsed control.
	ButtonState string
	// PadNodes appends this many depth-1 siblings, so a test can reach the
	// parser's node bound with a legitimately shaped tree instead of a byte blob
	// that the byte bound would refuse first.
	PadNodes int
}

// withDefaults fills the captured values. Zero means "as captured" for every
// field, so a variant states only what it is varying and cannot drift from the
// capture by accident.
func (fixture orcaTreeFixture) withDefaults() orcaTreeFixture {
	if fixture.AppID == "" {
		fixture.AppID = orcaFixtureAppID
	}
	if fixture.PID == 0 {
		fixture.PID = orcaTestPID
	}
	if fixture.AppName == "" {
		fixture.AppName = orcaFixtureAppName
	}
	if fixture.WindowTitle == "" {
		fixture.WindowTitle = orcaFixtureWindowTitle
	}
	if fixture.FocusedID == 0 {
		fixture.FocusedID = orcaFixtureFocusedID
	}
	return fixture
}

// elementCount is what the provider would report alongside this tree. Keeping it
// derived is what stops an envelope and its tree from disagreeing: a padded tree
// that still claimed the captured count would be refused as inconsistent rather
// than testing what the variant was written for.
func (fixture orcaTreeFixture) elementCount() int {
	return orcaFixtureNodeCount + fixture.PadNodes
}

// buttonPhrase renders the declared button's line body.
func (fixture orcaTreeFixture) buttonPhrase() string {
	if fixture.ButtonState == "" {
		return "button " + orcaFixtureButtonName
	}
	return "button (" + fixture.ButtonState + ") " + orcaFixtureButtonName
}

// render emits the rendered accessibility tree.
//
// Node 0 carries the window title because that is what the accessibility label
// of a window element is, so overriding WindowTitle moves the header and the
// element together, the way a real retitled window would.
func (fixture orcaTreeFixture) render() string {
	spec := fixture.withDefaults()
	lines := []string{
		fmt.Sprintf("App=%s (pid %d)", spec.AppID, spec.PID),
		fmt.Sprintf("Window: %q, App: %s.", spec.WindowTitle, spec.AppName),
		"",
		"0 standard window " + spec.WindowTitle,
		"\t1 scroll area",
		"\t\t2 HTML content " + orcaFixtureDocumentTitle,
		"\t\t\t3 " + spec.buttonPhrase(),
		"\t\t\t4 container, Text: " + orcaFixtureStatusText,
		"\t5 close button",
		"\t6 full screen button, Secondary Actions: zoom the window",
		"\t7 minimize button",
	}
	for index := range spec.PadNodes {
		lines = append(lines, fmt.Sprintf("\t%d group", orcaFixtureNodeCount+index))
	}
	return strings.Join(append(lines, "", fmt.Sprintf(
		"The focused UI element is %d HTML content %s.", spec.FocusedID, orcaFixtureDocumentTitle,
	)), "\n")
}
