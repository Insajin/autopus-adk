package codex

func codexRequiredContextDeliveryContract() string {
	return `

## Verified Required-Context Delivery

Token efficiency may reduce only optional recall. Required documents are a separate, non-reducible frozen snapshot and are never charged against the 800-2,000 token context-receipt budget.

Before dispatch:

1. Run ` + "`auto workflow context --project-dir <root> --command go --spec-dir <spec-dir> --format json`" + `.
2. Keep every ` + "`required_documents`" + ` entry with its stable source reference, ` + "`source_hash`" + `, ` + "`prompt_hash`" + `, token estimate, and ` + "`complete=true`" + ` state. Add task-specific ` + "`required_references`" + ` with repeated ` + "`--required-document <ref>`" + ` flags.
3. Put the exact required references and hashes in the worker message. ` + "`fork_turns=\"all\"`" + ` preserves conversation context, but it does not replace explicit document delivery.
4. Require the worker to read every reference before action and request ` + "`context_ack`" + ` with the observed hashes as diagnostic evidence. The supervisor-held reference set and hashes remain the integrity gate.
5. Verify the body-free manifest again before compact review selection with ` + "`auto workflow binding ... --context-manifest <manifest> --context-root <root> --context-command go --context-spec-dir <spec-dir> --context-required-document <ref>`" + `. Repeat the flag for the exact task-specific set used during context creation. The expected command, SPEC directory, conditional profiles, and references must match the manifest.

For required documents, never truncate, never summarize, and never drop content. Secret redaction and injection neutralization may transform unsafe spans, but the manifest must record that transformation. A missing, empty, unreadable, stale, incomplete, omitted-reference, replay, or hash mismatch result is ` + "`context_integrity_failed`" + `: retain ` + "`full_ultra`" + ` and block provider dispatch until the documents are restored or the task is split. If the complete snapshot cannot fit the model input safely, split or block the task; do not shrink required context.

Only optional recall from memory, knowledge, or index search may consume the residual receipt budget or be omitted. The receipt augments the frozen documents; it never replaces them. ` + "`compact_ultra`" + ` is eligible only when both the rollout receipt and required-context manifest verify successfully.
`
}
