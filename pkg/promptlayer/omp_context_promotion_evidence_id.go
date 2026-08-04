package promptlayer

import "encoding/json"

const ompContextPromotionEvidenceDomainV1 = "autopus.omp-context.promotion-evidence.v1\x00"

type ompContextPromotionEvidenceStatementV1 struct {
	ChallengeDigest string                            `json:"challenge_digest"`
	Producer        OMPContextPromotionProducerV1     `json:"producer"`
	Candidate       OMPContextPromotionCandidateV1    `json:"candidate"`
	Policy          OMPContextPromotionPolicyReportV1 `json:"policy"`
	Runtime         OMPContextPromotionRuntimeV1      `json:"runtime"`
}

func computeOMPContextPromotionEvidenceIDV1(report OMPContextPromotionReportV1) (string, error) {
	statement, err := json.Marshal(ompContextPromotionEvidenceStatementV1{
		ChallengeDigest: report.ChallengeDigest, Producer: report.Producer, Candidate: report.Candidate,
		Policy: report.Policy, Runtime: report.Runtime,
	})
	if err != nil {
		return "", err
	}
	message := append([]byte(ompContextPromotionEvidenceDomainV1), statement...)
	return promotionSHA256(message), nil
}
