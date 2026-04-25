package controlplane

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const PolicySigningSecretEnv = "AUTOPUS_A2A_POLICY_SIGNING_SECRET"

// @AX:ANCHOR [AUTO] signed control-plane enforcement gate; keep the env-driven on/off contract stable across worker routing and prompt fallback paths.
// @AX:REASON: Called by loop_task, pipeline execution, and phase parsing to decide when server-signed metadata must override local defaults.
func SignedControlPlaneEnforced() bool {
	return signingSecret() != ""
}

func ValidateSecurityPolicySignature(taskID string, policy any, signature string) error {
	secret := signingSecret()
	if secret == "" {
		return nil
	}
	if strings.TrimSpace(signature) == "" {
		return fmt.Errorf("missing policy signature")
	}
	return VerifySecurityPolicySignature(taskID, policy, signature, secret)
}

func SignSecurityPolicy(taskID string, policy any, secret string) (string, error) {
	payload, err := canonicalSecurityPolicyPayload(taskID, policy)
	if err != nil {
		return "", err
	}
	return signPayload(payload, secret, "policy")
}

func VerifySecurityPolicySignature(taskID string, policy any, signature, secret string) error {
	expected, err := SignSecurityPolicy(taskID, policy, secret)
	if err != nil {
		return err
	}
	if !hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signature))) {
		return fmt.Errorf("policy signature mismatch")
	}
	return nil
}

// @AX:NOTE [AUTO] [downgraded from ANCHOR - fan_in < 3] cached policy signature verification boundary; fail-open or filename changes here would weaken local policy trust.
// @AX:REASON: Called from CLI validation and retained A2A compatibility paths, and depends on the autopus-policy-{task}.json naming contract for sidecar signature lookup.
func VerifyCachedPolicyFile(policyPath string, policy any) error {
	secret := signingSecret()
	if secret == "" {
		return nil
	}
	taskID, err := taskIDFromPolicyPath(policyPath)
	if err != nil {
		return err
	}
	signature, err := ReadPolicySignature(policyPath)
	if err != nil {
		return err
	}
	return VerifySecurityPolicySignature(taskID, policy, signature, secret)
}

func ValidateControlPlaneSignature(taskID, model string, pipelinePhases []string, pipelineInstructions map[string]string, pipelinePromptTemplates map[string]string, iterationBudget any, capabilities []string, signature string) error {
	secret := signingSecret()
	if secret == "" {
		return nil
	}
	if !hasControlPlaneMetadata(model, pipelinePhases, pipelineInstructions, pipelinePromptTemplates, iterationBudget) && len(capabilities) == 0 && strings.TrimSpace(signature) == "" {
		return nil
	}
	if len(capabilities) == 0 {
		return fmt.Errorf("missing control plane capabilities")
	}
	if strings.TrimSpace(signature) == "" {
		return fmt.Errorf("missing control plane signature")
	}
	return VerifyControlPlaneSignature(taskID, model, pipelinePhases, pipelineInstructions, pipelinePromptTemplates, iterationBudget, capabilities, signature, secret)
}

func SignControlPlane(taskID, model string, pipelinePhases []string, pipelineInstructions map[string]string, pipelinePromptTemplates map[string]string, iterationBudget any, capabilities []string, secret string) (string, error) {
	payload, err := canonicalControlPlanePayload(taskID, model, pipelinePhases, pipelineInstructions, pipelinePromptTemplates, iterationBudget, capabilities)
	if err != nil {
		return "", err
	}
	return signPayload(payload, secret, "control plane")
}

func VerifyControlPlaneSignature(taskID, model string, pipelinePhases []string, pipelineInstructions map[string]string, pipelinePromptTemplates map[string]string, iterationBudget any, capabilities []string, signature, secret string) error {
	expected, err := SignControlPlane(taskID, model, pipelinePhases, pipelineInstructions, pipelinePromptTemplates, iterationBudget, capabilities, secret)
	if err != nil {
		return err
	}
	if !hmac.Equal([]byte(expected), []byte(strings.TrimSpace(signature))) {
		return fmt.Errorf("control plane signature mismatch")
	}
	return nil
}

func PolicySignaturePath(policyPath string) string {
	return policyPath + ".sig"
}

func WritePolicySignature(policyPath, signature string) error {
	if strings.TrimSpace(signature) == "" {
		return nil
	}
	path := PolicySignaturePath(policyPath)
	if err := os.WriteFile(path, []byte(signature+"\n"), 0o600); err != nil {
		return fmt.Errorf("write policy signature: %w", err)
	}
	return nil
}

func ReadPolicySignature(policyPath string) (string, error) {
	data, err := os.ReadFile(PolicySignaturePath(policyPath))
	if err != nil {
		return "", fmt.Errorf("read policy signature: %w", err)
	}
	return strings.TrimSpace(string(data)), nil
}

func signingSecret() string {
	return strings.TrimSpace(os.Getenv(PolicySigningSecretEnv))
}

func signPayload(payload []byte, secret, label string) (string, error) {
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write(payload); err != nil {
		return "", fmt.Errorf("sign %s payload: %w", label, err)
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}

func canonicalSecurityPolicyPayload(taskID string, policy any) ([]byte, error) {
	payload := struct {
		TaskID string `json:"task_id"`
		Policy any    `json:"policy"`
	}{
		TaskID: taskID,
		Policy: policy,
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical policy payload: %w", err)
	}
	return data, nil
}

func canonicalControlPlanePayload(taskID, model string, pipelinePhases []string, pipelineInstructions map[string]string, pipelinePromptTemplates map[string]string, iterationBudget any, capabilities []string) ([]byte, error) {
	payload := struct {
		TaskID                  string            `json:"task_id"`
		Model                   string            `json:"model,omitempty"`
		PipelinePhases          []string          `json:"pipeline_phases,omitempty"`
		PipelineInstructions    map[string]string `json:"pipeline_instructions,omitempty"`
		PipelinePromptTemplates map[string]string `json:"pipeline_prompt_templates,omitempty"`
		IterationBudget         any               `json:"iteration_budget,omitempty"`
		Capabilities            []string          `json:"capabilities,omitempty"`
	}{
		TaskID:                  taskID,
		Model:                   strings.TrimSpace(model),
		PipelinePhases:          append([]string(nil), pipelinePhases...),
		PipelineInstructions:    cloneStringMap(pipelineInstructions),
		PipelinePromptTemplates: cloneStringMap(pipelinePromptTemplates),
		IterationBudget:         iterationBudget,
		Capabilities:            append([]string(nil), capabilities...),
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("marshal canonical control plane payload: %w", err)
	}
	return data, nil
}

func hasControlPlaneMetadata(model string, pipelinePhases []string, pipelineInstructions map[string]string, pipelinePromptTemplates map[string]string, iterationBudget any) bool {
	return strings.TrimSpace(model) != "" || len(pipelinePhases) > 0 || len(pipelineInstructions) > 0 || len(pipelinePromptTemplates) > 0 || hasIterationBudget(iterationBudget)
}

func hasIterationBudget(iterationBudget any) bool {
	if iterationBudget == nil {
		return false
	}
	data, err := json.Marshal(iterationBudget)
	if err != nil || string(data) == "null" {
		return false
	}
	var payload struct {
		Limit int `json:"limit"`
	}
	return json.Unmarshal(data, &payload) == nil && payload.Limit > 0
}

func cloneStringMap(values map[string]string) map[string]string {
	if len(values) == 0 {
		return nil
	}
	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return cloned
}

func taskIDFromPolicyPath(policyPath string) (string, error) {
	base := filepath.Base(policyPath)
	const prefix = "autopus-policy-"
	const suffix = ".json"
	if !strings.HasPrefix(base, prefix) || !strings.HasSuffix(base, suffix) {
		return "", fmt.Errorf("unexpected policy filename: %s", base)
	}
	return strings.TrimSuffix(strings.TrimPrefix(base, prefix), suffix), nil
}
