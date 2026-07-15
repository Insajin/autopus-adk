package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/insajin/autopus-adk/pkg/experiment"
	"github.com/insajin/autopus-adk/pkg/workflow"
	"github.com/spf13/cobra"
)

const (
	bindingReasonCompact          = "eligible_compact"
	bindingReasonRiskFull         = "risk_requires_full"
	bindingReasonUnknown          = "unknown_risk"
	bindingReasonMissing          = "missing_risk_evidence"
	bindingReasonMalformed        = "malformed_risk_input"
	bindingReasonDiscoveryFailed  = "risk_discovery_failed"
	bindingReasonValidationFailed = "binding_validation_failed"
	bindingReasonAudit            = "audit_sample"
	bindingReasonBalanced         = "canonical_balanced"
	bindingReasonShadow           = "shadow_full"
	bindingReasonContextMissing   = "context_integrity_missing"
	bindingReasonContextFailed    = "context_integrity_failed"
	maxBindingJSONBytes           = 64 << 10
	maxBindingFiles               = 1000
	maxBindingPathBytes           = 4096
)

type workflowBindingReceipt struct {
	Quality                json.RawMessage `json:"quality"`
	Risk                   string          `json:"risk"`
	SelectionReason        string          `json:"selection_reason"`
	ReviewVotes            int             `json:"review_votes"`
	SecurityReviewRequired bool            `json:"security_review_required"`
	Synthesis              bool            `json:"synthesis"`
	FanOutCap              int             `json:"fan_out_cap"`
	EffortDownshifted      bool            `json:"effort_downshifted"`
	ContextIntegrity       string          `json:"context_integrity"`
}

type workflowBindingOptions struct {
	quality        string
	riskTier       string
	filesFile      string
	format         string
	fullDepthAudit bool
	rolloutReceipt string
	context        workflowBindingContextOptions
}

type bindingRiskResolution struct {
	tier   reviewRiskTier
	reason string
}

// newWorkflowBindingCmd resolves the complete route-team quality binding before
// dispatch. It always emits a usable receipt and fails closed to full Ultra for
// untrusted or incomplete evidence.
func newWorkflowBindingCmd(discover func() ([]string, error)) *cobra.Command {
	opts := workflowBindingOptions{}
	cmd := &cobra.Command{
		Use:           "binding",
		Short:         "Resolve the pre-dispatch route-team quality binding",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			receipt := resolveWorkflowBinding(opts, discover)
			data, err := json.Marshal(receipt)
			if err != nil {
				return fmt.Errorf("workflow binding: encode receipt: %w", err)
			}
			fmt.Fprintln(cmd.OutOrStdout(), string(data))
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.quality, "quality", "balanced", "Quality mode (ultra|balanced)")
	cmd.Flags().StringVar(&opts.riskTier, "risk-tier", "auto", "Risk tier (auto|low|medium|high|critical|unknown)")
	cmd.Flags().StringVar(&opts.filesFile, "files-file", "", "JSON file containing changed paths")
	cmd.Flags().StringVar(&opts.format, "format", "json", "Output format (json)")
	cmd.Flags().BoolVar(&opts.fullDepthAudit, "full-depth-audit", false, "Force the canonical full Ultra review profile")
	cmd.Flags().StringVar(&opts.rolloutReceipt, "rollout-receipt", "", "Verified canary rollout receipt JSON")
	registerWorkflowBindingContextFlags(cmd, &opts.context)
	return cmd
}

func resolveWorkflowBinding(opts workflowBindingOptions, discover func() ([]string, error)) workflowBindingReceipt {
	risk := resolveBindingRisk(opts.riskTier, opts.filesFile, discover)
	quality := strings.ToLower(strings.TrimSpace(opts.quality))

	binding := resolveTeamQualityBinding("ultra", "")
	reason := risk.reason
	canary := verifiedCanaryReceipt(opts.rolloutReceipt, risk.tier)
	contextIntegrity := verifyWorkflowContextManifest(opts.context)
	switch {
	case quality == "balanced":
		binding = resolveTeamQualityBinding("balanced", "")
		reason = bindingReasonBalanced
	case quality != "ultra":
		reason = bindingReasonValidationFailed
	case opts.fullDepthAudit:
		reason = bindingReasonAudit
	case risk.reason != "":
		// Risk evidence failures retain the canonical full Ultra binding.
	case (risk.tier == reviewRiskTierLow || risk.tier == reviewRiskTierMedium) && canary && contextIntegrity == contextIntegrityVerified:
		binding = compactUltraQualityBinding()
		reason = bindingReasonCompact
	case (risk.tier == reviewRiskTierLow || risk.tier == reviewRiskTierMedium) && canary && contextIntegrity == contextIntegrityMissing:
		reason = bindingReasonContextMissing
	case (risk.tier == reviewRiskTierLow || risk.tier == reviewRiskTierMedium) && canary:
		reason = bindingReasonContextFailed
	case risk.tier == reviewRiskTierLow || risk.tier == reviewRiskTierMedium:
		reason = bindingReasonShadow
	default:
		reason = bindingReasonRiskFull
	}

	if err := validateWorkflowQualityBinding(binding); err != nil {
		binding = resolveTeamQualityBinding("ultra", "")
		reason = bindingReasonValidationFailed
	}
	serialized, err := serializeTeamQualityBinding(binding)
	if err != nil {
		serialized, _ = serializeTeamQualityBinding(resolveTeamQualityBinding("ultra", ""))
		reason = bindingReasonValidationFailed
	}
	review := binding.Phases["review"]
	return workflowBindingReceipt{
		Quality:                json.RawMessage(serialized),
		Risk:                   bindingRiskName(risk.tier),
		SelectionReason:        reason,
		ReviewVotes:            review.VerifyVotes,
		SecurityReviewRequired: true,
		Synthesis:              review.Synthesis,
		FanOutCap:              workflow.MaxFanOut,
		EffortDownshifted:      false,
		ContextIntegrity:       contextIntegrity,
	}
}

func resolveBindingRisk(value, filesFile string, discover func() ([]string, error)) bindingRiskResolution {
	normalized := strings.ToLower(strings.TrimSpace(value))
	if normalized == "unknown" {
		return bindingRiskResolution{reason: bindingReasonUnknown}
	}
	tier, err := parseReviewRiskTier(normalized)
	if err != nil {
		return bindingRiskResolution{reason: bindingReasonMalformed}
	}
	if tier != reviewRiskTierAuto {
		return bindingRiskResolution{tier: tier}
	}

	var files []string
	if strings.TrimSpace(filesFile) != "" {
		files, err = readBindingFiles(filesFile)
		if err != nil {
			return bindingRiskResolution{reason: bindingReasonMalformed}
		}
	} else {
		if discover == nil {
			discover = discoverChangedFilesForRiskTier
		}
		files, err = discover()
		if err != nil {
			return bindingRiskResolution{reason: bindingReasonDiscoveryFailed}
		}
	}
	files = normalizeRiskTierFiles(files)
	if len(files) == 0 {
		return bindingRiskResolution{reason: bindingReasonMissing}
	}
	return bindingRiskResolution{tier: inferReviewRiskTier(files)}
}

func readBindingFiles(path string) ([]string, error) {
	data, err := readBoundedJSONFile(path)
	if err != nil {
		return nil, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var files []string
	if err := dec.Decode(&files); err != nil {
		return nil, err
	}
	if err := ensureBindingJSONEOF(dec); err != nil {
		return nil, err
	}
	if len(files) > maxBindingFiles {
		return nil, fmt.Errorf("too many changed paths")
	}
	for _, file := range files {
		if len(file) > maxBindingPathBytes {
			return nil, fmt.Errorf("changed path too long")
		}
	}
	return files, nil
}

func readBoundedJSONFile(path string) ([]byte, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxBindingJSONBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxBindingJSONBytes {
		return nil, fmt.Errorf("JSON input exceeds size limit")
	}
	return data, nil
}

func ensureBindingJSONEOF(dec *json.Decoder) error {
	var extra any
	err := dec.Decode(&extra)
	if err == io.EOF {
		return nil
	}
	if err == nil {
		return fmt.Errorf("multiple JSON values")
	}
	return err
}

func verifiedCanaryReceipt(path string, risk reviewRiskTier) bool {
	if path == "" || risk != reviewRiskTierLow && risk != reviewRiskTierMedium {
		return false
	}
	data, err := readBoundedJSONFile(path)
	if err != nil {
		return false
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var receipt experiment.RolloutReceipt
	if dec.Decode(&receipt) != nil || ensureBindingJSONEOF(dec) != nil {
		return false
	}
	return receipt.Version == 1 && receipt.ReceiptKind == "canary" && receipt.Decision == "CANARY" &&
		receipt.ActiveProfile == "compact_ultra" && !receipt.FullDepth && receipt.RiskTier == string(risk) &&
		canonicalReceiptHash(receipt.TaskCorpusHash) && canonicalReceiptHash(receipt.PolicyHash) && canonicalReceiptHash(receipt.ConfigHash)
}

func canonicalReceiptHash(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != 71 {
		return false
	}
	for _, char := range strings.TrimPrefix(value, "sha256:") {
		if !strings.ContainsRune("0123456789abcdefABCDEF", char) {
			return false
		}
	}
	return true
}

func compactUltraQualityBinding() workflow.QualityBinding {
	canonical := resolveTeamQualityBinding("ultra", "")
	phases := make(map[string]workflow.PhaseBinding, len(canonical.Phases))
	for phase, binding := range canonical.Phases {
		phases[phase] = binding
	}
	review := phases["review"]
	review.VerifyVotes = 1
	review.Synthesis = false
	phases["review"] = review
	return workflow.QualityBinding{Phases: phases}
}

func validateWorkflowQualityBinding(binding workflow.QualityBinding) error {
	for phase := range teamPhaseRoles {
		value, ok := binding.Phases[phase]
		if !ok || value.Model == "" || value.Effort == "" {
			return fmt.Errorf("invalid phase binding %q", phase)
		}
	}
	implementation := binding.Phases["implementation"]
	review := binding.Phases["review"]
	if implementation.FanOutCap != workflow.MaxFanOut || review.VerifyVotes < 1 || review.VerifyVotes > workflow.MaxVerifyVotes {
		return fmt.Errorf("invalid depth binding")
	}
	return nil
}

func bindingRiskName(tier reviewRiskTier) string {
	if tier == "" || tier == reviewRiskTierAuto {
		return "unknown"
	}
	return string(tier)
}
