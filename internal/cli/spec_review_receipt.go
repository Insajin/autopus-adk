package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const specReviewPromotionReceiptSchema = "spec_review_promotion_receipt.v1"

type specReviewPromotionReceipt struct {
	Schema          string   `json:"schema"`
	RunID           string   `json:"run_id"`
	FinishedAt      string   `json:"finished_at"`
	SpecID          string   `json:"spec_id,omitempty"`
	Verdict         string   `json:"verdict,omitempty"`
	AnalysisVerdict string   `json:"analysis_verdict"`
	GateStatus      string   `json:"gate_status"`
	CriticalVeto    bool     `json:"critical_veto"`
	PreviousStatus  string   `json:"previous_status,omitempty"`
	CurrentStatus   string   `json:"current_status,omitempty"`
	StatusChanged   bool     `json:"status_changed"`
	DegradedReasons []string `json:"degraded_reasons"`
	OverrideApplied bool     `json:"override_applied"`
}

type specReviewRuntimeEvidence struct {
	RunID      string
	FinishedAt time.Time
}

func persistSpecReviewPromotionReceipt(specDir string, receipt specReviewPromotionReceipt) (string, error) {
	path := filepath.Join(specDir, "review-receipt.json")
	data, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal SPEC review promotion receipt: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return "", fmt.Errorf("write SPEC review promotion receipt: %w", err)
	}
	return path, nil
}
