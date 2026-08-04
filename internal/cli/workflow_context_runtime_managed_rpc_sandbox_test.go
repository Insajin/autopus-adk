package cli

import (
	"runtime"
	"testing"
)

func requireDarwinManagedOMPSandboxForTest(t *testing.T) {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("managed OMP RPC network isolation is only implemented on Darwin")
	}
}
