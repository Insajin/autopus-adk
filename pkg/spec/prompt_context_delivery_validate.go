package spec

import (
	"fmt"
	"strings"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

func reviewDeliveryLayers(layers []promptlayer.Layer) (map[string]promptlayer.Layer, error) {
	bySource := make(map[string]promptlayer.Layer, len(layers))
	for _, layer := range layers {
		if layer.SourceRef == "" || strings.TrimSpace(layer.Content) == "" {
			return nil, fmt.Errorf("review context delivery contains an empty body: %s", layer.SourceRef)
		}
		if _, exists := bySource[layer.SourceRef]; exists {
			return nil, fmt.Errorf("review context delivery contains duplicate body: %s", layer.SourceRef)
		}
		bySource[layer.SourceRef] = layer
	}
	return bySource, nil
}

func reviewDeliveryManifestEntries(entries []promptlayer.ManifestEntry) (map[string]promptlayer.ManifestEntry, error) {
	bySource := make(map[string]promptlayer.ManifestEntry, len(entries))
	for _, entry := range entries {
		if entry.SourceRef == "" {
			return nil, fmt.Errorf("review context delivery manifest has an empty source reference")
		}
		if _, exists := bySource[entry.SourceRef]; exists {
			return nil, fmt.Errorf("review context delivery manifest contains duplicate source: %s", entry.SourceRef)
		}
		bySource[entry.SourceRef] = entry
	}
	return bySource, nil
}

func validateReviewDeliveryDocument(
	document promptlayer.ContextDeliveryDocument,
	layer promptlayer.Layer,
	entry promptlayer.ManifestEntry,
) error {
	if !canonicalReviewHash(document.SourceHash) || !canonicalReviewHash(document.PromptHash) {
		return fmt.Errorf("review context delivery has invalid hash metadata: %s", document.SourceRef)
	}
	wantPromptHash := "sha256:" + reviewDigest([]byte(layer.Content))
	if document.PromptHash != wantPromptHash || entry.Hash != strings.TrimPrefix(wantPromptHash, "sha256:") {
		return fmt.Errorf("review context delivery prompt hash mismatch: %s", document.SourceRef)
	}
	if document.Kind != layer.Kind || document.Kind != entry.Kind ||
		document.RedactionStatus != layer.RedactionStatus || document.RedactionStatus != entry.RedactionStatus ||
		document.InvalidationReason != layer.InvalidationReason || document.InvalidationReason != entry.InvalidationReason ||
		document.TokenEstimate != promptlayer.EstimateTokens(layer.Content) || document.TokenEstimate != entry.TokenEstimate {
		return fmt.Errorf("review context delivery metadata mismatch: %s", document.SourceRef)
	}
	return nil
}

func canonicalReviewHash(value string) bool {
	if !strings.HasPrefix(value, "sha256:") || len(value) != len("sha256:")+64 {
		return false
	}
	for _, char := range strings.TrimPrefix(value, "sha256:") {
		if !strings.ContainsRune("0123456789abcdef", char) {
			return false
		}
	}
	return true
}
