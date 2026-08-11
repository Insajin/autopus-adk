package execplane

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

// openAIAuthClaim is the namespaced claim in a Codex id_token that carries the
// subscription grade.
const openAIAuthClaim = "https://api.openai.com/auth"

// Entitlement is the grade a provider account is served under. It is the axis
// the model catalog actually depends on: probing `codex debug models` without
// authentication returns a different slug set than probing with it, while two
// accounts on the same grade returned identical decision fields.
//
// Grade is compared for equality only. It is never mapped to a model list —
// that mapping would be our estimate rather than the provider's answer.
type Entitlement struct {
	Grade  string `json:"grade,omitempty"`
	Source string `json:"source,omitempty"`
}

// Known reports whether a grade was recovered. An unknown grade cannot be
// compared, so it degrades to unverified rather than to a permissive default.
func (e Entitlement) Known() bool { return e.Grade != "" }

// ErrNoEntitlementClaim reports that a credential carried no grade claim.
var ErrNoEntitlementClaim = errors.New("execplane: credential carries no entitlement claim")

// codexAuthFile mirrors the id_token location in a Codex auth.json.
type codexAuthFile struct {
	Tokens struct {
		IDToken string `json:"id_token"`
	} `json:"tokens"`
	IDToken string `json:"id_token"`
}

// ParseCodexEntitlement recovers the subscription grade from a Codex auth.json
// payload. Only the grade leaves this function; the token and every account
// identifier inside it stay in memory.
func ParseCodexEntitlement(authJSON []byte, source string) (Entitlement, error) {
	var file codexAuthFile
	if err := json.Unmarshal(authJSON, &file); err != nil {
		return Entitlement{}, fmt.Errorf("parse codex credential: %w", err)
	}
	token := file.Tokens.IDToken
	if token == "" {
		token = file.IDToken
	}
	if token == "" {
		return Entitlement{}, ErrNoEntitlementClaim
	}
	claims, err := decodeJWTClaims(token)
	if err != nil {
		return Entitlement{}, err
	}
	auth, ok := claims[openAIAuthClaim].(map[string]any)
	if !ok {
		return Entitlement{}, ErrNoEntitlementClaim
	}
	grade, ok := auth["chatgpt_plan_type"].(string)
	if !ok || grade == "" {
		return Entitlement{}, ErrNoEntitlementClaim
	}
	return Entitlement{Grade: grade, Source: source}, nil
}

// claudeCredential mirrors the entitlement fields Claude records for the signed
// in account. The orca-managed copy stores the object at the top level while
// the host CLI nests it under `oauthAccount`; both carry identical field names,
// so one shape covers both sources.
type claudeCredential struct {
	OrganizationType string `json:"organizationType"`
	OAuthAccount     *struct {
		OrganizationType string `json:"organizationType"`
	} `json:"oauthAccount"`
}

// ParseClaudeEntitlement recovers the plan grade from a Claude credential.
//
// The grade is the organization plan, not the rate-limit tier: capacity tiers
// such as 5x versus 20x throttle the same model set, so comparing them would
// force a needless re-probe between accounts that are in fact equivalent.
func ParseClaudeEntitlement(payload []byte, source string) (Entitlement, error) {
	var credential claudeCredential
	if err := json.Unmarshal(payload, &credential); err != nil {
		return Entitlement{}, fmt.Errorf("parse claude credential: %w", err)
	}
	grade := credential.OrganizationType
	if grade == "" && credential.OAuthAccount != nil {
		grade = credential.OAuthAccount.OrganizationType
	}
	if grade == "" {
		return Entitlement{}, ErrNoEntitlementClaim
	}
	return Entitlement{Grade: grade, Source: source}, nil
}

// decodeJWTClaims decodes the payload segment without verifying the signature.
// The token already authenticated the local CLI; this only reads a self-report
// of the grade, and a forged grade would only make the gate re-probe or refuse.
func decodeJWTClaims(token string) (map[string]any, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("execplane: credential token is not a JWT")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode credential claims: %w", err)
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, fmt.Errorf("parse credential claims: %w", err)
	}
	return claims, nil
}

// Verdict is the outcome of comparing the execution and probe entitlements.
type Verdict string

const (
	// VerdictTrusted means the held catalog was probed under the same grade the
	// workload will run under, so it stands as evidence with no re-probe.
	VerdictTrusted Verdict = "trusted"
	// VerdictReprobe means the grades differ, so the held catalog proves nothing
	// about the execution account and must be replaced.
	VerdictReprobe Verdict = "reprobe"
	// VerdictUnverified means the grades could not be compared at all.
	VerdictUnverified Verdict = "unverified"
)

// CompareEntitlement decides whether a catalog probed under probe is evidence
// for a workload running under exec. It returns the verdict and the reason that
// belongs in the receipt; the reason is never empty.
func CompareEntitlement(exec, probe Entitlement) (Verdict, string) {
	switch {
	case !exec.Known() && !probe.Known():
		return VerdictUnverified, "neither execution nor probe entitlement is known"
	case !exec.Known():
		return VerdictUnverified, "execution account entitlement is unknown"
	case !probe.Known():
		return VerdictUnverified, "probe entitlement is unknown"
	case exec.Grade == probe.Grade:
		return VerdictTrusted, "execution and probe accounts share entitlement " + exec.Grade
	default:
		return VerdictReprobe,
			"entitlement differs: execution is " + exec.Grade + ", probe is " + probe.Grade
	}
}
