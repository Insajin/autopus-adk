package design

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type importMetadata struct {
	Version          int      `json:"version"`
	SourceURLHash    string   `json:"source_url_hash"`
	ImportedAt       string   `json:"imported_at"`
	TrustLabel       string   `json:"trust_label"`
	Promoted         bool     `json:"promoted"`
	SanitizerVersion string   `json:"sanitizer_version"`
	ContentHash      string   `json:"content_hash,omitempty"`
	RedactionCount   int      `json:"redaction_count"`
	RejectionReasons []string `json:"rejection_reasons,omitempty"`
}

// @AX:ANCHOR [AUTO]: External design import entry point; CLI and tests depend on sanitized artifact semantics.
// @AX:REASON: This is the only path that fetches remote design context, writes runtime artifacts, and records trust metadata.
func ImportURL(ctx context.Context, root, rawURL string, opts ImportOptions) (ImportResult, error) {
	now := opts.Now
	if now.IsZero() {
		now = time.Now().UTC()
	}
	trust := opts.TrustLabel
	if trust == "" {
		trust = "external-reference"
	}
	artifactDir := filepath.Join(root, ".autopus", "design", "imports", importID(rawURL, now))
	result := ImportResult{ArtifactDir: artifactDir}
	if err := os.MkdirAll(artifactDir, 0o755); err != nil {
		return result, err
	}

	body, reasons, err := fetchPublicHTTPS(ctx, rawURL, opts)
	if err != nil {
		return result, err
	}
	if len(reasons) > 0 {
		result.Rejected = true
		result.Reasons = reasons
		return result, writeMetadata(artifactDir, rawURL, now, trust, "", 0, reasons)
	}

	sanitized := sanitizeImportContent(string(body))
	if sanitized.Rejected {
		result.Rejected = true
		result.Reasons = sanitized.Reasons
		return result, writeMetadata(artifactDir, rawURL, now, trust, "", 0, sanitized.Reasons)
	}
	contentHash := hashString(sanitized.Content)
	if err := os.WriteFile(filepath.Join(artifactDir, "content.md"), []byte(sanitized.Content), 0o644); err != nil {
		return result, err
	}
	if err := writeMetadata(artifactDir, rawURL, now, trust, contentHash, sanitized.Redactions, nil); err != nil {
		return result, err
	}
	return result, nil
}

func writeMetadata(dir, rawURL string, now time.Time, trust, contentHash string, redactions int, reasons []string) error {
	metadata := importMetadata{
		Version:          1,
		SourceURLHash:    hashString(rawURL),
		ImportedAt:       now.UTC().Format(time.RFC3339),
		TrustLabel:       trust,
		Promoted:         false,
		SanitizerVersion: sanitizerVersion,
		ContentHash:      contentHash,
		RedactionCount:   redactions,
		RejectionReasons: reasons,
	}
	data, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(dir, "metadata.json"), data, 0o644)
}

func importID(rawURL string, now time.Time) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s\n%s", rawURL, now.UTC().Format(time.RFC3339Nano))))
	return hex.EncodeToString(sum[:])[:16]
}

func hashString(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}
