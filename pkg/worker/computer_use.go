package worker

// ComputerUseSupported returns whether the named provider supports computer use.
func ComputerUseSupported(providerName string) bool {
	switch providerName {
	case "claude":
		return true
	default:
		return false
	}
}
