package desktopobserve

func cloneProjection(source SemanticProjection) SemanticProjection {
	clone := source
	clone.Root = cloneSemanticNode(source.Root)
	clone.CanonicalJSON = append([]byte(nil), source.CanonicalJSON...)
	return clone
}

func cloneSemanticNode(source SemanticNode) SemanticNode {
	clone := source
	clone.SemanticState = SemanticState{
		Enabled:  copyPointer(source.SemanticState.Enabled),
		Expanded: copyPointer(source.SemanticState.Expanded),
		Focused:  copyPointer(source.SemanticState.Focused),
		Selected: copyPointer(source.SemanticState.Selected),
	}
	clone.Frame = copyFrame(source.Frame)
	clone.AdvertisedActions = append([]Action(nil), source.AdvertisedActions...)
	clone.Children = make([]SemanticNode, len(source.Children))
	for index := range source.Children {
		clone.Children[index] = cloneSemanticNode(source.Children[index])
	}
	return clone
}

func cloneRuntimeReceipt(source RuntimeReceipt) RuntimeReceipt {
	clone := source
	clone.CapabilitySummary = cloneCapabilities(source.CapabilitySummary)
	clone.ReasonCode = copyPointer(source.ReasonCode)
	clone.NextStep = copyPointer(source.NextStep)
	return clone
}

func cloneCapabilities(source []CapabilityStatus) []CapabilityStatus {
	return append([]CapabilityStatus(nil), source...)
}
