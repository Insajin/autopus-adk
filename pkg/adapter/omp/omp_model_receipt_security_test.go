package omp

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/insajin/autopus-adk/pkg/config"
)

func TestCanonicalOMPModelResolutionReceipt_OperatorAttestationRejectsIndependentEvidence(t *testing.T) {
	t.Parallel()

	receipt := modelReceiptFixture(time.Date(2026, 8, 30, 1, 2, 3, 0, time.UTC))
	receipt.CatalogTrust = config.RoleModelCatalogTrustOperatorAttested
	for index := range receipt.Roles {
		receipt.Roles[index].EvidenceClass = "operator_attested"
		receipt.Roles[index].EffectiveFamily = receipt.Roles[index].Provider
	}

	canonical, _, err := CanonicalOMPModelResolutionReceipt(receipt)
	require.NoError(t, err)
	for _, role := range canonical.Roles {
		require.False(t, role.FamilyDiversity.IndependentProviderEvidence)
	}

	receipt.Roles[0].FamilyDiversity.IndependentProviderEvidence = true
	_, _, err = CanonicalOMPModelResolutionReceipt(receipt)
	require.ErrorContains(t, err, "independent provider evidence")
}
