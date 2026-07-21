package orchestra

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"strconv"
	"strings"
)

func PrepareManagedReview(contract ReviewPrepareContract) (ReviewPreparation, error) {
	if validateReviewPrepareContract(contract) != nil {
		return ReviewPreparation{}, ErrReviewPrepareInvalid
	}
	preparation := ReviewPreparation{
		SchemaVersion:     ReviewPreparationSchemaV1,
		RequestID:         contract.RequestID,
		WorkspaceID:       contract.WorkspaceID,
		RepoScopeRef:      contract.RepoScopeRef,
		WorkItemID:        contract.WorkItemID,
		ReviewRunID:       contract.ReviewRunID,
		SnapshotDigest:    contract.SnapshotDigest,
		ContractDigest:    contract.ContractDigest,
		ProviderContracts: make([]ReviewProviderPreparation, 0, len(contract.Providers)),
	}
	for _, provider := range contract.Providers {
		prompt := buildManagedReviewPrompt(contract, provider)
		preparation.ProviderContracts = append(preparation.ProviderContracts, ReviewProviderPreparation{
			AdapterID:      provider.AdapterID,
			Model:          provider.Model,
			Role:           provider.Role,
			Prompt:         prompt,
			PromptDigest:   reviewPromptDigest(prompt),
			ResultSchema:   ReviewProviderResultSchema,
			MaxResultBytes: contract.Bounds.MaxResultBytes,
			MaxFindings:    contract.Bounds.MaxFindings,
		})
	}
	return preparation, nil
}

func ReadAndPrepareManagedReview(reader io.Reader, maxBytes int) (ReviewPreparation, error) {
	limit := maxBytes
	if limit <= 0 || limit > ReviewPrepareMaximumBytes {
		limit = ReviewPrepareMaximumBytes
	}
	payload, err := io.ReadAll(io.LimitReader(reader, int64(limit)+1))
	if err != nil || len(payload) == 0 || len(payload) > limit {
		return ReviewPreparation{}, ErrReviewPrepareInvalid
	}
	contract, err := DecodeReviewPrepareContractStrict(payload, limit)
	if err != nil {
		return ReviewPreparation{}, ErrReviewPrepareInvalid
	}
	return PrepareManagedReview(contract)
}

func buildManagedReviewPrompt(contract ReviewPrepareContract, provider ReviewProviderSpec) string {
	var prompt strings.Builder
	prompt.WriteString("Review exactly one sealed snapshot using typed evidence only.\n")
	prompt.WriteString("Review run: ")
	prompt.WriteString(contract.ReviewRunID)
	prompt.WriteString("\nWork item: ")
	prompt.WriteString(contract.WorkItemID)
	prompt.WriteString("\nSnapshot SHA-256: ")
	prompt.WriteString(contract.SnapshotDigest)
	prompt.WriteString("\nAdapter: ")
	prompt.WriteString(provider.AdapterID)
	prompt.WriteString("\nModel: ")
	prompt.WriteString(provider.Model)
	prompt.WriteString("\nRole: ")
	prompt.WriteString(provider.Role)
	prompt.WriteString("\nReturn exactly one strict ")
	prompt.WriteString(ReviewProviderResultSchema)
	prompt.WriteString(" JSON object. Every finding must include severity, category, scope, typed evidence references, provider provenance, snapshot digest, and result digest. ")
	prompt.WriteString("Set raw_terminal_omitted=true and never include transcript bytes. ")
	prompt.WriteString("Use at most ")
	prompt.WriteString(strconv.Itoa(contract.Bounds.MaxFindings))
	prompt.WriteString(" findings and keep the encoded result at or below ")
	prompt.WriteString(strconv.Itoa(contract.Bounds.MaxResultBytes))
	prompt.WriteString(" bytes.")
	return prompt.String()
}

func reviewPromptDigest(prompt string) string {
	digest := sha256.Sum256([]byte(prompt))
	return hex.EncodeToString(digest[:])
}
