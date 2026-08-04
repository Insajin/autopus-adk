package omp

import (
	"sort"
	"strings"
)

type OMPModelDoctorInput struct {
	Enabled                bool
	WorkspaceRoot          string
	Profile                string
	ConfigSource           string
	ConfiguredSource       string
	ProjectOwnershipDigest string
	Probe                  OMPModelCatalogProbeResult
	Activation             OMPModelActivationEvidence
	Compilation            OMPModelRoutingCompilation
}

type OMPModelDoctorReport struct {
	Enabled       bool                    `json:"enabled"`
	Profile       string                  `json:"profile,omitempty"`
	Status        string                  `json:"status,omitempty"`
	Reason        string                  `json:"reason,omitempty"`
	ReceiptStatus string                  `json:"receipt_status,omitempty"`
	Roles         []OMPModelDoctorRoleRow `json:"roles,omitempty"`
}

type OMPModelDoctorRoleRow struct {
	Agent           string `json:"agent"`
	Role            string `json:"role"`
	Capability      string `json:"capability"`
	Status          string `json:"status"`
	Reason          string `json:"reason"`
	FamilyDiversity string `json:"family_diversity"`
	FamilyReason    string `json:"family_reason"`
	EvidenceClass   string `json:"evidence_class"`
	QuorumEvidence  bool   `json:"quorum_evidence"`
}

// CheckOMPModelRoutingDoctor independently compares the persisted activation
// receipt with a current probe, activation readback, and routing compilation.
func CheckOMPModelRoutingDoctor(input OMPModelDoctorInput) OMPModelDoctorReport {
	if !input.Enabled {
		return OMPModelDoctorReport{}
	}
	report := OMPModelDoctorReport{
		Enabled: true, Profile: safeOMPModelDoctorToken(input.Profile),
		Status: "blocked", ReceiptStatus: "invalid",
		Roles: projectOMPModelDoctorRoles(input.Compilation.Resolutions),
	}
	receipt, receiptReason := readOMPModelDoctorReceipt(input.WorkspaceRoot)
	if receiptReason != "" {
		report.Reason = receiptReason
		if receiptReason == "receipt_missing" {
			report.ReceiptStatus = "missing"
		}
		return report
	}
	report.ReceiptStatus = "valid"

	if input.Probe.Reason != "catalog_ready" || input.Probe.Status != "ready" {
		report.Reason = safeOMPModelDoctorReason(input.Probe.Reason)
		return report
	}
	if receipt.OMPVersion != input.Probe.Version {
		report.Reason = "version_stale"
		return report
	}
	if receipt.CatalogFingerprint != input.Probe.Catalog.Fingerprint {
		report.Reason = "catalog_stale"
		return report
	}
	if !ompModelDoctorProjectionMatches(receipt, input) {
		report.Reason = "projection_mismatch"
		return report
	}

	report.Status, report.Reason = ompModelDoctorRoleSummary(report.Roles)
	return report
}

func projectOMPModelDoctorRoles(resolutions []OMPModelRouteResolution) []OMPModelDoctorRoleRow {
	rows := make([]OMPModelDoctorRoleRow, 0, len(resolutions))
	for _, resolution := range resolutions {
		agent := resolution.Agent
		if agent == "" {
			agent = resolution.RouteID
		}
		status := "blocked"
		switch resolution.Status {
		case "selected":
			status = "supported"
		case "degraded":
			status = "degraded"
		}
		familyStatus := resolution.FamilyDiversity.Status
		if familyStatus == "" {
			familyStatus = "not_applicable"
		}
		familyReason := resolution.FamilyDiversity.Reason
		if familyReason == "" {
			familyReason = "not_applicable"
		}
		rows = append(rows, OMPModelDoctorRoleRow{
			Agent: safeOMPModelDoctorToken(agent), Role: safeOMPModelDoctorToken(resolution.RequestedRole),
			Capability: safeOMPModelDoctorToken(resolution.Capability), Status: status,
			Reason:          safeOMPModelDoctorReason(resolution.Reason),
			FamilyDiversity: safeOMPModelDoctorToken(familyStatus),
			FamilyReason:    safeOMPModelDoctorReason(familyReason),
			EvidenceClass:   "availability", QuorumEvidence: false,
		})
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Agent == rows[j].Agent {
			return rows[i].Role < rows[j].Role
		}
		return rows[i].Agent < rows[j].Agent
	})
	return rows
}

func ompModelDoctorRoleSummary(rows []OMPModelDoctorRoleRow) (string, string) {
	status, reason := "supported", "fresh"
	for _, row := range rows {
		if row.Status == "blocked" {
			return "blocked", "role_blocked"
		}
		if row.Status == "degraded" {
			status, reason = "degraded", "role_degraded"
		}
	}
	return status, reason
}

func safeOMPModelDoctorToken(value string) string {
	if value == "" || len(value) > 128 {
		return "redacted"
	}
	for _, char := range value {
		if char >= 'a' && char <= 'z' || char >= 'A' && char <= 'Z' ||
			char >= '0' && char <= '9' || strings.ContainsRune("._-", char) {
			continue
		}
		return "redacted"
	}
	lower := strings.ToLower(value)
	for _, marker := range []string{"secret", "token", "password", "bearer", "api_key", "api-key", "provider"} {
		if strings.Contains(lower, marker) {
			return "redacted"
		}
	}
	return value
}

func safeOMPModelDoctorReason(value string) string {
	allowed := map[string]bool{
		"selected": true, "catalog_ready": true, "catalog_empty": true,
		"catalog_invalid": true, "catalog_oversized": true, "catalog_timeout": true,
		"catalog_metadata_insufficient": true, "identity_unverified": true,
		"no_compatible_candidate": true, "explicit_degraded": true,
		"role_unknown": true, "capability_mismatch": true,
		"same_family_only": true, "family_unknown": true,
		"not_applicable": true,
	}
	if allowed[value] {
		return value
	}
	return "redacted"
}
