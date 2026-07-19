package desktopobserve

import (
	"fmt"
	"sync"
)

type StateBinding struct {
	StateRef    string
	ProviderRef string
	AppRef      string
	WindowRef   string
	Digest      string
}

type stateRecord struct {
	binding StateBinding
	active  bool
}

type StateLedger struct {
	mu      sync.Mutex
	records map[string]stateRecord
	active  map[string]string
}

func NewStateLedger() *StateLedger {
	return &StateLedger{
		records: make(map[string]stateRecord),
		active:  make(map[string]string),
	}
}

func (ledger *StateLedger) Register(binding StateBinding) error {
	if ledger == nil || !validStateBinding(binding) {
		return reasonError{code: ReasonStaleState}
	}
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	if _, exists := ledger.records[binding.StateRef]; exists {
		return reasonError{code: ReasonStaleState}
	}
	scope := binding.scopeKey()
	if previousRef, exists := ledger.active[scope]; exists {
		previous := ledger.records[previousRef]
		previous.active = false
		ledger.records[previousRef] = previous
	}
	ledger.records[binding.StateRef] = stateRecord{binding: binding, active: true}
	ledger.active[scope] = binding.StateRef
	return nil
}

func (ledger *StateLedger) Consume(binding StateBinding) error {
	if ledger == nil || !validStateBinding(binding) {
		return reasonError{code: ReasonStaleState}
	}
	ledger.mu.Lock()
	defer ledger.mu.Unlock()
	record, exists := ledger.records[binding.StateRef]
	if !exists || !record.active || record.binding != binding ||
		ledger.active[binding.scopeKey()] != binding.StateRef {
		return reasonError{code: ReasonStaleState}
	}
	record.active = false
	ledger.records[binding.StateRef] = record
	delete(ledger.active, binding.scopeKey())
	return nil
}

func (binding StateBinding) scopeKey() string {
	return fmt.Sprintf("%s\x00%s\x00%s", binding.ProviderRef, binding.AppRef, binding.WindowRef)
}

func validStateBinding(binding StateBinding) bool {
	return safePublicRef(binding.StateRef) && safePublicRef(binding.ProviderRef) &&
		safePublicRef(binding.AppRef) && safePublicRef(binding.WindowRef) && binding.Digest != ""
}
