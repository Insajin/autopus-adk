package companionmanifest

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDarwinReleaseProducer_ExecutionSmokeFailureFailsClosed(t *testing.T) {
	dir, artifact, output, err := runDarwinReleaseProducer(t, "execution_smoke_failure", false)
	if err == nil {
		t.Fatalf("producer accepted failed execution smoke\n%s", output)
	}
	if !strings.Contains(string(output), "final signed companion execution smoke failed") {
		t.Fatalf("producer did not report execution smoke failure: %s", output)
	}
	events, readErr := os.ReadFile(filepath.Join(dir, "events"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	wantEvents := []string{
		"developer_id_sign", "notary_container", "accepted_notarization",
		"identity_verification",
	}
	if got := strings.Fields(string(events)); !reflect.DeepEqual(got, wantEvents) {
		t.Fatalf("release events = %v, want %v", got, wantEvents)
	}
	assertNoDarwinReleaseMetadata(t, filepath.Dir(artifact))
}

func TestDarwinReleaseProducer_ExecutionSmokeMutationFailsClosed(t *testing.T) {
	dir, artifact, output, err := runDarwinReleaseProducer(t, "execution_smoke_mutation", false)
	if err == nil {
		t.Fatalf("producer accepted artifact mutation during execution smoke\n%s", output)
	}
	if !strings.Contains(string(output), "changed during execution smoke") {
		t.Fatalf("producer did not report execution-smoke mutation: %s", output)
	}
	events, readErr := os.ReadFile(filepath.Join(dir, "events"))
	if readErr != nil {
		t.Fatal(readErr)
	}
	wantEvents := []string{
		"developer_id_sign", "notary_container", "accepted_notarization",
		"identity_verification", "execution_smoke",
	}
	if got := strings.Fields(string(events)); !reflect.DeepEqual(got, wantEvents) {
		t.Fatalf("release events = %v, want %v", got, wantEvents)
	}
	assertNoDarwinReleaseMetadata(t, filepath.Dir(artifact))
}
