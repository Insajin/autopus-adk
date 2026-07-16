package companionmanifest

import "testing"

func TestBuildProvenance_CrossLanguageAllowlist(t *testing.T) {
	accepted := []string{
		"github-actions:release-123@abcdef",
		"release/0.50.69+darwin_arm64",
	}
	for _, value := range accepted {
		manifest := testManifest()
		manifest.BuildProvenance = value
		if _, err := CanonicalBytes(manifest); err != nil {
			t.Fatalf("accepted vector %q: %v", value, err)
		}
	}

	rejected := []string{
		"release&job",
		"release<job",
		"release>job",
		"release\u0085job",
	}
	for _, value := range rejected {
		manifest := testManifest()
		manifest.BuildProvenance = value
		if _, err := CanonicalBytes(manifest); err == nil {
			t.Fatalf("rejected vector %q was accepted", value)
		}
	}
}
