package release

func BlockerRulesForProfile(profile string) (BlockerRules, error) {
	policy, err := profilePolicy(profile)
	if err != nil {
		return BlockerRules{}, err
	}
	return blockerRulesForPolicy(profile, policy), nil
}

func blockerRulesForPolicy(profile string, policy ProfilePolicy) BlockerRules {
	return BlockerRules{
		Profile:       profile,
		MustLanes:     policy.MustLanes,
		OptionalLanes: policy.OptionalLanes,
		DeferredLanes: policy.DeferredLanes,
		SeverityOrder: SeverityOrder(),
		MatrixVersion: BlockingMatrixVersion,
		RuleRows:      policyRuleRows(),
	}
}

// @AX:ANCHOR [AUTO] @AX:SPEC: SPEC-QAMESH-004: lane row normalization is shared by planning, execution, and blocker matrix tests.
// @AX:REASON: Callers depend on a single verdict source for setup gaps, failed lanes, deferred lanes, and invalid row contracts.
// @AX:WARN [AUTO] @AX:SPEC: SPEC-QAMESH-004: release lane normalization has 8+ guard branches and intentionally fails closed.
// @AX:REASON: Blocking correctness depends on preserving setup-gap precedence, severity defaults, invalid-row quarantine, and blocker injection order.
func NormalizeLaneRow(row LaneRow) LaneRow {
	if row.Status == LaneStatusWarning {
		row.Status = LaneStatusWarn
	}
	if row.SetupGapClass == "" {
		row.SetupGapClass = SetupGapNone
	}
	if row.SetupGapClass != SetupGapNone && (row.Status == "" || row.Status == LaneStatusWarning || row.Status == LaneStatusWarn) {
		row.Status = LaneStatusSetupGap
	}
	if row.Severity == "" {
		row.Severity = SeverityNone
	}
	if row.ManifestPaths == nil {
		row.ManifestPaths = []string{}
	}
	if row.FeedbackRefs == nil {
		row.FeedbackRefs = []string{}
	}
	if row.Blockers == nil {
		row.Blockers = []Blocker{}
	}
	if row.Status == LaneStatusSetupGap && row.Severity == SeverityNone {
		row.Severity = severityForSetupGap(row.LanePolicy, row.SetupGapClass)
	}
	if (row.Status == LaneStatusFailed || row.Status == LaneStatusBlocked) && row.Severity == SeverityNone {
		if row.LanePolicy == LanePolicyMust {
			row.Severity = SeverityHigh
		} else {
			row.Severity = SeverityMedium
		}
	}
	if invalidLaneRow(row) {
		row.Status = LaneStatusBlocked
		row.SetupGapClass = SetupGapPolicyForbidden
		row.Severity = SeverityHigh
		row.Blockers = []Blocker{{Lane: row.Lane, Reason: "invalid_lane_row_contract"}}
	}
	row.LaneVerdict = evaluateVerdict(row)
	if row.LaneVerdict == LaneVerdictBlock && len(row.Blockers) == 0 {
		row.Blockers = []Blocker{{Lane: row.Lane, Reason: "release_lane_blocked"}}
	}
	return row
}

func AggregateGateStatus(rows []LaneRow) GateStatus {
	status := GateStatusPassed
	for _, row := range rows {
		switch row.LaneVerdict {
		case LaneVerdictBlock:
			return GateStatusBlocked
		case LaneVerdictWarn:
			status = GateStatusWarn
		}
	}
	return status
}

func invalidLaneRow(row LaneRow) bool {
	switch row.Status {
	case LaneStatusPassed:
		return row.SetupGapClass != SetupGapNone || row.Severity != SeverityNone || len(row.Blockers) > 0
	case LaneStatusSetupGap:
		return row.SetupGapClass == SetupGapNone || row.Severity == SeverityNone
	case LaneStatusFailed, LaneStatusBlocked:
		return row.Severity == SeverityNone
	case LaneStatusDeferred, LaneStatusSkipped:
		return row.SetupGapClass == SetupGapPolicyForbidden || row.SetupGapClass == SetupGapUnsafeCommand
	}
	return false
}

func evaluateVerdict(row LaneRow) LaneVerdict {
	if row.SetupGapClass == SetupGapPolicyForbidden || row.SetupGapClass == SetupGapUnsafeCommand {
		return LaneVerdictBlock
	}
	switch row.LanePolicy {
	case LanePolicyMust:
		return mustVerdict(row)
	case LanePolicyOptional:
		return optionalVerdict(row)
	default:
		return deferredVerdict(row)
	}
}

func mustVerdict(row LaneRow) LaneVerdict {
	switch row.Status {
	case LaneStatusPassed:
		return LaneVerdictPass
	case LaneStatusWarn:
		if severityAtLeast(row.Severity, SeverityMedium) {
			return LaneVerdictBlock
		}
		return LaneVerdictWarn
	default:
		return LaneVerdictBlock
	}
}

func optionalVerdict(row LaneRow) LaneVerdict {
	switch row.Status {
	case LaneStatusPassed:
		return LaneVerdictPass
	case LaneStatusFailed, LaneStatusBlocked:
		if severityAtLeast(row.Severity, SeverityHigh) {
			return LaneVerdictBlock
		}
		return LaneVerdictWarn
	default:
		return LaneVerdictWarn
	}
}

func deferredVerdict(row LaneRow) LaneVerdict {
	if (row.Status == LaneStatusFailed || row.Status == LaneStatusBlocked) && row.Severity == SeverityCritical {
		return LaneVerdictBlock
	}
	if row.Status == LaneStatusPassed || (row.Status == LaneStatusDeferred && row.SetupGapClass == SetupGapNone) {
		return LaneVerdictPass
	}
	return LaneVerdictWarn
}

func severityForSetupGap(policy LanePolicy, class SetupGapClass) Severity {
	switch class {
	case SetupGapPolicyForbidden, SetupGapUnsafeCommand:
		return SeverityCritical
	case SetupGapMissingJourneyPack, SetupGapToolUnavailable, SetupGapEnvMissing:
		if policy == LanePolicyMust {
			return SeverityHigh
		}
		return SeverityMedium
	case SetupGapCanaryTemplate:
		if policy == LanePolicyMust {
			return SeverityHigh
		}
		return SeverityLow
	case SetupGapSiblingSpecPending:
		if policy == LanePolicyMust {
			return SeverityMedium
		}
		return SeverityLow
	default:
		return SeverityNone
	}
}

func severityAtLeast(value, threshold Severity) bool {
	order := map[Severity]int{}
	for index, severity := range SeverityOrder() {
		order[severity] = index
	}
	return order[value] >= order[threshold]
}

// @AX:NOTE [AUTO] @AX:SPEC: SPEC-QAMESH-004: blocker matrix rows are governance policy and must stay aligned with NormalizeLaneRow verdict semantics.
func policyRuleRows() []BlockerRuleRow {
	return []BlockerRuleRow{
		{LanePolicy: LanePolicyMust, LaneStatus: "passed", SetupGapClass: "none", Severity: "none", LaneVerdict: LaneVerdictPass, GateEffect: "keep current gate status"},
		{LanePolicy: LanePolicyMust, LaneStatus: "warn", SetupGapClass: "none", Severity: "none|info|low", LaneVerdict: LaneVerdictWarn, GateEffect: "keep current gate status"},
		{LanePolicy: LanePolicyMust, LaneStatus: "warn", SetupGapClass: "none", Severity: "medium|high|critical", LaneVerdict: LaneVerdictBlock, GateEffect: "gate blocked"},
		{LanePolicy: LanePolicyMust, LaneStatus: "failed|blocked|setup_gap|deferred|skipped", SetupGapClass: "any", Severity: "any", LaneVerdict: LaneVerdictBlock, GateEffect: "gate blocked"},
		{LanePolicy: LanePolicyOptional, LaneStatus: "passed", SetupGapClass: "none", Severity: "none", LaneVerdict: LaneVerdictPass, GateEffect: "keep current gate status"},
		{LanePolicy: LanePolicyOptional, LaneStatus: "warn|setup_gap|deferred|skipped", SetupGapClass: "non-policy", Severity: "any", LaneVerdict: LaneVerdictWarn, GateEffect: "keep current gate status"},
		{LanePolicy: LanePolicyOptional, LaneStatus: "failed|blocked", SetupGapClass: "none", Severity: "info|low|medium", LaneVerdict: LaneVerdictWarn, GateEffect: "keep current gate status"},
		{LanePolicy: LanePolicyOptional, LaneStatus: "failed|blocked", SetupGapClass: "none", Severity: "high|critical", LaneVerdict: LaneVerdictBlock, GateEffect: "gate blocked"},
		{LanePolicy: LanePolicyOptional, LaneStatus: "any", SetupGapClass: "policy-forbidden|unsafe-command", Severity: "any", LaneVerdict: LaneVerdictBlock, GateEffect: "gate blocked"},
		{LanePolicy: LanePolicyDeferred, LaneStatus: "passed|deferred", SetupGapClass: "none", Severity: "none", LaneVerdict: LaneVerdictPass, GateEffect: "keep current gate status"},
		{LanePolicy: LanePolicyDeferred, LaneStatus: "warn|setup_gap|skipped", SetupGapClass: "non-policy", Severity: "any", LaneVerdict: LaneVerdictWarn, GateEffect: "keep current gate status"},
		{LanePolicy: LanePolicyDeferred, LaneStatus: "failed|blocked", SetupGapClass: "none", Severity: "info|low|medium|high", LaneVerdict: LaneVerdictWarn, GateEffect: "keep current gate status"},
		{LanePolicy: LanePolicyDeferred, LaneStatus: "failed|blocked|any", SetupGapClass: "policy-forbidden|unsafe-command|none", Severity: "critical", LaneVerdict: LaneVerdictBlock, GateEffect: "gate blocked"},
	}
}
