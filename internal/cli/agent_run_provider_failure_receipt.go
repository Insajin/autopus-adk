package cli

import "encoding/json"

var providerFailureShapeOrder = []string{
	"top_level_message", "top_level_code", "top_level_status", "top_level_status_code",
	"top_level_error_string", "nested_error_object", "nested_error_message",
	"nested_error_type", "nested_error_code",
}

type providerFailureEventReceipt struct {
	Kind           string   `json:"kind" yaml:"kind"`
	Shape          []string `json:"shape" yaml:"shape"`
	Traits         []string `json:"traits" yaml:"traits"`
	StatusFamilies []string `json:"status_families" yaml:"status_families"`
}

type providerFailureEventState struct {
	shape, traits, statusFamilies map[string]bool
}

type providerFailureObservation struct {
	errorEvent, turnFailedEvent bool
	events                      map[string]*providerFailureEventState
}

func (o *providerFailureObservation) Observe(eventType, eventSubtype string, data []byte) {
	kind := providerFailureEventKind(eventType, eventSubtype)
	if kind == "" {
		return
	}
	if kind == "error" {
		o.errorEvent = true
	} else {
		o.turnFailedEvent = true
	}
	state := o.eventState(kind)
	var top map[string]json.RawMessage
	if json.Unmarshal(data, &top) != nil {
		return
	}
	o.observeStringField(state, top, "message", "top_level_message", true)
	o.observeStringField(state, top, "code", "top_level_code", false)
	o.observeStatusField(state, top, "status", "top_level_status")
	o.observeStatusField(state, top, "status_code", "top_level_status_code")
	if raw, ok := top["error"]; ok {
		var text string
		if json.Unmarshal(raw, &text) == nil {
			state.shape["top_level_error_string"] = true
			addProviderMessageMetadata(state, text)
			return
		}
		var nested map[string]json.RawMessage
		if json.Unmarshal(raw, &nested) == nil {
			state.shape["nested_error_object"] = true
			o.observeStringField(state, nested, "message", "nested_error_message", true)
			o.observeStringField(state, nested, "type", "nested_error_type", false)
			o.observeStringField(state, nested, "code", "nested_error_code", false)
		}
	}
}

func providerFailureEventKind(eventType, eventSubtype string) string {
	if eventType == "error" {
		return "error"
	}
	if eventType == "turn" && eventSubtype == "failed" {
		return "turn_failed"
	}
	return ""
}

func (o *providerFailureObservation) eventState(kind string) *providerFailureEventState {
	if o.events == nil {
		o.events = make(map[string]*providerFailureEventState)
	}
	if o.events[kind] == nil {
		o.events[kind] = &providerFailureEventState{
			shape: make(map[string]bool), traits: make(map[string]bool), statusFamilies: make(map[string]bool),
		}
	}
	return o.events[kind]
}

func (o *providerFailureObservation) observeStringField(state *providerFailureEventState, object map[string]json.RawMessage, field, shape string, message bool) {
	raw, ok := object[field]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return
	}
	state.shape[shape] = true
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return
	}
	if message {
		addProviderMessageMetadata(state, value)
	} else {
		addProviderValueTraits(state, value)
	}
}

func (o *providerFailureObservation) observeStatusField(state *providerFailureEventState, object map[string]json.RawMessage, field, shape string) {
	raw, ok := object[field]
	if !ok || len(raw) == 0 || string(raw) == "null" {
		return
	}
	state.shape[shape] = true
	if code, ok := providerStatusCode(raw); ok {
		addProviderStatusMetadata(state, code)
	}
}

func (o providerFailureObservation) Observed() bool { return o.errorEvent || o.turnFailedEvent }

func (o providerFailureObservation) Kind() string {
	switch {
	case o.errorEvent && o.turnFailedEvent:
		return "error_and_turn_failed"
	case o.errorEvent:
		return "error"
	case o.turnFailedEvent:
		return "turn_failed"
	default:
		return ""
	}
}

func (o providerFailureObservation) Events() []providerFailureEventReceipt {
	var receipts []providerFailureEventReceipt
	for _, kind := range []string{"error", "turn_failed"} {
		state := o.events[kind]
		if state == nil {
			continue
		}
		receipts = append(receipts, providerFailureEventReceipt{
			Kind: kind, Shape: canonicalProviderMetadata(state.shape, providerFailureShapeOrder),
			Traits:         canonicalProviderMetadata(state.traits, providerFailureTraitOrder),
			StatusFamilies: canonicalProviderMetadata(state.statusFamilies, providerFailureStatusOrder),
		})
	}
	return receipts
}

func (o providerFailureObservation) Shape() []string {
	union := make(map[string]bool)
	for _, receipt := range o.Events() {
		for _, shape := range receipt.Shape {
			union[shape] = true
		}
	}
	return canonicalProviderMetadata(union, providerFailureShapeOrder)
}

func canonicalProviderMetadata(values map[string]bool, order []string) []string {
	result := make([]string, 0, len(order))
	for _, value := range order {
		if values[value] {
			result = append(result, value)
		}
	}
	return result
}

func cloneProviderFailureEvents(input []providerFailureEventReceipt) []providerFailureEventReceipt {
	cloned := make([]providerFailureEventReceipt, len(input))
	for i, event := range input {
		cloned[i] = providerFailureEventReceipt{
			Kind: event.Kind, Shape: append([]string{}, event.Shape...),
			Traits:         append([]string{}, event.Traits...),
			StatusFamilies: append([]string{}, event.StatusFamilies...),
		}
	}
	return cloned
}
