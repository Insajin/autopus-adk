package release

import (
	"path/filepath"

	"github.com/insajin/autopus-adk/pkg/qa/journey"
	qaproject "github.com/insajin/autopus-adk/pkg/qa/project"
)

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-QAMESH-004: dry-run release plan is the side-effect-free contract for gate inspection.
// @AX:REASON: CLI dry-run, release execution preflight, and template guidance depend on setup gaps, blocker matrix, redaction, and sibling SPEC fields staying aligned.
func BuildPlan(opts Options) (Plan, error) {
	opts = normalizeOptions(opts)
	if err := ValidateProfile(opts.Profile); err != nil {
		return Plan{}, err
	}
	policy, _ := profilePolicy(opts.Profile)
	packs, err := journey.LoadDir(opts.ProjectDir)
	if err != nil {
		return Plan{}, err
	}
	journeyRows, redactionStatus := journeyPackRows(packs)
	policy = adaptPolicyToProject(opts.ProjectDir, policy, journeyRows)
	blockerRules := blockerRulesForPolicy(opts.Profile, policy)
	setupGaps := releaseSetupGaps(policy, journeyRows)
	redactionStatus = setupGapRedactionStatus(setupGaps, redactionStatus)
	return Plan{
		SchemaVersion:   PlanSchemaVersion,
		Command:         opts.Command,
		DryRun:          true,
		Profile:         opts.Profile,
		LaneCatalog:     LaneCatalog(),
		SelectedLanes:   ReleaseLanes(),
		JourneyPacks:    journeyRows,
		SetupGaps:       setupGaps,
		BlockerRules:    blockerRules,
		OutputPaths:     planOutputPaths(opts),
		SiblingSpecs:    SiblingSpecs(),
		RedactionStatus: redactionStatus,
		RedactionRules:  RedactionRules(),
		SideEffects:     []string{},
	}, nil
}

func adaptPolicyToProject(projectDir string, policy ProfilePolicy, rows []JourneyPackRow) ProfilePolicy {
	hasBrowser := qaproject.HasBrowserSignals(projectDir) || laneCoveredByRows(rows, "browser-staging")
	hasDesktop := qaproject.HasDesktopGUISignals(projectDir) || laneCoveredByRows(rows, "desktop-native")
	hasGUI := hasBrowser || hasDesktop || laneCoveredByRows(rows, "gui-explore")
	if !hasBrowser {
		policy = demoteLane(policy, "browser-staging")
	}
	if !hasDesktop {
		policy = demoteLane(policy, "desktop-native")
	}
	if !hasGUI {
		policy = demoteLane(policy, "gui-explore")
	}
	return policy
}

func laneCoveredByRows(rows []JourneyPackRow, lane string) bool {
	for _, row := range rows {
		if row.Lane == lane {
			return true
		}
	}
	return false
}

func demoteLane(policy ProfilePolicy, lane string) ProfilePolicy {
	policy.MustLanes = removeLane(policy.MustLanes, lane)
	policy.OptionalLanes = removeLane(policy.OptionalLanes, lane)
	if !containsLane(policy.DeferredLanes, lane) {
		policy.DeferredLanes = append(policy.DeferredLanes, lane)
	}
	return policy
}

func removeLane(values []string, lane string) []string {
	out := values[:0]
	for _, value := range values {
		if value != lane {
			out = append(out, value)
		}
	}
	return out
}

func containsLane(values []string, lane string) bool {
	for _, value := range values {
		if value == lane {
			return true
		}
	}
	return false
}

func journeyPackRows(packs []journey.Pack) ([]JourneyPackRow, RedactionState) {
	rows := []JourneyPackRow{}
	redactionStatus := RedactionClean
	for _, pack := range packs {
		for _, lane := range ReleaseLanes() {
			if !journey.HasLane(pack, lane) {
				continue
			}
			preview, redacted := commandPreview(pack.Command.Argv, pack.Command.Run)
			if redacted {
				redactionStatus = RedactionRedacted
			}
			rows = append(rows, JourneyPackRow{
				Lane:                   lane,
				JourneyID:              pack.ID,
				Adapter:                pack.Adapter.ID,
				Source:                 sourceLabel(pack.Source),
				CommandDeclared:        commandDeclared(pack.Command),
				CommandPreview:         preview,
				CommandPreviewRedacted: redacted,
				Executable:             commandDeclared(pack.Command),
				SourceSpec:             sourceSpecForPack(pack, lane),
				AcceptanceRefs:         nonNilStrings(pack.SourceRefs.AcceptanceRefs),
				InventedCommand:        false,
			})
		}
	}
	return rows, redactionStatus
}

func nonNilStrings(values []string) []string {
	if values == nil {
		return []string{}
	}
	return values
}

func releaseSetupGaps(policy ProfilePolicy, journeyRows []JourneyPackRow) []SetupGapRow {
	covered := map[string]bool{}
	for _, row := range journeyRows {
		if row.CommandDeclared && row.Executable {
			covered[row.Lane] = true
		}
	}
	gaps := []SetupGapRow{}
	for _, lane := range ReleaseLanes() {
		if covered[lane] {
			continue
		}
		lanePolicy := lanePolicy(policy, lane)
		class, reason := setupGapForLane(lane)
		severity := severityForSetupGap(lanePolicy, class)
		normalized := NormalizeLaneRow(LaneRow{
			Lane:          lane,
			LanePolicy:    lanePolicy,
			Status:        LaneStatusSetupGap,
			SetupGapClass: class,
			Severity:      severity,
		})
		catalog := laneByID(lane)
		gaps = append(gaps, SetupGapRow{
			Lane:            lane,
			SetupGapClass:   class,
			Reason:          reason,
			Severity:        severity,
			Blocking:        normalized.LaneVerdict == LaneVerdictBlock,
			OwnerSpec:       catalog.OwnerSpec,
			OwnerRepo:       catalog.OwnerRepo,
			InventedCommand: false,
		})
	}
	return gaps
}

func setupGapForLane(lane string) (SetupGapClass, string) {
	switch lane {
	case "canary-explicit":
		return SetupGapCanaryTemplate, "explicit safe canary command is required"
	case "mobile-readiness", "evidence-dashboard":
		return SetupGapSiblingSpecPending, "sibling SPEC readiness contract is pending"
	default:
		return SetupGapMissingJourneyPack, "project-local Journey Pack is required"
	}
}

func planOutputPaths(opts Options) OutputPaths {
	return OutputPaths{
		ReleaseIndexPreviewPath: filepath.Join(opts.Output, "<release-id>", "release-index.json"),
		RunIndexRoot:            opts.RunOutputRoot,
		EvidenceRoot:            filepath.Join(opts.ProjectDir, ".autopus", "qa", "evidence"),
		FeedbackRoot:            filepath.Join(opts.ProjectDir, ".autopus", "qa", "feedback"),
	}
}

func sourceLabel(value string) string {
	if value == "" {
		return "configured"
	}
	return value
}

func commandDeclared(command journey.Command) bool {
	return len(command.Argv) > 0 || command.Run != ""
}

func sourceSpecForPack(pack journey.Pack, lane string) string {
	if pack.SourceRefs.SourceSpec != "" {
		return pack.SourceRefs.SourceSpec
	}
	return laneByID(lane).OwnerSpec
}

func setupGapRedactionStatus(gaps []SetupGapRow, current RedactionState) RedactionState {
	for _, gap := range gaps {
		if gap.SetupGapClass == SetupGapPolicyForbidden || gap.SetupGapClass == SetupGapUnsafeCommand {
			return RedactionBlocked
		}
	}
	return current
}
