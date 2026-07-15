package spec

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

// BuildReviewPromptFromContextDeliveryChecked builds a complete review prompt
// from an in-memory context delivery that the supervisor already verified.
// It never reloads or duplicates the four SPEC documents.
func BuildReviewPromptFromContextDeliveryChecked(
	doc *SpecDocument,
	codeContext string,
	opts ReviewPromptOptions,
	delivery promptlayer.ContextDeliveryResult,
) (string, error) {
	documents, err := completeReviewDocumentsFromDelivery(delivery)
	if err != nil {
		return "", err
	}
	frozenDoc, err := freezeCompleteReviewSpec(doc, documents)
	if err != nil {
		return "", err
	}
	opts.RequireCompleteDocuments = true
	opts.SpecDir = delivery.SpecDir
	opts.completeDocuments = documents
	return buildReviewPromptWithAdmission(frozenDoc, codeContext, opts)
}

func completeReviewDocumentsFromDelivery(delivery promptlayer.ContextDeliveryResult) ([]completeReviewDocument, error) {
	if delivery.SchemaVersion != promptlayer.ContextDeliverySchemaVersion || delivery.IntegrityStatus != "verified" {
		return nil, fmt.Errorf("review context delivery is not verified")
	}
	if delivery.Command != "review" {
		return nil, fmt.Errorf("review context delivery has wrong command: %s", delivery.Command)
	}
	if strings.TrimSpace(delivery.SpecDir) == "" {
		return nil, fmt.Errorf("review context delivery has no SPEC directory")
	}
	if len(delivery.RequiredDocuments) != len(delivery.Layers) ||
		len(delivery.RequiredDocuments) != len(delivery.PromptManifest.Entries) {
		return nil, fmt.Errorf("review context delivery body and manifest sets disagree")
	}

	layers, err := reviewDeliveryLayers(delivery.Layers)
	if err != nil {
		return nil, err
	}
	entries, err := reviewDeliveryManifestEntries(delivery.PromptManifest.Entries)
	if err != nil {
		return nil, err
	}
	specNames := reviewSpecDocumentRefs(delivery.SpecDir)
	foundSpec := make(map[string]bool, len(requiredReviewDocumentNames))
	documents := make([]completeReviewDocument, 0, len(delivery.RequiredDocuments))
	seen := make(map[string]bool, len(delivery.RequiredDocuments))
	for _, receipt := range delivery.RequiredDocuments {
		if seen[receipt.SourceRef] || !receipt.Complete {
			return nil, fmt.Errorf("review context delivery contains incomplete or duplicate document: %s", receipt.SourceRef)
		}
		seen[receipt.SourceRef] = true
		layer, layerOK := layers[receipt.SourceRef]
		entry, entryOK := entries[receipt.SourceRef]
		if !layerOK || !entryOK {
			return nil, fmt.Errorf("review context delivery body is unavailable: %s", receipt.SourceRef)
		}
		if err := validateReviewDeliveryDocument(receipt, layer, entry); err != nil {
			return nil, err
		}
		name := specNames[receipt.SourceRef]
		document := completeReviewDocument{
			name: name, sourceRef: receipt.SourceRef, content: layer.Content,
			sourceHash: receipt.SourceHash, promptHash: receipt.PromptHash,
			redactionStatus: receipt.RedactionStatus, invalidationReason: receipt.InvalidationReason,
		}
		if name == "spec.md" {
			if err := promptlayer.VerifyContextSpecIdentity(delivery.SpecDir, []byte(layer.Content)); err != nil {
				return nil, err
			}
			identity := ParseSpecMetadata(layer.Content)
			document.specIdentity = &identity
		}
		if name != "" {
			foundSpec[name] = true
		}
		documents = append(documents, document)
	}
	for _, name := range requiredReviewDocumentNames {
		if !foundSpec[name] {
			return nil, fmt.Errorf("review context delivery is missing required SPEC document: %s", name)
		}
	}
	return documents, nil
}

func reviewSpecDocumentRefs(specDir string) map[string]string {
	refs := make(map[string]string, len(requiredReviewDocumentNames))
	for _, name := range requiredReviewDocumentNames {
		ref := filepath.ToSlash(filepath.Join(filepath.FromSlash(specDir), name))
		refs[ref] = name
	}
	return refs
}

func injectCompleteContextDocs(sb *strings.Builder, documents []completeReviewDocument) {
	for _, document := range documents {
		if document.name != "" {
			continue
		}
		fmt.Fprintf(sb, "### Required Context Document: `%s`\n\n", document.sourceRef)
		injectCompleteReviewDocument(sb, document)
	}
}
