package adapter

type Metadata struct {
	ID                   string   `json:"id"`
	Surfaces             []string `json:"surfaces"`
	RequiredBinaries     []string `json:"required_binaries"`
	DefaultLanes         []string `json:"default_lanes"`
	ArtifactCapabilities []string `json:"artifact_capabilities"`
	SetupGapReason       string   `json:"setup_gap_reason,omitempty"`
}

func Registry() []Metadata {
	return []Metadata{
		metadata("go-test", []string{"cli"}, []string{"go"}),
		metadata("node-script", []string{"package"}, []string{"node", "npm"}),
		metadata("vitest", []string{"frontend", "package"}, []string{"node", "npm"}),
		metadata("jest", []string{"frontend", "package"}, []string{"node", "npm"}),
		metadata("playwright", []string{"frontend"}, []string{"node", "npm"}),
		metadata("pytest", []string{"cli"}, []string{"pytest"}),
		metadata("cargo-test", []string{"cli"}, []string{"cargo"}),
		metadata("auto-test-run", []string{"multi"}, []string{"auto"}),
		metadata("auto-verify", []string{"frontend"}, []string{"auto"}),
		metadata("canary-template", []string{"multi"}, nil),
		metadata("custom-command", []string{"custom"}, nil),
	}
}

func ByID(id string) (Metadata, bool) {
	for _, item := range Registry() {
		if item.ID == id {
			return item, true
		}
	}
	return Metadata{}, false
}

func metadata(id string, surfaces, binaries []string) Metadata {
	return Metadata{
		ID:                   id,
		Surfaces:             surfaces,
		RequiredBinaries:     binaries,
		DefaultLanes:         []string{"fast"},
		ArtifactCapabilities: []string{"stdout", "stderr"},
	}
}
