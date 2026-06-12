package design

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestClassifyVisualArtifact(t *testing.T) {
	cases := []struct {
		name, path, want string
	}{
		{"home-expected.png", "snap/home-expected.png", "expected"},
		{"home-actual.png", "snap/home-actual.png", "actual"},
		{"home-diff.png", "snap/home-diff.png", "diff"},
		{"capture.png", "out/capture.png", "screenshot"},
		{"capture.jpeg", "out/capture.jpeg", "screenshot"},
		{"shot", "out/myscreenshot", "screenshot"},
		{"notes.txt", "out/notes.txt", "other"},
	}
	for _, tc := range cases {
		got := ClassifyVisualArtifact(tc.name, tc.path)
		if got != tc.want {
			t.Errorf("Classify(%q,%q) = %q, want %q", tc.name, tc.path, got, tc.want)
		}
	}
}

func TestVisualGateReport_Summary(t *testing.T) {
	report := VisualGateReport{
		Verdict: "PASS",
		Checks: []VisualCheck{
			{ID: "design_context", Status: "PASS", Message: "available"},
			{ID: "screenshot_capture", Status: "FAIL", Message: "none captured"},
		},
	}
	// With a path the header includes the slash-normalized path.
	s := report.Summary("out/report.json")
	if !strings.HasPrefix(s, "visual gate: PASS (out/report.json)") {
		t.Errorf("summary header = %q", s)
	}
	if !strings.Contains(s, "design_context: PASS — available") {
		t.Errorf("summary missing check line: %q", s)
	}
	if !strings.Contains(s, "screenshot_capture: FAIL — none captured") {
		t.Errorf("summary missing fail check line: %q", s)
	}
	// Without a path the parenthetical is omitted.
	noPath := report.Summary("")
	if strings.Contains(noPath, "(") {
		t.Errorf("empty-path summary should omit parens: %q", noPath)
	}
}

func TestPack_JSON(t *testing.T) {
	pack := Pack{Version: 1}
	data, err := pack.JSON()
	if err != nil {
		t.Fatalf("Pack.JSON: %v", err)
	}
	if data[len(data)-1] != '\n' {
		t.Error("Pack.JSON must end with newline")
	}
	var round Pack
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if round.Version != 1 {
		t.Errorf("round-trip version = %d, want 1", round.Version)
	}
}

func TestFigmaAudit_JSON(t *testing.T) {
	audit := FigmaAudit{
		FigmaRefs: []FigmaRef{{SourcePath: "DESIGN.md", FileKey: "abc", NodeID: "1:2"}},
		SetupGaps: []string{"figma_token_missing"},
	}
	data, err := audit.JSON()
	if err != nil {
		t.Fatalf("FigmaAudit.JSON: %v", err)
	}
	if data[len(data)-1] != '\n' {
		t.Error("FigmaAudit.JSON must end with newline")
	}
	var round FigmaAudit
	if err := json.Unmarshal(data, &round); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(round.FigmaRefs) != 1 || round.FigmaRefs[0].FileKey != "abc" {
		t.Errorf("round-trip refs = %+v", round.FigmaRefs)
	}
}
