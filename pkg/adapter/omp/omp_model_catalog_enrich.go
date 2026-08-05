package omp

type OMPModelCatalogDeclaration struct {
	Selector   string `json:"selector"`
	Family     string `json:"family"`
	Capability string `json:"capability"`
}

// NormalizeOMPAvailableCatalog no longer treats profile declarations as
// runtime evidence. Only a catalog carrying independently observed semantic
// metadata can become ready.
func NormalizeOMPAvailableCatalog(
	data []byte,
	maxOutput int,
	declarations []OMPModelCatalogDeclaration,
) (OMPModelCatalog, string) {
	_ = declarations
	return NormalizeOMPModelCatalog(data, maxOutput)
}
