package delivery

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const maximumExecutionContractBytes = 64 * 1024

var digestPattern = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)

func Prepare(data []byte, workingDirectory string, now time.Time) (Preparation, error) {
	contract, err := ParseExecutionContract(data)
	if err != nil || ValidateExecutionContract(contract, now) != nil {
		return Preparation{}, convergenceError(ReasonContractInvalid)
	}
	receipt, err := Doctor(DoctorOptions{
		WorkingDirectory: workingDirectory,
		RepoScopeRef:     contract.RepoScopeRef,
		Phase:            contract.Phase,
	})
	if err != nil {
		return Preparation{}, err
	}
	phaseContract := buildPhasePreparation(contract, receipt)
	preparationDigest, err := digestJSON(phaseContract)
	if err != nil {
		return Preparation{}, convergenceError(ReasonContractInvalid)
	}
	return Preparation{
		SchemaVersion:           DeliveryPreparationSchema,
		RepoScopeRef:            contract.RepoScopeRef,
		Phase:                   contract.Phase,
		ExecutionContractDigest: contract.ExecutionContractDigest,
		PreparationDigest:       preparationDigest,
		PhaseContracts:          []PhasePreparation{phaseContract},
	}, nil
}

func ReadAndPrepare(reader io.Reader, workingDirectory string, now time.Time) (Preparation, error) {
	data, err := io.ReadAll(io.LimitReader(reader, maximumExecutionContractBytes+1))
	if err != nil || len(data) == 0 || len(data) > maximumExecutionContractBytes {
		return Preparation{}, convergenceError(ReasonContractInvalid)
	}
	return Prepare(data, workingDirectory, now)
}

func ParseExecutionContract(data []byte) (ExecutionContract, error) {
	if len(data) == 0 || len(data) > maximumExecutionContractBytes {
		return ExecutionContract{}, convergenceError(ReasonContractInvalid)
	}
	if rejectDuplicateJSONKeys(data) != nil {
		return ExecutionContract{}, convergenceError(ReasonContractInvalid)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var contract ExecutionContract
	if err := decoder.Decode(&contract); err != nil {
		return ExecutionContract{}, convergenceError(ReasonContractInvalid)
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		return ExecutionContract{}, convergenceError(ReasonContractInvalid)
	}
	return contract, nil
}

func ValidateExecutionContract(contract ExecutionContract, now time.Time) error {
	if contract.ContractVersion != ExecutionContractV1 ||
		contract.ExpectedResultSchema != PhaseResultSchemaV1 ||
		!digestPattern.MatchString(contract.ExecutionContractDigest) ||
		ValidateOpaqueRepoScopeRef(contract.RepoScopeRef) != nil ||
		ValidatePhase(contract.Phase) != nil || contract.Attempt < 1 {
		return convergenceError(ReasonContractInvalid)
	}
	for _, value := range []string{
		contract.RequestID, contract.ExecutionID, contract.WorkspaceID,
		contract.RuntimeInstanceID, contract.LeaseID,
	} {
		if !validBoundedOpaque(value, 256, true) {
			return convergenceError(ReasonContractInvalid)
		}
	}
	if !validBoundedOpaque(contract.RepoConnectionID, 256, true) {
		return convergenceError(ReasonContractInvalid)
	}
	if contract.CorrelationID != "" && !validBoundedOpaque(contract.CorrelationID, 256, true) {
		return convergenceError(ReasonContractInvalid)
	}
	if !validObjective(contract.Objective) {
		return convergenceError(ReasonContractInvalid)
	}
	expiresAt, err := time.Parse(time.RFC3339Nano, contract.LeaseExpiresAt)
	if err != nil || !now.Before(expiresAt) {
		return convergenceError(ReasonContractInvalid)
	}
	if contract.LeaseIssuedAt != "" {
		issuedAt, parseErr := time.Parse(time.RFC3339Nano, contract.LeaseIssuedAt)
		if parseErr != nil || !issuedAt.Before(expiresAt) {
			return convergenceError(ReasonContractInvalid)
		}
	}
	return nil
}

func validBoundedOpaque(value string, maximum int, pathSensitive bool) bool {
	if value == "" || len(value) > maximum || strings.TrimSpace(value) != value {
		return false
	}
	for _, character := range []byte(value) {
		if character < 0x21 || character == 0x7f {
			return false
		}
	}
	if pathSensitive && (strings.ContainsAny(value, `/\\`) || strings.Contains(value, "://")) {
		return false
	}
	return true
}

func validObjective(value string) bool {
	if strings.TrimSpace(value) == "" || len(value) > 16*1024 || strings.ContainsRune(value, '\x00') {
		return false
	}
	if strings.Contains(strings.ToLower(value), "file://") {
		return false
	}
	for _, token := range strings.Fields(value) {
		candidate := strings.Trim(token, `"'()[]{}<>,;:`)
		if filepath.IsAbs(candidate) || containsAbsolutePath(candidate) {
			return false
		}
	}
	return true
}

func containsAbsolutePath(value string) bool {
	if strings.Contains(value, `\\`) {
		return true
	}
	for index := 0; index+2 < len(value); index++ {
		if ((value[index] >= 'A' && value[index] <= 'Z') ||
			(value[index] >= 'a' && value[index] <= 'z')) && value[index+1] == ':' &&
			(value[index+2] == '\\' || value[index+2] == '/') {
			return true
		}
	}
	for index, character := range []byte(value) {
		if character != '/' || index == 0 {
			continue
		}
		if strings.ContainsRune(`=:(['"`, rune(value[index-1])) {
			return true
		}
	}
	return false
}

func buildPhasePreparation(contract ExecutionContract, receipt DoctorReceipt) PhasePreparation {
	return PhasePreparation{
		ContractVersion:         contract.ContractVersion,
		RequestID:               contract.RequestID,
		ExecutionID:             contract.ExecutionID,
		WorkspaceID:             contract.WorkspaceID,
		RuntimeInstanceID:       contract.RuntimeInstanceID,
		RepoConnectionID:        contract.RepoConnectionID,
		RepoScopeRef:            contract.RepoScopeRef,
		Phase:                   contract.Phase,
		Attempt:                 contract.Attempt,
		LeaseID:                 contract.LeaseID,
		LeaseExpiresAt:          contract.LeaseExpiresAt,
		CorrelationID:           contract.CorrelationID,
		LeaseIssuedAt:           contract.LeaseIssuedAt,
		Prompt:                  buildBoundedPhasePrompt(contract),
		ExpectedResultSchema:    contract.ExpectedResultSchema,
		ExecutionContractDigest: contract.ExecutionContractDigest,
		HarnessDigest:           receipt.HarnessDigest,
		ContextDigest:           receipt.ContextDigest,
	}
}

func buildBoundedPhasePrompt(contract ExecutionContract) string {
	var prompt strings.Builder
	prompt.WriteString("Execute exactly one Backend-authorized CodeOps phase.\n")
	prompt.WriteString("Contract: ")
	prompt.WriteString(contract.ContractVersion)
	prompt.WriteString("\nPhase: ")
	prompt.WriteString(string(contract.Phase))
	prompt.WriteString("\nAttempt: ")
	prompt.WriteString(jsonNumber(contract.Attempt))
	prompt.WriteString("\nRequest: ")
	prompt.WriteString(contract.RequestID)
	prompt.WriteString("\nExecution: ")
	prompt.WriteString(contract.ExecutionID)
	prompt.WriteString("\nObjective:\n")
	prompt.WriteString(strings.TrimSpace(contract.Objective))
	prompt.WriteString("\n\nWork only in the current repository-scoped isolated worktree. ")
	prompt.WriteString("Do not choose or start another phase. Return exactly one strict ")
	prompt.WriteString(contract.ExpectedResultSchema)
	prompt.WriteString(" JSON result bound to this request, phase, attempt, and lease. ")
	prompt.WriteString("Do not include raw terminal output or a phase transition decision.")
	return prompt.String()
}

func jsonNumber(value int) string {
	encoded, _ := json.Marshal(value)
	return string(encoded)
}

func digestJSON(value any) (string, error) {
	encoded, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(encoded)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
