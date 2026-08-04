package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/insajin/autopus-adk/pkg/promptlayer"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	pipelineOMPCohortCredentialLocator = "AUTOPUS_OMP_CONTEXT_PROVIDER_TOKEN"
	pipelineOMPCohortCredentialSecret  = "cohort-credential-secret"
	pipelineOMPCohortOutputSecret      = "cohort-output-secret"
)

func TestPipelineOMPContextCohort_RequiresExplicitLiveOptInAndExactHardCap(t *testing.T) {
	for _, test := range []struct {
		name, optIn string
		taskCount   int
	}{
		{name: "live opt-in absent", taskCount: 20},
		{name: "cohort exceeds hard cap", optIn: "explicit-live", taskCount: 21},
	} {
		t.Run(test.name, func(t *testing.T) {
			spy := &pipelineOMPCohortProviderSpy{}
			result, err := runPipelineOMPContextCohort(context.Background(), pipelineOMPContextCohortInput{
				LiveOptIn: test.optIn, CredentialLocator: pipelineOMPCohortCredentialLocator,
				Tasks: pipelineOMPCohortTasks(test.taskCount), Run: spy.run,
			})

			require.Error(t, err)
			assertPipelineOMPCohortNonAuthoritative(t, result)
			assert.Zero(t, spy.callCount())
		})
	}
}

func TestPipelineOMPContextCohort_EvaluatesTwentyBalancedPairsAsShadowCandidateWithoutAuthority(t *testing.T) {
	spy := &pipelineOMPCohortProviderSpy{}
	var logs bytes.Buffer
	result, err := runPipelineOMPContextCohort(context.Background(), pipelineOMPContextCohortInput{
		LiveOptIn: "explicit-live", CredentialLocator: pipelineOMPCohortCredentialLocator,
		Tasks: pipelineOMPCohortTasks(20), Run: spy.run, Log: &logs,
	})

	require.NoError(t, err)
	assert.Equal(t, "shadow-candidate", result.Receipt.EvidenceClass)
	assert.Equal(t, "evaluated", result.Receipt.Status)
	assertPipelineOMPCohortNonAuthoritative(t, result)
	assert.Equal(t, 40, spy.callCount())
	assert.Equal(t, 1, spy.maxConcurrent())
	assert.True(t, spy.allLocatorsEqual(pipelineOMPCohortCredentialLocator))
	aggregate, err := promptlayer.ReduceOMPContextCanaryPairsV1(result.Rows)
	require.NoError(t, err)
	assert.Equal(t, 20, aggregate.PairCount)
	assert.Equal(t, 10, aggregate.ABCount)
	assert.Equal(t, 10, aggregate.BACount)
	assert.True(t, aggregate.BalancedOrder)
	assert.Zero(t, aggregate.UnpairedRows)
	assert.Zero(t, aggregate.DuplicateRows)

	receipt, err := json.Marshal(result.Receipt)
	require.NoError(t, err)
	visible := string(receipt) + "\n" + logs.String()
	assert.Contains(t, visible, pipelineOMPCohortCredentialLocator)
	assertPipelineOMPCohortBodyFree(t, visible)
}

func TestPipelineOMPContextCohort_RejectsIncompleteOrFailingCandidateWithoutAuthoritySemantics(t *testing.T) {
	for _, test := range []struct {
		name string
		spy  *pipelineOMPCohortProviderSpy
	}{
		{name: "incomplete provider cohort", spy: &pipelineOMPCohortProviderSpy{failCall: 40}},
		{name: "integrity oracle failed", spy: &pipelineOMPCohortProviderSpy{failIntegrity: true}},
		{name: "quality oracle regressed", spy: &pipelineOMPCohortProviderSpy{failQuality: true}},
	} {
		t.Run(test.name, func(t *testing.T) {
			var logs bytes.Buffer
			result, err := runPipelineOMPContextCohort(context.Background(), pipelineOMPContextCohortInput{
				LiveOptIn: "explicit-live", CredentialLocator: pipelineOMPCohortCredentialLocator,
				Tasks: pipelineOMPCohortTasks(20), Run: test.spy.run, Log: &logs,
			})

			require.Error(t, err)
			assertPipelineOMPCohortNonAuthoritative(t, result)
			receipt, marshalErr := json.Marshal(result.Receipt)
			require.NoError(t, marshalErr)
			visible := err.Error() + "\n" + string(receipt) + "\n" + logs.String()
			assertPipelineOMPCohortBodyFree(t, visible)
			assert.LessOrEqual(t, test.spy.callCount(), 40)
		})
	}
}

type pipelineOMPCohortProviderSpy struct {
	mu            sync.Mutex
	calls         int
	inFlight      int
	maxInFlight   int
	failCall      int
	failIntegrity bool
	failQuality   bool
	locators      []string
}

func (spy *pipelineOMPCohortProviderSpy) run(
	_ context.Context, call pipelineOMPContextCohortCall,
) (pipelineOMPContextCohortObservation, error) {
	spy.mu.Lock()
	spy.calls++
	callNumber := spy.calls
	spy.inFlight++
	if spy.inFlight > spy.maxInFlight {
		spy.maxInFlight = spy.inFlight
	}
	spy.locators = append(spy.locators, call.CredentialLocator)
	spy.mu.Unlock()
	time.Sleep(2 * time.Millisecond)
	defer func() {
		spy.mu.Lock()
		spy.inFlight--
		spy.mu.Unlock()
	}()
	if spy.failCall == callNumber {
		return pipelineOMPContextCohortObservation{}, errors.New(
			"provider failure: " + pipelineOMPCohortCredentialSecret + " " + call.Prompt,
		)
	}
	optimized := call.Variant == promptlayer.OMPContextCanaryVariantOptimizedV1
	observation := pipelineOMPContextCohortObservation{
		Tokens: 10000, IntegrityPassed: true, SecurityPassed: true, QualityScore: 100,
		OutputBody: pipelineOMPCohortOutputSecret,
	}
	if optimized {
		observation.Tokens = 7500
		observation.FallbackVerified, observation.RollbackVerified = true, true
		observation.IntegrityPassed = !spy.failIntegrity
		if spy.failQuality {
			observation.QualityScore = 99
		}
	}
	return observation, nil
}

func pipelineOMPCohortTasks(count int) []pipelineOMPContextCohortTask {
	tasks := make([]pipelineOMPContextCohortTask, 0, count)
	for index := 0; index < count; index++ {
		tasks = append(tasks, pipelineOMPContextCohortTask{
			TaskID: fmt.Sprintf("cohort-%02d", index), Prompt: fmt.Sprintf("prompt-secret-%02d", index),
		})
	}
	return tasks
}

func (spy *pipelineOMPCohortProviderSpy) callCount() int {
	spy.mu.Lock()
	defer spy.mu.Unlock()
	return spy.calls
}

func (spy *pipelineOMPCohortProviderSpy) maxConcurrent() int {
	spy.mu.Lock()
	defer spy.mu.Unlock()
	return spy.maxInFlight
}

func (spy *pipelineOMPCohortProviderSpy) allLocatorsEqual(want string) bool {
	spy.mu.Lock()
	defer spy.mu.Unlock()
	for _, locator := range spy.locators {
		if locator != want {
			return false
		}
	}
	return true
}

func assertPipelineOMPCohortBodyFree(t *testing.T, visible string) {
	t.Helper()
	for _, forbidden := range []string{
		pipelineOMPCohortCredentialSecret, "prompt-secret-", pipelineOMPCohortOutputSecret,
	} {
		assert.NotContains(t, visible, forbidden)
	}
	assert.False(t, strings.Contains(strings.ToLower(visible), "authorization"))
}

func assertPipelineOMPCohortNonAuthoritative(t *testing.T, result pipelineOMPContextCohortResult) {
	t.Helper()
	encoded, err := json.Marshal(result)
	require.NoError(t, err)
	visible := strings.ToLower(string(encoded))
	assert.NotContains(t, visible, "authority")
	assert.NotContains(t, visible, "authorized")
}
