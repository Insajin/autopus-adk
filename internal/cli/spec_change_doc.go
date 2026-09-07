package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/insajin/autopus-adk/pkg/spec"
	"github.com/insajin/autopus-adk/pkg/spec/gates"
)

// specChangeSchema identifies the compact change contract payload.
const specChangeSchema = "autopus.change-contract.v1"

// specChangeContract is the machine-readable compact change contract. It
// references an existing SPEC by id and acceptance-criteria ids; it never
// restates requirements.
type specChangeContract struct {
	Schema           string               `json:"schema"`
	SpecID           string               `json:"spec_id"`
	SpecPath         string               `json:"spec_path"`
	ChangeID         string               `json:"change_id,omitempty"`
	AcceptanceIDs    []string             `json:"acceptance_ids"`
	Surface          []string             `json:"intended_surface"`
	VerificationPlan []string             `json:"verification_plan"`
	NewContract      bool                 `json:"new_exported_contract"`
	Risk             gates.ChangeRisk     `json:"change_risk"`
	Gates            []gates.GateDecision `json:"gates"`
	ContractPath     string               `json:"contract_path,omitempty"`
	GeneratedAt      string               `json:"generated_at"`
}

// resolveAcceptanceIDs verifies every requested acceptance-criteria id exists
// in the SPEC's acceptance.md. Ids match either a parsed criterion id or the
// literal document text, so explicit scenario ids resolve alongside the
// auto-assigned AC-NNN ids.
func resolveAcceptanceIDs(specDir string, requested []string) ([]string, error) {
	path := filepath.Join(specDir, "acceptance.md")
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w (a compact change contract must reference existing acceptance criteria)",
			filepath.ToSlash(path), err)
	}
	criteria, _ := spec.ParseGherkin(string(content))
	known := make(map[string]bool, len(criteria))
	names := make([]string, 0, len(criteria))
	for _, criterion := range criteria {
		known[strings.ToUpper(criterion.ID)] = true
		names = append(names, criterion.ID)
	}
	sort.Strings(names)

	seen := map[string]bool{}
	resolved := make([]string, 0, len(requested))
	for _, id := range trimmedList(requested) {
		if seen[id] {
			continue
		}
		seen[id] = true
		if known[strings.ToUpper(id)] || strings.Contains(string(content), id) {
			resolved = append(resolved, id)
			continue
		}
		return nil, fmt.Errorf("acceptance criterion %q not found in %s (parsed ids: %s)",
			id, filepath.ToSlash(path), strings.Join(names, ", "))
	}
	return resolved, nil
}

// printSpecChange emits the decision document. It prints on escalation too,
// so a refused compact contract still yields a machine-readable decision.
func printSpecChange(cmd *cobra.Command, contract specChangeContract, asJSON bool) {
	w := cmd.OutOrStdout()
	if asJSON {
		data, err := json.MarshalIndent(contract, "", "  ")
		if err != nil {
			fmt.Fprintf(w, "encode change contract: %v\n", err)
			return
		}
		fmt.Fprintln(w, string(data))
		return
	}
	fmt.Fprintf(w, "%s: %s (declared %s, effective %s, risk %s)\n",
		contract.SpecID, contract.Risk.Decision, contract.Risk.DeclaredClass,
		contract.Risk.EffectiveClass, contract.Risk.Tier)
	if len(contract.Risk.Reasons) > 0 {
		fmt.Fprintf(w, "reasons: %s\n", strings.Join(contract.Risk.Reasons, ", "))
	}
	fmt.Fprintf(w, "acceptance: %s\n", strings.Join(contract.AcceptanceIDs, ", "))
	for _, decision := range contract.Gates {
		fmt.Fprintf(w, "%s: %s — %s\n", decision.Gate, decision.Applicability, decision.Reason)
	}
	if contract.ContractPath != "" {
		fmt.Fprintf(w, "contract: %s\n", contract.ContractPath)
	}
}

// renderChangeContract renders the compact change.md body. Every field the
// pipeline needs is present; nothing from spec.md is copied.
func renderChangeContract(c specChangeContract) string {
	var sb strings.Builder
	title := c.SpecID
	if c.ChangeID != "" {
		title = "CHG-" + c.ChangeID + " → " + c.SpecID
	}
	fmt.Fprintf(&sb, "# Change Contract: %s\n\n", title)
	sb.WriteString("---\n")
	fmt.Fprintf(&sb, "schema: %s\n", c.Schema)
	fmt.Fprintf(&sb, "spec_id: %s\n", c.SpecID)
	if c.ChangeID != "" {
		fmt.Fprintf(&sb, "change_id: CHG-%s\n", c.ChangeID)
	}
	fmt.Fprintf(&sb, "declared_class: %s\n", c.Risk.DeclaredClass)
	fmt.Fprintf(&sb, "effective_class: %s\n", c.Risk.EffectiveClass)
	fmt.Fprintf(&sb, "risk_tier: %s\n", c.Risk.Tier)
	fmt.Fprintf(&sb, "decision: %s\n", c.Risk.Decision)
	fmt.Fprintf(&sb, "new_exported_contract: %t\n", c.NewContract)
	fmt.Fprintf(&sb, "generated_at: %s\n", c.GeneratedAt)
	sb.WriteString("---\n\n")

	sb.WriteString("## Referenced SPEC\n\n")
	fmt.Fprintf(&sb, "- SPEC: `%s` (`%s`)\n", c.SpecID, c.SpecPath)
	fmt.Fprintf(&sb, "- Acceptance criteria: %s\n\n", backquoteList(c.AcceptanceIDs))
	sb.WriteString("Requirements and acceptance criteria stay in the referenced SPEC. This contract adds no requirement of its own.\n\n")

	sb.WriteString("## Intended Surface\n\n")
	for _, path := range c.Surface {
		fmt.Fprintf(&sb, "- `%s`\n", path)
	}
	sb.WriteString("\nA change outside this surface invalidates the contract: re-run `auto spec change` with the real surface.\n\n")

	sb.WriteString("## Verification Plan\n\n")
	for i, entry := range c.VerificationPlan {
		fmt.Fprintf(&sb, "%d. %s\n", i+1, entry)
	}
	sb.WriteString("\nOne merged final verification run is enough, but the receipt must record a verdict and evidence per referenced acceptance-criteria id.\n\n")

	renderChangeGateTable(&sb, c)
	return sb.String()
}

func renderChangeGateTable(sb *strings.Builder, c specChangeContract) {
	sb.WriteString("## Gate Applicability\n\n")
	sb.WriteString("| gate | applicability | reason |\n|---|---|---|\n")
	for _, decision := range c.Gates {
		fmt.Fprintf(sb, "| `%s` | `%s` | %s |\n", decision.Gate, decision.Applicability, decision.Reason)
	}
	sb.WriteString("\n`security`, `validation`, `data_loss`, and `deterministic_oracle` are never `not_applicable`. ")
	sb.WriteString("Accessibility and UX capture stay `required` whenever the surface has UI paths.\n")
}

func backquoteList(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, "`"+value+"`")
	}
	return strings.Join(quoted, ", ")
}
