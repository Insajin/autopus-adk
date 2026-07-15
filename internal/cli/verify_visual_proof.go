package cli

import "strings"

func applySnapshotProofEvidence(evidence *visualEvidence, proof snapshotComparisonProof, present bool) {
	if !present {
		diagnostic := proof.Diagnostic
		if diagnostic == "" {
			diagnostic = "snapshot comparison proof is missing"
		}
		proof = missingSnapshotProof(verifyProjectSelection{NoFilter: true}, diagnostic)
	}
	if len(proof.RequiredProjects) == 0 {
		proof.RequiredProjects = requiredSnapshotProjects(proof, verifyProjectSelection{NoFilter: true})
	}
	diagnostic := proof.Diagnostic
	if diagnostic == "" {
		diagnostic = assessSnapshotProof(proof, verifyProjectSelection{NoFilter: true})
		proof.Diagnostic = diagnostic
	}
	evidence.RequiredProjects = append([]string(nil), proof.RequiredProjects...)
	evidence.SnapshotProofStatus = snapshotProofOverallStatus(proof)
	evidence.SnapshotProofDiagnostic = diagnostic
	evidence.SnapshotProof = projectDesignSnapshotProof(proof)
}

func snapshotProofOverallStatus(proof snapshotComparisonProof) string {
	projects := make(map[string]snapshotProjectProof, len(proof.Projects))
	for _, project := range proof.Projects {
		projects[project.Name] = project
	}
	status := "enabled"
	if proof.UpdateSnapshots != "none" || len(proof.RequiredProjects) == 0 {
		status = "unproven"
	}
	for _, name := range proof.RequiredProjects {
		project, ok := projects[name]
		if !ok || project.State == "unproven" || strings.TrimSpace(project.State) == "" {
			status = "unproven"
			continue
		}
		if project.State == "disabled" {
			return "disabled"
		}
	}
	return status
}
