package cli

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestEvaluateEvidenceResult_ProviderFailureEventsPersistWithoutRawValues(t *testing.T) {
	var observed providerFailureObservation
	observed.Observe("turn", "failed", []byte(`{"type":"turn.failed","error":{"message":"model gpt-nested-private does not exist NESTED-EVIDENCE-SECRET","code":"model_not_found"}}`))
	observed.Observe("error", "", []byte(`{"type":"error","message":"model gpt-top-private does not exist TOP-EVIDENCE-SECRET","status_code":404}`))
	events := observed.Events()
	res := execResult{
		Status:                       "failed",
		OperationalErrorClass:        providerOperationalClass(events),
		OperationalErrorFingerprint:  "sha256:81eff724aac41f9a749c1f19103199a2356368b76407fb6c1ae738f7ccfdc266",
		OperationalErrorStage:        "process_wait",
		OperationalErrorSignals:      []string{"provider_failure_event"},
		OperationalProviderEventKind: "error_and_turn_failed",
		OperationalProviderEventShape: []string{
			"top_level_message", "top_level_status_code", "nested_error_object", "nested_error_message", "nested_error_code",
		},
		OperationalProviderEvents: events,
	}
	result, err := evaluateEvidenceResult("E01", validEvidenceContext(), res, errors.New("synthetic provider failure"))
	if err == nil {
		t.Fatal("provider failure unexpectedly evaluated as success")
	}
	if !reflect.DeepEqual(result.OperationalProviderEvents, events) {
		t.Fatalf("events=%#v want=%#v", result.OperationalProviderEvents, events)
	}

	runsDir := t.TempDir()
	if err := writeTaskResult(runsDir, result); err != nil {
		t.Fatalf("write task result: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(runsDir, "result.yaml"))
	if err != nil {
		t.Fatalf("read result: %v", err)
	}
	var persisted taskResult
	if err := yaml.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if !reflect.DeepEqual(persisted.OperationalProviderEvents, events) {
		t.Fatalf("persisted events=%#v want=%#v", persisted.OperationalProviderEvents, events)
	}
	text := string(data)
	for _, required := range []string{
		"operational_provider_events:", "kind: error", "kind: turn_failed", "model_access", "http_4xx",
	} {
		if !strings.Contains(text, required) {
			t.Fatalf("result missing %q:\n%s", required, text)
		}
	}
	for _, key := range []string{"    shape:", "    traits:", "    status_families:"} {
		if count := strings.Count(text, key); count != len(events) {
			t.Fatalf("receipt key %q count=%d want=%d:\n%s", key, count, len(events), text)
		}
	}
	for _, forbidden := range []string{
		"gpt-top-private", "gpt-nested-private", "TOP-EVIDENCE-SECRET", "NESTED-EVIDENCE-SECRET",
		"model_not_found", "status_code:", "message:",
	} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("result retained raw provider value %q:\n%s", forbidden, text)
		}
	}
}

func TestEvaluateEvidenceResult_ProviderFailureEventsAreDeepCopied(t *testing.T) {
	events := []providerFailureEventReceipt{{
		Kind: "error", Shape: []string{"top_level_message"},
		Traits: []string{"network_transport"}, StatusFamilies: []string{"http_5xx"},
	}}
	res := execResult{
		Status: "failed", OperationalErrorClass: "network_transport",
		OperationalProviderEvents: events,
	}
	result, _ := evaluateEvidenceResult("E01", validEvidenceContext(), res, errors.New("synthetic provider failure"))
	events[0].Shape[0] = "RAW-SHAPE-MUTATION"
	events[0].Traits[0] = "RAW-TRAIT-MUTATION"
	events[0].StatusFamilies[0] = "RAW-STATUS-MUTATION"
	serialized, err := yaml.Marshal(result)
	if err != nil {
		t.Fatalf("marshal result: %v", err)
	}
	for _, forbidden := range []string{"RAW-SHAPE-MUTATION", "RAW-TRAIT-MUTATION", "RAW-STATUS-MUTATION"} {
		if strings.Contains(string(serialized), forbidden) {
			t.Fatalf("result aliases mutable provider metadata: %s", serialized)
		}
	}
}
