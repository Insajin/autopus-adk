package companionmanifest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHomebrewFormulaBridge_RejectsAlreadyCurrentFormulaRace(t *testing.T) {
	fixture := newHomebrewBridgeFixture(t)
	if output, err := fixture.run(nil); err != nil {
		t.Fatalf("publish Cask fixture: %v\n%s", err, output)
	}
	writeTapRaceMarker(t, fixture, "idempotent-formula-race")
	output, err := fixture.run(nil)
	if err == nil || !strings.Contains(string(output), "idempotent tree") {
		t.Fatalf("idempotent Formula race result: %v\n%s", err, output)
	}
	if got := fixture.updateCount(t, "cask"); got != "1" {
		t.Fatalf("idempotent Formula race performed %s Cask updates", got)
	}
	if got := tapContentBlob(t, fixture, "formula.json"); got != "6666666666666666666666666666666666666666" {
		t.Fatalf("idempotent Formula race blob = %s", got)
	}
}

func TestHomebrewFormulaBridge_RejectsAlreadyCurrentFinalHeadRace(t *testing.T) {
	fixture := newHomebrewBridgeFixture(t)
	if output, err := fixture.run(nil); err != nil {
		t.Fatalf("publish Cask fixture: %v\n%s", err, output)
	}
	if err := os.Remove(filepath.Join(fixture.state, "branch-get.calls")); err != nil {
		t.Fatal(err)
	}
	writeTapRaceMarker(t, fixture, "idempotent-ref-race")
	output, err := fixture.run(nil)
	if err == nil || !strings.Contains(string(output), "head moved during idempotent verification") {
		t.Fatalf("idempotent final-head race result: %v\n%s", err, output)
	}
	if got := fixture.updateCount(t, "cask"); got != "1" {
		t.Fatalf("idempotent final-head race performed %s Cask updates", got)
	}
	if got := tapBranchHead(t, fixture); got != "4444444444444444444444444444444444444444" {
		t.Fatalf("idempotent final-head race commit = %s", got)
	}
}

func TestHomebrewFormulaBridge_RejectsAlreadyCurrentMalformedCaskBlob(t *testing.T) {
	fixture := newHomebrewBridgeFixture(t)
	if output, err := fixture.run(nil); err != nil {
		t.Fatalf("publish Cask fixture: %v\n%s", err, output)
	}
	current := fixture.apiContent(t, "cask.json")
	fixture.writeAPIContent(t, "cask.json", "not-a-git-blob", current)
	output, err := fixture.run(nil)
	if err == nil || !strings.Contains(string(output), "invalid blob SHA") {
		t.Fatalf("idempotent malformed Cask blob result: %v\n%s", err, output)
	}
	if got := fixture.updateCount(t, "cask"); got != "1" {
		t.Fatalf("malformed Cask blob performed %s Cask updates", got)
	}
}

func writeTapRaceMarker(t *testing.T, fixture homebrewBridgeFixture, name string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(fixture.state, name), nil, 0o600); err != nil {
		t.Fatal(err)
	}
}

func tapContentBlob(t *testing.T, fixture homebrewBridgeFixture, name string) string {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join(fixture.state, name))
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		SHA string `json:"sha"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatal(err)
	}
	return response.SHA
}

func tapBranchHead(t *testing.T, fixture homebrewBridgeFixture) string {
	t.Helper()
	payload, err := os.ReadFile(filepath.Join(fixture.state, "branch.json"))
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Object struct {
			SHA string `json:"sha"`
		} `json:"object"`
	}
	if err := json.Unmarshal(payload, &response); err != nil {
		t.Fatal(err)
	}
	return response.Object.SHA
}
