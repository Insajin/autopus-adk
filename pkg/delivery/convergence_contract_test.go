package delivery

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const convergenceTestDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

func TestConvergencePrepareReturnsOneBoundedPhase(t *testing.T) {
	fixture := newConvergenceGitFixture(t)
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	contract := validConvergenceContract(now)
	data, err := json.Marshal(contract)
	require.NoError(t, err)

	preparation, err := Prepare(data, fixture.worktree, now)
	require.NoError(t, err)
	require.Len(t, preparation.PhaseContracts, 1)
	assert.Equal(t, DeliveryPreparationSchema, preparation.SchemaVersion)
	assert.Equal(t, PhaseImplement, preparation.Phase)
	assert.Equal(t, convergenceTestDigest, preparation.ExecutionContractDigest)
	assert.Regexp(t, digestPattern, preparation.PreparationDigest)
	phase := preparation.PhaseContracts[0]
	assert.Equal(t, contract.LeaseID, phase.LeaseID)
	assert.Contains(t, phase.Prompt, "Execute exactly one Backend-authorized CodeOps phase.")
	assert.NotContains(t, phase.Prompt, "next_phase")
	assert.NotContains(t, phase.Prompt, fixture.worktree)

	fromReader, err := ReadAndPrepare(bytes.NewReader(data), fixture.worktree, now)
	require.NoError(t, err)
	assert.Equal(t, preparation.PreparationDigest, fromReader.PreparationDigest)
}

func TestConvergenceContractParsingRejectsAmbiguousJSON(t *testing.T) {
	validData, err := json.Marshal(validConvergenceContract(time.Now()))
	require.NoError(t, err)
	parsed, err := ParseExecutionContract(validData)
	require.NoError(t, err)
	assert.Equal(t, ExecutionContractV1, parsed.ContractVersion)

	invalid := [][]byte{
		nil,
		[]byte(`{"contract_version":"codeops.execution.v1","contract_version":"other"}`),
		[]byte(`{"nested":{"key":1,"key":2}}`),
		[]byte(`{"items":[{"key":1,"key":2}]}`),
		append(append([]byte{}, validData...), []byte(` {}`)...),
		append(validData[:len(validData)-1], []byte(`,"unknown":true}`)...),
		[]byte(`{"broken":`),
		bytes.Repeat([]byte("x"), maximumExecutionContractBytes+1),
	}
	for index, data := range invalid {
		_, parseErr := ParseExecutionContract(data)
		assert.Error(t, parseErr, "case %d", index)
		assert.Equal(t, ReasonContractInvalid, ConvergenceReasonCode(parseErr))
	}
}

func TestConvergenceContractValidationFailsClosed(t *testing.T) {
	now := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	tests := []struct {
		name   string
		mutate func(*ExecutionContract)
	}{
		{"version", func(c *ExecutionContract) { c.ContractVersion = "codeops.execution.v2" }},
		{"result schema", func(c *ExecutionContract) { c.ExpectedResultSchema = "other" }},
		{"digest", func(c *ExecutionContract) { c.ExecutionContractDigest = "sha256:bad" }},
		{"scope", func(c *ExecutionContract) { c.RepoScopeRef = "../escape" }},
		{"phase", func(c *ExecutionContract) { c.Phase = "invent" }},
		{"attempt", func(c *ExecutionContract) { c.Attempt = 0 }},
		{"request path", func(c *ExecutionContract) { c.RequestID = "/private/request" }},
		{"empty execution", func(c *ExecutionContract) { c.ExecutionID = "" }},
		{"workspace whitespace", func(c *ExecutionContract) { c.WorkspaceID = " ws" }},
		{"runtime control", func(c *ExecutionContract) { c.RuntimeInstanceID = "rt\nunsafe" }},
		{"lease uri", func(c *ExecutionContract) { c.LeaseID = "file://lease" }},
		{"repository path", func(c *ExecutionContract) { c.RepoConnectionID = `C:\\repo` }},
		{"correlation path", func(c *ExecutionContract) { c.CorrelationID = "a/b" }},
		{"objective empty", func(c *ExecutionContract) { c.Objective = "  " }},
		{"objective unix path", func(c *ExecutionContract) { c.Objective = "Edit /private/project/file.go" }},
		{"objective windows path", func(c *ExecutionContract) { c.Objective = `Edit C:\\project\\file.go` }},
		{"objective file uri", func(c *ExecutionContract) { c.Objective = "Edit FILE://private/project" }},
		{"expired", func(c *ExecutionContract) { c.LeaseExpiresAt = now.Format(time.RFC3339Nano) }},
		{"bad expiry", func(c *ExecutionContract) { c.LeaseExpiresAt = "tomorrow" }},
		{"issued after expiry", func(c *ExecutionContract) { c.LeaseIssuedAt = now.Add(3 * time.Hour).Format(time.RFC3339Nano) }},
		{"bad issued", func(c *ExecutionContract) { c.LeaseIssuedAt = "yesterday" }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			contract := validConvergenceContract(now)
			test.mutate(&contract)
			err := ValidateExecutionContract(contract, now)
			require.Error(t, err)
			assert.Equal(t, ReasonContractInvalid, ConvergenceReasonCode(err))
		})
	}
}

func TestConvergenceContractHelperBounds(t *testing.T) {
	now := time.Now().UTC()
	assert.NoError(t, ValidateExecutionContract(validConvergenceContract(now), now))
	invalidData := []byte(`{"contract_version":"invalid"}`)
	_, err := Prepare(invalidData, "", now)
	assert.Error(t, err)
	fixture := newConvergenceGitFixture(t)
	validData, marshalErr := json.Marshal(validConvergenceContract(now))
	require.NoError(t, marshalErr)
	_, err = Prepare(validData, fixture.repository, now)
	assert.Error(t, err)
	assert.True(t, validBoundedOpaque("opaque-id", 16, true))
	assert.False(t, validBoundedOpaque(strings.Repeat("a", 17), 16, true))
	assert.False(t, validBoundedOpaque("has space", 16, true))
	assert.True(t, validObjective("Change src/value.txt"))
	assert.False(t, validObjective("open value=/private/repo"))
	assert.False(t, validObjective("open (C:/private/repo)"))
	assert.False(t, validObjective("bad\x00objective"))
	assert.False(t, validObjective(strings.Repeat("x", 16*1024+1)))
	assert.True(t, containsAbsolutePath("value=/private/repo"))
	assert.True(t, containsAbsolutePath(`C:\\private\\repo`))
	assert.False(t, containsAbsolutePath("src/value.txt"))

	_, err = ReadAndPrepare(errorReader{}, "", now)
	assert.Error(t, err)
	_, err = ReadAndPrepare(bytes.NewReader(nil), "", now)
	assert.Error(t, err)
	_, err = ReadAndPrepare(bytes.NewReader(bytes.Repeat([]byte("x"), maximumExecutionContractBytes+1)), "", now)
	assert.Error(t, err)

	typed := convergenceError(ReasonScopeInvalid)
	assert.Equal(t, "delivery convergence validation failed", typed.Error())
	assert.Equal(t, ReasonScopeInvalid, ConvergenceReasonCode(typed))
	_, err = digestJSON(make(chan int))
	assert.Error(t, err)

	closing := json.NewDecoder(strings.NewReader("]"))
	assert.Error(t, consumeClosingDelimiter(closing, '}'))
	closing = json.NewDecoder(strings.NewReader(""))
	assert.Error(t, consumeClosingDelimiter(closing, '}'))
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("read failed") }

func validConvergenceContract(now time.Time) ExecutionContract {
	return ExecutionContract{
		ContractVersion:         ExecutionContractV1,
		RequestID:               "req-e2e",
		ExecutionID:             "exec-e2e",
		WorkspaceID:             "ws-e2e",
		RuntimeInstanceID:       "rt-e2e",
		RepoConnectionID:        "repo-connection-e2e",
		RepoScopeRef:            "repo-fixture",
		Phase:                   PhaseImplement,
		Attempt:                 1,
		LeaseID:                 "lease-e2e",
		LeaseIssuedAt:           now.Add(-time.Minute).Format(time.RFC3339Nano),
		LeaseExpiresAt:          now.Add(2 * time.Hour).Format(time.RFC3339Nano),
		CorrelationID:           "correlation-e2e",
		Objective:               "Change src/value.txt and return a strict phase result.",
		ExpectedResultSchema:    PhaseResultSchemaV1,
		ExecutionContractDigest: convergenceTestDigest,
	}
}
