package adapter

// AuditMetadata represents audit event metadata generated from task config.
type AuditMetadata struct {
	ComputerUse bool
}

// buildAuditMetadata maps TaskConfig fields to audit event metadata.
func buildAuditMetadata(task TaskConfig) AuditMetadata {
	return AuditMetadata{
		ComputerUse: task.ComputerUse,
	}
}
