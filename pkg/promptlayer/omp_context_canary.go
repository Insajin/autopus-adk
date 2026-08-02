package promptlayer

const OMPContextCanarySchemaV1 = "autopus.omp_context_canary.v1"

type OMPContextCanaryVariantV1 string

const (
	OMPContextCanaryVariantFullV1      OMPContextCanaryVariantV1 = "A"
	OMPContextCanaryVariantOptimizedV1 OMPContextCanaryVariantV1 = "B"
)

type OMPContextHistoryModeV1 string

const (
	OMPContextHistoryModeOffV1    OMPContextHistoryModeV1 = "off"
	OMPContextHistoryModeShadowV1 OMPContextHistoryModeV1 = "shadow"
	OMPContextHistoryModeActiveV1 OMPContextHistoryModeV1 = "active"
)

type OMPContextMemoryModeV1 string

const (
	OMPContextMemoryModeOffV1    OMPContextMemoryModeV1 = "off"
	OMPContextMemoryModeShadowV1 OMPContextMemoryModeV1 = "shadow"
)

const (
	OMPContextPromotionReasonPassedV1                = "promotion-gates-passed"
	OMPContextPromotionReasonIntegrityMismatchV1     = "integrity-mismatch"
	OMPContextPromotionReasonSecurityFindingV1       = "security-finding"
	OMPContextPromotionReasonQualityRegressionV1     = "quality-regression"
	OMPContextPromotionReasonInsufficientPairCountV1 = "insufficient-pair-count"
	OMPContextPromotionReasonUnbalancedOrderV1       = "unbalanced-order"
	OMPContextPromotionReasonInsufficientReductionV1 = "insufficient-token-reduction"
	OMPContextPromotionReasonFallbackUnverifiedV1    = "fallback-unverified"
	OMPContextPromotionReasonRollbackUnverifiedV1    = "rollback-unverified"
	OMPContextPromotionReasonEvidenceUnverifiableV1  = "promotion-evidence-unverifiable"
	OMPContextPromotionReasonInvalidModeV1           = "invalid-mode"
)

const (
	OMPContextRollbackReasonQualityRegressionV1          = "quality-regression"
	OMPContextRollbackReasonIntegrityRegressionV1        = "integrity-regression"
	OMPContextRollbackReasonSecurityRegressionV1         = "security-regression"
	OMPContextRollbackReasonOperatorRequestedV1          = "operator-requested"
	OMPContextRollbackReasonReadbackMismatchV1           = "effective-readback-mismatch"
	OMPContextRollbackReasonOverlayUnchangedV1           = "rollback-overlay-unchanged"
	OMPContextRollbackReasonUserConfigMutatedV1          = "user-config-mutated"
	OMPContextRollbackReasonMemoryChangedV1              = "memory-mode-changed"
	OMPContextRollbackReasonInvalidEvidenceV1            = "invalid-rollback-evidence"
	OMPContextRollbackReasonCanonicalRestartUnverifiedV1 = "canonical-restart-unverified"
)

type OMPContextCanaryRowV1 struct {
	TaskID           string                    `json:"task_id"`
	Variant          OMPContextCanaryVariantV1 `json:"variant"`
	Order            int                       `json:"order"`
	Tokens           int64                     `json:"tokens"`
	IntegrityPassed  bool                      `json:"integrity_passed"`
	SecurityPassed   bool                      `json:"security_passed"`
	QualityScore     int64                     `json:"quality_score"`
	FallbackVerified bool                      `json:"fallback_verified"`
	RollbackVerified bool                      `json:"rollback_verified"`
}

type OMPContextCanaryPairV1 struct {
	TaskID               string `json:"task_id"`
	Order                string `json:"order"`
	FullTokens           int64  `json:"full_tokens"`
	OptimizedTokens      int64  `json:"optimized_tokens"`
	ReductionBasisPoints int64  `json:"reduction_basis_points"`
	ReductionPercent     string `json:"reduction_percent"`
	IntegrityPassed      bool   `json:"integrity_passed"`
	SecurityPassed       bool   `json:"security_passed"`
	QualityDelta         int64  `json:"quality_delta"`
	FallbackVerified     bool   `json:"fallback_verified"`
	RollbackVerified     bool   `json:"rollback_verified"`
}

type OMPContextCanaryAggregateV1 struct {
	SchemaVersion              string                   `json:"schema_version"`
	PairKeys                   []string                 `json:"pair_keys"`
	Pairs                      []OMPContextCanaryPairV1 `json:"pairs"`
	PairCount                  int                      `json:"pair_count"`
	ABCount                    int                      `json:"ab_count"`
	BACount                    int                      `json:"ba_count"`
	BalancedOrder              bool                     `json:"balanced_order"`
	UnpairedRows               int                      `json:"unpaired_rows"`
	DuplicateRows              int                      `json:"duplicate_rows"`
	InvalidRows                int                      `json:"invalid_rows"`
	MedianReductionBasisPoints int64                    `json:"median_reduction_basis_points"`
	MedianReductionPercent     string                   `json:"median_reduction_percent"`
	IntegrityFailures          int                      `json:"integrity_failures"`
	SecurityFailures           int                      `json:"security_failures"`
	QualityRegressions         int                      `json:"quality_regressions"`
	FallbackVerified           bool                     `json:"fallback_verified"`
	RollbackVerified           bool                     `json:"rollback_verified"`
}

type OMPContextPromotionGateV1 struct {
	ID     string `json:"id"`
	Passed bool   `json:"passed"`
	Reason string `json:"reason"`
}

type OMPContextHistoryPromotionInputV1 struct {
	RequestedHistoryMode OMPContextHistoryModeV1
	PreviousHistoryMode  OMPContextHistoryModeV1
	MemoryMode           OMPContextMemoryModeV1
	Rows                 []OMPContextCanaryRowV1
	// Aggregate is retained for source compatibility but never trusted.
	Aggregate OMPContextCanaryAggregateV1
}

type OMPContextHistoryPromotionDecisionV1 struct {
	RequestedHistoryMode OMPContextHistoryModeV1     `json:"requested_history_mode"`
	EffectiveHistoryMode OMPContextHistoryModeV1     `json:"effective_history_mode"`
	PreviousHistoryMode  OMPContextHistoryModeV1     `json:"previous_history_mode"`
	RequestedMemoryMode  OMPContextMemoryModeV1      `json:"requested_memory_mode"`
	EffectiveMemoryMode  OMPContextMemoryModeV1      `json:"effective_memory_mode"`
	Admitted             bool                        `json:"admitted"`
	Reason               string                      `json:"reason"`
	Gates                []OMPContextPromotionGateV1 `json:"gates"`
}

type OMPContextRollbackInputV1 struct {
	RequestedHistoryMode  OMPContextHistoryModeV1
	PreviousHistoryMode   OMPContextHistoryModeV1
	MemoryModeBefore      OMPContextMemoryModeV1
	MemoryModeAfter       OMPContextMemoryModeV1
	ActiveOverlayHash     string
	RollbackOverlayHash   string
	EffectiveReadbackHash string
	UserConfigBeforeHash  string
	UserConfigAfterHash   string
	TriggerReason         string
}

type OMPContextRollbackReceiptV1 struct {
	RequestedHistoryMode    OMPContextHistoryModeV1 `json:"requested_history_mode"`
	EffectiveHistoryMode    OMPContextHistoryModeV1 `json:"effective_history_mode"`
	PreviousHistoryMode     OMPContextHistoryModeV1 `json:"previous_history_mode"`
	EffectiveMemoryMode     OMPContextMemoryModeV1  `json:"effective_memory_mode"`
	ActiveOverlayHash       string                  `json:"active_overlay_hash"`
	RollbackOverlayHash     string                  `json:"rollback_overlay_hash"`
	EffectiveReadbackHash   string                  `json:"effective_readback_hash"`
	ReadbackVerified        bool                    `json:"readback_verified"`
	CanonicalFullDelivery   bool                    `json:"canonical_full_delivery"`
	OptimizedStateContinued bool                    `json:"optimized_state_continued"`
	Admitted                bool                    `json:"admitted"`
	Reason                  string                  `json:"reason"`
}
