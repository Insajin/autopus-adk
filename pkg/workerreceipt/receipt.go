package workerreceipt

const (
	SchemaVersion = "autopus.worker_receipt.v1"
	BeginMarker   = "<!-- AUTOPUS_WORKER_RECEIPT_BEGIN -->"
	EndMarker     = "<!-- AUTOPUS_WORKER_RECEIPT_END -->"
)

// Receipt is the exact canonical worker handoff body.
type Receipt struct {
	OwnedPaths       []string `json:"owned_paths" yaml:"owned_paths"`
	ChangedFiles     []string `json:"changed_files" yaml:"changed_files"`
	Verification     []string `json:"verification" yaml:"verification"`
	Blockers         []string `json:"blockers" yaml:"blockers"`
	NextRequiredStep string   `json:"next_required_step" yaml:"next_required_step"`
}

// EvidenceReference is body-free optional evidence metadata.
type EvidenceReference struct {
	Ref  string `json:"ref" yaml:"ref"`
	Hash string `json:"hash" yaml:"hash"`
}

// Envelope versions the canonical body while keeping evidence as a sibling.
type Envelope struct {
	SchemaVersion string              `json:"schema_version" yaml:"schema_version"`
	Receipt       Receipt             `json:"receipt" yaml:"receipt"`
	Evidence      []EvidenceReference `json:"evidence,omitempty" yaml:"evidence,omitempty"`
}
