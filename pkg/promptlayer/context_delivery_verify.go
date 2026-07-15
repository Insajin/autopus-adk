package promptlayer

import (
	"fmt"
	"reflect"
)

// VerifyContextDelivery rejects incomplete, tampered, stale, or
// profile-weakened receipts. Callers that own task-specific references should
// use VerifyContextDeliveryForOptions so the receipt cannot define its own
// expected reference set.
func VerifyContextDelivery(root string, receipt ContextDeliveryResult) error {
	refs := make([]string, 0, len(receipt.RequiredDocuments))
	for _, document := range receipt.RequiredDocuments {
		refs = append(refs, document.SourceRef)
	}
	return VerifyContextDeliveryForOptions(ContextDeliveryOptions{
		Root: root, Command: receipt.Command, SpecDir: receipt.SpecDir,
		RequiredReferences: refs,
	}, receipt)
}

// VerifyContextDeliveryForOptions rebuilds the snapshot from a
// supervisor-held command, SPEC, conditional-profile, and required-reference
// set. A valid receipt that omits or replays a task-specific document fails.
func VerifyContextDeliveryForOptions(opts ContextDeliveryOptions, receipt ContextDeliveryResult) error {
	if receipt.SchemaVersion != ContextDeliverySchemaVersion || receipt.IntegrityStatus != "verified" {
		return fmt.Errorf("context delivery is not verified")
	}
	seen := make(map[string]bool, len(receipt.RequiredDocuments))
	for _, document := range receipt.RequiredDocuments {
		if !document.Complete || seen[document.SourceRef] {
			return fmt.Errorf("context delivery contains incomplete or duplicate document: %s", document.SourceRef)
		}
		seen[document.SourceRef] = true
	}
	rebuilt, err := BuildContextDelivery(opts)
	if err != nil {
		return fmt.Errorf("context integrity failed: %w", err)
	}
	if receipt.Command != rebuilt.Command || receipt.SpecDir != rebuilt.SpecDir ||
		receipt.SnapshotHash != rebuilt.SnapshotHash ||
		receipt.PromptManifestHash != rebuilt.PromptManifestHash ||
		receipt.RequiredTokenEstimate != rebuilt.RequiredTokenEstimate ||
		!reflect.DeepEqual(receipt.RequiredDocuments, rebuilt.RequiredDocuments) ||
		!reflect.DeepEqual(receipt.PromptManifest, rebuilt.PromptManifest) {
		return fmt.Errorf("context integrity failed: manifest is stale, replayed, incomplete, or tampered")
	}
	return nil
}
