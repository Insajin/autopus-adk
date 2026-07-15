package worker

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/insajin/autopus-adk/pkg/worker/adapter"
)

const maxRetainedRequiredContextTokens = 128 * 1024

func (wl *WorkerLoop) requiredContextForTask(specID string, requiredReferences []string) (string, error) {
	return wl.requiredContextForTaskAtRoot(wl.config.WorkDir, specID, requiredReferences)
}

func (wl *WorkerLoop) requiredContextForTaskAtRoot(root, specID string, requiredReferences []string) (string, error) {
	if !workerUsesGPTContextDelivery(wl.config.Provider) {
		return "", nil
	}
	specID = strings.TrimSpace(specID)
	root = strings.TrimSpace(root)
	if root == "" {
		root = "."
	}
	command, specDir := "worker", ""
	if specID != "" {
		if filepath.Base(specID) != specID || specID == "." || specID == ".." || strings.ContainsAny(specID, `/\`) {
			return "", fmt.Errorf("invalid local SPEC ID %q", specID)
		}
		command, specDir = "go", filepath.Join(".autopus", "specs", specID)
		info, err := os.Stat(filepath.Join(root, specDir))
		if err != nil {
			return "", fmt.Errorf("required local SPEC %s is unavailable: %w", specID, err)
		}
		if !info.IsDir() {
			return "", fmt.Errorf("local SPEC path is not a directory: %s", specID)
		}
	}

	deliveryOpts := promptlayer.ContextDeliveryOptions{
		Root: root, Command: command, SpecDir: filepath.ToSlash(specDir),
		RequiredReferences: requiredReferences,
	}
	receipt, err := promptlayer.BuildContextDelivery(deliveryOpts)
	if err != nil {
		return "", fmt.Errorf("build required context for %s: %w", contextTaskLabel(specID), err)
	}
	if err := promptlayer.VerifyContextDeliveryForOptions(deliveryOpts, receipt); err != nil {
		return "", fmt.Errorf("verify required context for %s: %w", contextTaskLabel(specID), err)
	}
	if receipt.RequiredTokenEstimate > maxRetainedRequiredContextTokens {
		return "", fmt.Errorf(
			"required context for %s is %d tokens, above the %d-token safe admission limit; split the task instead of truncating documents",
			contextTaskLabel(specID), receipt.RequiredTokenEstimate, maxRetainedRequiredContextTokens,
		)
	}
	return renderRetainedRequiredContext(receipt), nil
}

func (wl *WorkerLoop) resolveTaskRequiredContext(task *adapter.TaskConfig, root string) error {
	if task == nil || !task.ResolveContext {
		return nil
	}
	context, err := wl.requiredContextForTaskAtRoot(root, task.ContextSpecID, task.ContextRefs)
	if err != nil {
		return err
	}
	task.RequiredContext = context
	return nil
}

func contextTaskLabel(specID string) string {
	if specID == "" {
		return "core worker context"
	}
	return specID
}

// persistentTaskContext freezes the backend task contract separately from
// optional knowledge and memory recall so every pipeline phase sees the
// original request even when prior phase output is compressed or replaced.
func (wl *WorkerLoop) persistentTaskContext(taskID string, msg taskPayloadMessage) string {
	prompt := strings.TrimSpace(msg.Prompt)
	if prompt == "" {
		prompt = wl.builder.Build(TaskPayload{
			TaskID:        taskID,
			Description:   msg.Description,
			PMNotes:       msg.PMNotes,
			PolicySummary: msg.PolicySummary,
			SpecID:        msg.SpecID,
		})
	}
	return appendRedlineInstructionsToPrompt(prompt, msg.RedlineInstructions)
}

func workerUsesGPTContextDelivery(provider adapter.ProviderAdapter) bool {
	if provider == nil {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(provider.Name())) {
	case "codex", "openai", "gpt":
		return true
	default:
		return false
	}
}

func renderRetainedRequiredContext(receipt promptlayer.ContextDeliveryResult) string {
	layers := make(map[string]promptlayer.Layer, len(receipt.Layers))
	for _, layer := range receipt.Layers {
		layers[layer.SourceRef] = layer
	}
	var out strings.Builder
	out.WriteString("# Verified Required Context Snapshot\n\n")
	fmt.Fprintf(&out, "schema_version: %s\nsnapshot_hash: %s\nprompt_manifest_hash: %s\n\n", receipt.SchemaVersion, receipt.SnapshotHash, receipt.PromptManifestHash)
	out.WriteString("Read every required document before acting. Emit diagnostic `context_ack` with each source_ref and observed source_hash; supervisor-held references and hashes are the enforcement gate. The task/receipt may not replace these documents.\n")
	for _, document := range receipt.RequiredDocuments {
		layer := layers[document.SourceRef]
		fmt.Fprintf(&out, "\n## Required Document: %s\n\n", document.SourceRef)
		fmt.Fprintf(&out, "[source_hash=%s prompt_hash=%s complete=%t redaction_status=%s]\n\n", document.SourceHash, document.PromptHash, document.Complete, document.RedactionStatus)
		out.WriteString(layer.Content)
		out.WriteByte('\n')
	}
	return out.String()
}

func attachRequiredContext(task adapter.TaskConfig) adapter.TaskConfig {
	if strings.TrimSpace(task.RequiredContext) == "" {
		return task
	}
	task.Prompt = task.RequiredContext + "\n\n# Ephemeral Task Context\n\n" + task.Prompt
	return task
}

func validateRetainedPromptAdmission(prompt string) error {
	tokens := promptlayer.EstimateTokens(prompt)
	if tokens > maxRetainedRequiredContextTokens {
		return fmt.Errorf("verified prompt is %d tokens, above the %d-token safe admission limit; split the task instead of truncating context", tokens, maxRetainedRequiredContextTokens)
	}
	return nil
}
