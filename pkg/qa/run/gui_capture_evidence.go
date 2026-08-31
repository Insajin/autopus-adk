package run

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/insajin/autopus-adk/pkg/qa/capture"
	"github.com/insajin/autopus-adk/pkg/qa/journey"
)

// maxGuardReceiptBytes bounds the append-only receipt. It holds one short line
// per intercepted navigation or action; a larger file means a runaway journey.
const maxGuardReceiptBytes = 8 << 20

// guardReceipt is the harness-owned enforcement witness, reconstructed from the
// preload's JSONL output.
type guardReceipt struct {
	Navigations    int
	Actions        int
	Requests       int
	OffOrigin      []string
	BlockedActions []string
	// BlockedRequests names requests the route policy aborted. These are the
	// policy working, not a violation, so they are reported and never blocking.
	BlockedRequests []string
}

// Enforced reports whether the guard demonstrably intercepted the journey.
func (r guardReceipt) Enforced() bool {
	return r.Navigations > 0 || r.Actions > 0 || r.Requests > 0 ||
		len(r.OffOrigin) > 0 || len(r.BlockedActions) > 0 || len(r.BlockedRequests) > 0
}

type guardEvent struct {
	T            string `json:"t"`
	Origin       string `json:"origin"`
	Allowed      *bool  `json:"allowed"`
	Method       string `json:"method"`
	Blocked      *bool  `json:"blocked"`
	ResourceType string `json:"resource_type"`
	Reason       string `json:"reason"`
}

func readGuardReceipt(path string) (guardReceipt, error) {
	receipt := guardReceipt{}
	if strings.TrimSpace(path) == "" {
		return receipt, fmt.Errorf("gui_policy_guard.receipt")
	}
	file, err := os.Open(path)
	if err != nil {
		return receipt, fmt.Errorf("gui_policy_guard.receipt")
	}
	defer file.Close()
	scanner := bufio.NewScanner(io.LimitReader(file, maxGuardReceiptBytes))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	offOrigin := map[string]bool{}
	blocked := map[string]bool{}
	requests := map[string]bool{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		var event guardEvent
		if err := json.Unmarshal([]byte(line), &event); err != nil {
			continue

		}
		applyGuardEvent(&receipt, event, offOrigin, blocked, requests)
	}
	receipt.OffOrigin = sortedKeys(offOrigin)
	receipt.BlockedActions = sortedKeys(blocked)
	receipt.BlockedRequests = sortedKeys(requests)
	return receipt, nil
}

func applyGuardEvent(receipt *guardReceipt, event guardEvent, offOrigin, blocked, requests map[string]bool) {
	switch event.T {
	case "goto":
		receipt.Navigations++
		if event.Allowed != nil && !*event.Allowed {
			offOrigin["off_origin:"+orUnknownOrigin(event.Origin)] = true
		}
	case "action":
		receipt.Actions++
		if event.Blocked != nil && *event.Blocked {
			blocked["forbidden_action:"+event.Method] = true
		}
	case "request":
		receipt.Requests++
		if event.Blocked != nil && *event.Blocked {
			requests[event.Reason+":"+orUnknownOrigin(event.Origin)] = true
		}
	}
}

func orUnknownOrigin(origin string) string {
	if strings.TrimSpace(origin) == "" {
		return "unresolved"
	}
	return origin
}

func sortedKeys(values map[string]bool) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// evaluateCapturePolicyEvidence is the capture-enabled replacement for the
// producer-authored journey_graph oracle.
//
// Enforcement and blocked attempts come from the guard receipt, which the
// producer cannot write; route coverage comes from the validated capture index.
// That removes the producer's ability to certify its own policy compliance.
func evaluateCapturePolicyEvidence(pack journey.Pack, result *commandResult) guiPolicyEvaluation {
	eval := guiPolicyEvaluation{}
	receipt, err := readGuardReceipt(result.GUIGuardReceiptPath)
	if err != nil {
		eval.missingEvidence = append(eval.missingEvidence, err.Error())
		return eval
	}
	eval.confirmed = receipt.Enforced()
	eval.blockedAttempts = append(eval.blockedAttempts, receipt.OffOrigin...)
	eval.blockedAttempts = append(eval.blockedAttempts, receipt.BlockedActions...)
	// Route-stopped requests are the declared policy working, not a violation, so
	// they are reported alongside the verdict instead of blocking the journey.
	eval.networkStopped = receipt.BlockedRequests
	index, err := loadCaptureIndexOnce(result)
	if err != nil {
		eval.missingEvidence = append(eval.missingEvidence, "capture_index: "+err.Error())
		return eval
	}
	outside, missing := captureNetworkOriginFindings(pack, result)
	if len(missing) > 0 {
		eval.missingEvidence = append(eval.missingEvidence, missing...)
		return eval
	}
	eval.outsideRequests = outside
	eval.missingScreens, eval.missingActions = evaluateCaptureScreenMatrix(index, pack.GUI.ScreenMatrix)
	if !eval.confirmed {
		eval.missingEvidence = append(eval.missingEvidence, "gui_policy_guard.enforcement")
	}
	return eval
}

// evaluateCaptureScreenMatrix checks the declared screen matrix against the
// per-step evidence in the capture index rather than a producer-authored graph.
func evaluateCaptureScreenMatrix(index capture.Index, rows []journey.GUIScreenMatrixRow) (missingScreens, missingActions []string) {
	if len(rows) == 0 {
		return nil, nil
	}
	actionsByScreen := map[string]map[string]bool{}
	for _, step := range index.Steps {
		key := strings.TrimSpace(step.ScreenRef)
		if key == "" {
			continue
		}
		if actionsByScreen[key] == nil {
			actionsByScreen[key] = map[string]bool{}
		}
		for _, action := range step.Actions {
			actionsByScreen[key][strings.ToLower(strings.TrimSpace(action.API))] = true
		}
	}
	for _, row := range rows {
		key := strings.TrimSpace(row.ID)
		if key == "" {
			key = strings.TrimSpace(row.Path)
		}
		observed, ok := actionsByScreen[key]
		if !ok {
			missingScreens = append(missingScreens, key)
			continue
		}
		for _, required := range row.RequiredActions {
			if !observed[strings.ToLower(strings.TrimSpace(required))] {
				missingActions = append(missingActions, key+":"+required)
			}
		}
	}
	return missingScreens, missingActions
}
