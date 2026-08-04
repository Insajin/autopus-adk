package cli

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestPipelineOMPBackend_GenericFactoryRequiresRunScopedAuthority(t *testing.T) {
	backend, err := newPipelineProviderBackend("omp")
	require.Nil(t, backend)
	require.ErrorContains(t, err, "run-scoped OMP authority")
}

func TestPipelineRunHelp_AdvertisesOMPAsAnOwnedPlatform(t *testing.T) {
	cmd := newPipelineRunCmd()
	require.Contains(t, cmd.Flags().Lookup("platform").Usage, "omp")
}
