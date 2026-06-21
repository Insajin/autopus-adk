package workflow

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

// DryRunReport is the inspectable output of `auto workflow render --dry-run`:
// the deterministic phase order, the gate verdict source, the manifest/schema
// paths, a deterministic prompt-manifest hash, and the generated workflow JS.
type DryRunReport struct {
	PhaseOrder         []string        `json:"phase_order"`
	GateVerdictSource  string          `json:"gate_verdict_source"`
	ManifestPath       string          `json:"manifest_path"`
	SchemaPath         string          `json:"schema_path"`
	PromptManifestHash string          `json:"prompt_manifest_hash"`
	JS                 string          `json:"js"`
	Phases             []RenderedPhase `json:"phases"`
}

// RenderedPhase exposes the per-phase model, effort, and depth surface so the
// dry-run report is inspectable (REQ-012, S9). It is the rendered (baseline or
// overlaid) view of a single phase.
type RenderedPhase struct {
	ID          string `json:"id"`
	Model       string `json:"model"`
	Effort      string `json:"effort"`
	VerifyVotes int    `json:"verify_votes"`
	FanOutCap   int    `json:"fan_out_cap"`
	Synthesis   bool   `json:"synthesis"`
}

// Render builds the dry-run report from the parsed schema, prompt layers, and
// generated JS. PhaseOrder comes from the manifest; GateVerdictSource is the
// canonical "exit_code"; the prompt-manifest hash is deterministic over the
// non-ephemeral layers. Render is pure: it reads no files, the CLI passes data.
func Render(s Schema, layers []promptlayer.Layer, jsContent, manifestPath, schemaPath string) DryRunReport {
	return DryRunReport{
		PhaseOrder:         s.PhaseIDs(),
		GateVerdictSource:  VerdictSourceExitCode,
		ManifestPath:       manifestPath,
		SchemaPath:         schemaPath,
		PromptManifestHash: PromptManifestHash(layers),
		JS:                 jsContent,
		Phases:             OverlayPhases(s, nil),
	}
}

// PromptManifestHash folds the sorted per-layer content hashes of the
// non-ephemeral (stable + snapshot) layers into one sha256 hex digest. Ephemeral
// layers are excluded, so mutating only ephemeral context leaves the hash
// unchanged while mutating any stable/snapshot layer changes it.
func PromptManifestHash(layers []promptlayer.Layer) string {
	filtered := make([]promptlayer.Layer, 0, len(layers))
	for _, l := range layers {
		if l.Kind != promptlayer.KindEphemeral {
			filtered = append(filtered, l)
		}
	}

	result, err := promptlayer.Render(filtered)
	if err != nil {
		return ""
	}

	h := sha256.New()
	for _, entry := range result.Manifest.Entries {
		h.Write([]byte(entry.Hash))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}
