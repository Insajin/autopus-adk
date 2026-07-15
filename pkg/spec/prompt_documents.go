package spec

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
)

var requiredReviewDocumentNames = []string{"spec.md", "plan.md", "research.md", "acceptance.md"}

const maxCompleteReviewPromptTokens = 128 * 1024

type completeReviewDocument struct {
	name               string
	sourceRef          string
	content            string
	sourceHash         string
	promptHash         string
	redactionStatus    string
	invalidationReason string
	specIdentity       *SpecDocument
}

// BuildReviewPromptChecked validates and freezes required review documents
// before it returns a prompt that is safe to dispatch to a provider.
func BuildReviewPromptChecked(doc *SpecDocument, codeContext string, opts ReviewPromptOptions, specDir ...string) (string, error) {
	dir := resolveReviewSpecDir(opts, specDir)
	if opts.RequireCompleteDocuments {
		documents, err := loadCompleteReviewDocuments(dir)
		if err != nil {
			return "", err
		}
		frozenDoc, err := freezeCompleteReviewSpec(doc, documents)
		if err != nil {
			return "", err
		}
		doc = frozenDoc
		opts.SpecDir = dir
		opts.completeDocuments = documents
	}
	return buildReviewPromptWithAdmission(doc, codeContext, opts)
}

func buildReviewPromptWithAdmission(doc *SpecDocument, codeContext string, opts ReviewPromptOptions) (string, error) {
	prompt := BuildReviewPrompt(doc, codeContext, opts)
	if !opts.RequireCompleteDocuments {
		return prompt, nil
	}
	tokens := (len(prompt) + 3) / 4
	if tokens > maxCompleteReviewPromptTokens {
		return "", fmt.Errorf("complete review prompt is %d tokens, above the %d-token safe admission limit; split the review instead of truncating documents", tokens, maxCompleteReviewPromptTokens)
	}
	return prompt, nil
}

func freezeCompleteReviewSpec(doc *SpecDocument, documents []completeReviewDocument) (*SpecDocument, error) {
	if doc == nil {
		return nil, fmt.Errorf("complete review requires a parsed SpecDocument")
	}
	snapshot, ok := findCompleteReviewDocument(documents, "spec.md")
	if !ok {
		return nil, fmt.Errorf("required review document spec.md is missing from the frozen snapshot")
	}
	if snapshot.specIdentity == nil {
		return nil, fmt.Errorf("frozen spec.md identity is unavailable")
	}
	parsed := *snapshot.specIdentity
	if doc.RawContent != "" && !reviewSourceHashMatches(snapshot.sourceHash, []byte(doc.RawContent)) {
		return nil, fmt.Errorf("spec.md changed after SpecDocument load; reload the SPEC before review")
	}
	if doc.ID != "" && doc.ID != parsed.ID {
		return nil, fmt.Errorf("frozen spec.md identity mismatch: loaded ID %q, current ID %q", doc.ID, parsed.ID)
	}
	if doc.Title != "" && doc.Title != parsed.Title {
		return nil, fmt.Errorf("frozen spec.md identity mismatch: loaded title %q, current title %q", doc.Title, parsed.Title)
	}
	return &parsed, nil
}

func resolveReviewSpecDir(opts ReviewPromptOptions, specDir []string) string {
	dir := opts.SpecDir
	if len(specDir) > 0 && strings.TrimSpace(specDir[0]) != "" {
		dir = specDir[0]
	}
	return strings.TrimSpace(dir)
}

func loadCompleteReviewDocuments(specDir string) ([]completeReviewDocument, error) {
	return loadCompleteReviewDocumentsWithHook(specDir, nil)
}

// loadCompleteReviewDocumentsWithHook provides a deterministic test seam
// between snapshot collection and the second-pass source verification.
func loadCompleteReviewDocumentsWithHook(specDir string, afterFirstPass func(string) error) ([]completeReviewDocument, error) {
	if specDir == "" {
		return nil, fmt.Errorf("complete review documents require a SPEC directory")
	}
	root, err := filepath.Abs(specDir)
	if err != nil {
		return nil, fmt.Errorf("resolve SPEC directory: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("SPEC directory unavailable: %s", specDir)
	}
	documents := make([]completeReviewDocument, 0, len(requiredReviewDocumentNames))
	for _, name := range requiredReviewDocumentNames {
		data, err := readCompleteReviewSource(root, name)
		if err != nil {
			return nil, err
		}
		var specIdentity *SpecDocument
		if name == "spec.md" {
			metadata := ParseSpecMetadata(string(data))
			if metadata.ID == "" {
				return nil, fmt.Errorf("parse frozen spec.md: SPEC ID is missing")
			}
			expectedID := filepath.Base(root)
			if metadata.ID != expectedID {
				return nil, fmt.Errorf("spec.md identity mismatch: directory ID %q, document ID %q", expectedID, metadata.ID)
			}
			specIdentity = &metadata
		}
		sanitized := promptlayer.SanitizeContent(string(data), promptlayer.ContextOptions{
			Required: true, PreserveInjectionEvidence: true,
		})
		documents = append(documents, completeReviewDocument{
			name: name, sourceRef: name, content: sanitized.Content,
			sourceHash: reviewDigest(data), promptHash: reviewDigest([]byte(sanitized.Content)),
			redactionStatus: sanitized.RedactionStatus, invalidationReason: sanitized.InvalidationReason,
			specIdentity: specIdentity,
		})
	}
	if afterFirstPass != nil {
		if err := afterFirstPass(root); err != nil {
			return nil, fmt.Errorf("complete review snapshot test seam: %w", err)
		}
	}
	if err := verifyCompleteReviewSourcesUnchanged(root, documents); err != nil {
		return nil, err
	}
	return documents, nil
}

func readCompleteReviewSource(root, name string) ([]byte, error) {
	path := filepath.Join(root, name)
	fileInfo, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("required review document %s: %w", name, err)
	}
	if fileInfo.Mode()&os.ModeSymlink != 0 || !fileInfo.Mode().IsRegular() {
		return nil, fmt.Errorf("required review document %s must be a regular file", name)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read required review document %s: %w", name, err)
	}
	if strings.TrimSpace(string(data)) == "" {
		return nil, fmt.Errorf("required review document %s is empty", name)
	}
	return data, nil
}

func verifyCompleteReviewSourcesUnchanged(root string, documents []completeReviewDocument) error {
	for _, document := range documents {
		data, err := readCompleteReviewSource(root, document.name)
		if err != nil {
			return err
		}
		if !reviewSourceHashMatches(document.sourceHash, data) {
			return fmt.Errorf("required review document %s changed while complete review snapshot was being built", document.name)
		}
	}
	return nil
}

func injectCompleteAuxDocs(sb *strings.Builder, documents []completeReviewDocument) {
	sectionNames := map[string]string{
		"plan.md":       "### Plan Document",
		"research.md":   "### Research Document",
		"acceptance.md": "### Acceptance Criteria Document",
	}
	for _, document := range documents {
		header, ok := sectionNames[document.name]
		if !ok {
			continue
		}
		sb.WriteString(header)
		sb.WriteString("\n\n")
		injectCompleteReviewDocument(sb, document)
	}
}

func injectCompleteSpecDoc(sb *strings.Builder, documents []completeReviewDocument) bool {
	document, ok := findCompleteReviewDocument(documents, "spec.md")
	if !ok {
		return false
	}
	sb.WriteString("### Full SPEC Document\n\n")
	injectCompleteReviewDocument(sb, document)
	return true
}

func injectCompleteReviewDocument(sb *strings.Builder, document completeReviewDocument) {
	fmt.Fprintf(
		sb,
		"[Review document metadata: source_ref=%s source_sha256=%s prompt_sha256=%s redaction_status=%s invalidation_reason=%s complete=true]\n\n",
		document.sourceRef, document.sourceHash, document.promptHash, document.redactionStatus, document.invalidationReason,
	)
	sb.WriteString(document.content)
	sb.WriteString("\n\n")
}

func reviewDigest(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func reviewSourceHashMatches(expected string, data []byte) bool {
	actual := reviewDigest(data)
	return expected == actual || expected == "sha256:"+actual
}

func findCompleteReviewDocument(documents []completeReviewDocument, name string) (completeReviewDocument, bool) {
	for _, document := range documents {
		if document.name == name {
			return document, true
		}
	}
	return completeReviewDocument{}, false
}
