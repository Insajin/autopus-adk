package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func TestProviderFailureObservation_PerEventTraitsAreCanonicalAndRawFree(t *testing.T) {
	t.Parallel()
	var observed providerFailureObservation
	observed.Observe("turn", "failed", []byte(`{"type":"turn.failed","error":{"message":"model gpt-private does not exist NESTED-SECRET","type":"model_not_found","code":"unsupported_model"}}`))
	observed.Observe("error", "", []byte(`{"type":"error","message":"invalid token TOP-SECRET","status_code":401}`))
	observed.Observe("error", "", []byte(`{"type":"error","message":"invalid token TOP-SECRET","status_code":401}`))

	want := []providerFailureEventReceipt{
		{
			Kind:           "error",
			Shape:          []string{"top_level_message", "top_level_status_code"},
			Traits:         []string{"authentication"},
			StatusFamilies: []string{"http_4xx"},
		},
		{
			Kind:           "turn_failed",
			Shape:          []string{"nested_error_object", "nested_error_message", "nested_error_type", "nested_error_code"},
			Traits:         []string{"model_access"},
			StatusFamilies: []string{},
		},
	}
	got := observed.Events()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("events=%#v want=%#v", got, want)
	}
	assertProviderReceiptsExclude(t, got,
		"gpt-private", "TOP-SECRET", "NESTED-SECRET", "invalid token",
		"model_not_found", "unsupported_model", "401")
}

func TestProviderFailureObservation_MessageTraitAllowlist(t *testing.T) {
	t.Parallel()
	tests := []struct {
		trait   string
		message string
	}{
		{"authentication", "invalid token AUTH-SECRET"},
		{"authorization_or_entitlement", "permission denied ENTITLEMENT-SECRET"},
		{"model_access", "model gpt-private does not exist MODEL-SECRET"},
		{"rate_limit_or_quota", "too many requests RATE-SECRET"},
		{"provider_unavailable", "service unavailable PROVIDER-SECRET"},
		{"network_transport", "stream disconnected before completion NETWORK-SECRET"},
		{"request_validation", "invalid request REQUEST-SECRET"},
		{"schema_or_response", "response format contains invalid json SCHEMA-SECRET"},
	}
	for _, tc := range tests {
		t.Run(tc.trait, func(t *testing.T) {
			var observed providerFailureObservation
			payload := fmt.Sprintf(`{"type":"error","message":%q}`, tc.message)
			observed.Observe("error", "", []byte(payload))
			got := observed.Events()
			if len(got) != 1 {
				t.Fatalf("events=%#v want one event", got)
			}
			if !reflect.DeepEqual(got[0].Traits, []string{tc.trait}) {
				t.Fatalf("traits=%v want=%q", got[0].Traits, tc.trait)
			}
			assertProviderReceiptsExclude(t, got, tc.message)
		})
	}
}

func TestProviderFailureObservation_TraitUnionIsCanonicalAndDeduplicated(t *testing.T) {
	t.Parallel()
	inputs := []string{
		"response format contains invalid json",
		"invalid request",
		"stream disconnected before completion",
		"service unavailable",
		"too many requests",
		"model gpt-private does not exist",
		"permission denied",
		"invalid token",
	}
	var observed providerFailureObservation
	for _, message := range inputs {
		payload := fmt.Sprintf(`{"type":"error","message":%q}`, message)
		observed.Observe("error", "", []byte(payload))
		observed.Observe("error", "", []byte(payload))
	}
	got := observed.Events()
	if len(got) != 1 {
		t.Fatalf("events=%#v want one canonical event", got)
	}
	want := []string{
		"authentication", "authorization_or_entitlement", "model_access", "rate_limit_or_quota",
		"provider_unavailable", "network_transport", "request_validation", "schema_or_response",
	}
	if !reflect.DeepEqual(got[0].Traits, want) {
		t.Fatalf("traits=%v want=%v", got[0].Traits, want)
	}
}

func TestProviderFailureObservation_StatusFamilyRequiresStructuredOrContextualHTTP(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		payload string
		want    []string
	}{
		{"structured status code 4xx", `{"type":"error","status_code":429}`, []string{"http_4xx"}},
		{"structured status 5xx", `{"type":"error","status":503}`, []string{"http_5xx"}},
		{"contextual HTTP 4xx", `{"type":"error","message":"HTTP 403 CONTEXT-SECRET"}`, []string{"http_4xx"}},
		{"contextual status 5xx", `{"type":"error","message":"status 503 CONTEXT-SECRET"}`, []string{"http_5xx"}},
		{"unrelated number", `{"type":"error","message":"build 14019 OPAQUE-SECRET"}`, []string{}},
		{"bare status-like number", `{"type":"error","message":"failure 503 OPAQUE-SECRET"}`, []string{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var observed providerFailureObservation
			observed.Observe("error", "", []byte(tc.payload))
			got := observed.Events()
			if len(got) != 1 {
				t.Fatalf("events=%#v want one event", got)
			}
			if !reflect.DeepEqual(got[0].StatusFamilies, tc.want) {
				t.Fatalf("status families=%v want=%v", got[0].StatusFamilies, tc.want)
			}
			assertProviderReceiptsExclude(t, got, "403", "503", "429", "14019", "CONTEXT-SECRET", "OPAQUE-SECRET")
		})
	}
}

func TestProviderFailureObservation_StatusFamiliesAreCanonicalAndDeduplicated(t *testing.T) {
	t.Parallel()
	var observed providerFailureObservation
	for _, payload := range []string{
		`{"type":"error","status":503}`,
		`{"type":"error","message":"HTTP 429"}`,
		`{"type":"error","status_code":503}`,
		`{"type":"error","message":"status 429"}`,
	} {
		observed.Observe("error", "", []byte(payload))
	}
	got := observed.Events()
	if len(got) != 1 || !reflect.DeepEqual(got[0].StatusFamilies, []string{"http_4xx", "http_5xx"}) {
		t.Fatalf("events=%#v want canonical status families", got)
	}
}

func TestProviderFailureObservation_OpaqueValuesProduceEmptyCoarseMetadata(t *testing.T) {
	t.Parallel()
	var observed providerFailureObservation
	observed.Observe("error", "", []byte(`{"type":"error","message":"OPAQUE-MESSAGE-SECRET","code":"PRIVATE-CODE","status":"banana"}`))
	got := observed.Events()
	if len(got) != 1 {
		t.Fatalf("events=%#v want one event", got)
	}
	if len(got[0].Traits) != 0 || len(got[0].StatusFamilies) != 0 {
		t.Fatalf("opaque metadata was classified: %#v", got[0])
	}
	assertProviderReceiptsExclude(t, got, "OPAQUE-MESSAGE-SECRET", "PRIVATE-CODE", "banana")
}

func TestProviderOperationalClass_ConflictingTraitsFailClosed(t *testing.T) {
	t.Parallel()
	events := []providerFailureEventReceipt{
		{Kind: "error", Traits: []string{"authentication"}},
		{Kind: "turn_failed", Traits: []string{"network_transport"}},
	}
	if got := providerOperationalClass(events); got != "unknown" {
		t.Fatalf("class=%q want=unknown for conflicting traits", got)
	}
}

func TestProviderOperationalClass_ModelEntitlementModifierIsEventLocal(t *testing.T) {
	t.Parallel()
	combined := []providerFailureEventReceipt{{
		Kind: "error", Traits: []string{"authorization_or_entitlement", "model_access"},
	}}
	if got := providerOperationalClass(combined); got != "model_access" {
		t.Fatalf("combined class=%q want=model_access", got)
	}
	separate := []providerFailureEventReceipt{
		{Kind: "error", Traits: []string{"authorization_or_entitlement"}},
		{Kind: "turn_failed", Traits: []string{"model_access"}},
	}
	if got := providerOperationalClass(separate); got != "unknown" {
		t.Fatalf("separate class=%q want=unknown", got)
	}
}

func TestClassifyOperationalErrorWithProvider_ObservedUnknownDominatesStderr(t *testing.T) {
	t.Parallel()
	events := []providerFailureEventReceipt{{
		Kind: "error", Traits: []string{"authentication", "network_transport"},
	}}
	class, _ := classifyOperationalErrorWithProvider(
		"401 Unauthorized login required", errors.New("exit status 1"), events,
	)
	if class != "unknown" {
		t.Fatalf("class=%q want=unknown for observed provider conflict", class)
	}
	class, _ = classifyOperationalErrorWithProvider(
		"401 Unauthorized login required", errors.New("exit status 1"), nil,
	)
	if class != "authentication" {
		t.Fatalf("class=%q want=authentication without provider evidence", class)
	}
}

func assertProviderReceiptsExclude(t *testing.T, receipts []providerFailureEventReceipt, forbidden ...string) {
	t.Helper()
	serialized, err := json.Marshal(receipts)
	if err != nil {
		t.Fatalf("marshal provider receipts: %v", err)
	}
	for _, value := range forbidden {
		if value != "" && strings.Contains(string(serialized), value) {
			t.Fatalf("provider receipt retained raw value %q: %s", value, serialized)
		}
	}
}
