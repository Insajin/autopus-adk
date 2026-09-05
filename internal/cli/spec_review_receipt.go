package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/spec"
)

const specReviewPromotionReceiptSchema = "spec_review_promotion_receipt.v1"

type specReviewJudgeReceipt struct {
	Provider    string   `json:"provider"`
	Family      string   `json:"family"`
	Status      string   `json:"status"`
	Verdict     string   `json:"verdict"`
	Accepted    int      `json:"accepted"`
	Rejected    int      `json:"rejected"`
	Merged      int      `json:"merged"`
	AcceptedIDs []string `json:"accepted_ids"`
	Rationale   string   `json:"rationale"`
	Reason      string   `json:"reason"`
}

type specReviewPromotionReceipt struct {
	Schema          string                  `json:"schema"`
	RunID           string                  `json:"run_id"`
	FinishedAt      string                  `json:"finished_at"`
	SpecID          string                  `json:"spec_id,omitempty"`
	Verdict         string                  `json:"verdict,omitempty"`
	AnalysisVerdict string                  `json:"analysis_verdict"`
	GateStatus      string                  `json:"gate_status"`
	CriticalVeto    bool                    `json:"critical_veto"`
	PreviousStatus  string                  `json:"previous_status,omitempty"`
	CurrentStatus   string                  `json:"current_status,omitempty"`
	StatusChanged   bool                    `json:"status_changed"`
	DegradedReasons []string                `json:"degraded_reasons"`
	OverrideApplied bool                    `json:"override_applied"`
	Providers       []spec.ProviderStatus   `json:"providers,omitempty"`
	Judge           *specReviewJudgeReceipt `json:"judge,omitempty"`
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

func truncateSpecReviewJudgeRationale(value string) string {
	const maxRunes = 500
	value = strings.TrimSpace(value)
	count := 0
	for index := range value {
		if count == maxRunes {
			return value[:index]
		}
		count++
	}
	return value
}
