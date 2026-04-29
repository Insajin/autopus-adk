package a2a

import (
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
)

// providerSkills maps known provider names to their skill sets.
var providerSkills = map[string][]string{
	"claude":   {"coding", "analysis", "review"},
	"codex":    {"coding", "generation"},
	"gemini":   {"coding", "analysis", "search"},
	"opencode": {"coding"},
}

// defaultProviderSkills is used for unknown providers.
var defaultProviderSkills = []string{"coding"}

const executionLaneBuildChange = "build_change"

var defaultExecutionLanes = []string{executionLaneBuildChange}

var codexUnsupportedModelOverrides = []string{
	"openai/*codex*",
	"*codex*",
}

// CardBuilder constructs an AgentCard from worker configuration.
type CardBuilder struct {
	workerName     string
	backendURL     string
	providers      []string
	version        string
	executionLanes []string
}

// NewCardBuilder creates a CardBuilder with the given worker name and backend URL.
func NewCardBuilder(name, backendURL string) *CardBuilder {
	return &CardBuilder{
		workerName: name,
		backendURL: backendURL,
	}
}

// WithProviders sets the provider list for skill resolution.
func (b *CardBuilder) WithProviders(providers []string) *CardBuilder {
	b.providers = normalizeProviderList(providers)
	return b
}

// WithVersion sets the worker version string.
func (b *CardBuilder) WithVersion(version string) *CardBuilder {
	b.version = version
	return b
}

// WithExecutionLanes sets the explicitly advertised execution lanes.
func (b *CardBuilder) WithExecutionLanes(lanes []string) *CardBuilder {
	b.executionLanes = append([]string(nil), lanes...)
	return b
}

// Build assembles an AgentCard with deduplicated skills from all providers.
func (b *CardBuilder) Build() AgentCard {
	skills := b.resolveSkills()

	description := "Autopus ADK Worker"
	if b.version != "" {
		description = fmt.Sprintf("Autopus ADK Worker v%s", b.version)
	}

	unsupportedModelOverrides := b.resolveUnsupportedModelOverrides()
	log.Printf("[a2a] built agent card: name=%s providers=%v skills=%v unsupported_model_overrides=%v",
		b.workerName, b.providers, skills, unsupportedModelOverrides)

	return AgentCard{
		Name:                      b.workerName,
		Description:               description,
		URL:                       b.backendURL,
		Providers:                 append([]string(nil), b.providers...),
		Skills:                    skills,
		ExecutionLanes:            b.resolveExecutionLanes(),
		Capabilities:              DefaultCapabilities(),
		UnsupportedModelOverrides: unsupportedModelOverrides,
		SupportedInputModes:       []string{"text"},
	}
}

func normalizeProviderName(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	name = strings.ReplaceAll(name, "_", "-")
	switch name {
	case "anthropic", "claude", "claude-code":
		return "claude"
	case "openai", "codex", "openai-codex":
		return "codex"
	case "google", "gemini", "gemini-cli":
		return "gemini"
	case "opencode", "open-code":
		return "opencode"
	default:
		return name
	}
}

func normalizeProviderList(providers []string) []string {
	seen := make(map[string]struct{}, len(providers))
	result := make([]string, 0, len(providers))
	for _, provider := range providers {
		normalized := normalizeProviderName(provider)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		result = append(result, normalized)
	}
	sort.Strings(result)
	return result
}

func mergeSkills(left, right []string) []string {
	seen := make(map[string]struct{}, len(left)+len(right))
	result := make([]string, 0, len(left)+len(right))
	for _, skills := range [][]string{left, right} {
		for _, skill := range skills {
			trimmed := strings.TrimSpace(skill)
			if trimmed == "" {
				continue
			}
			if _, ok := seen[trimmed]; ok {
				continue
			}
			seen[trimmed] = struct{}{}
			result = append(result, trimmed)
		}
	}
	sort.Strings(result)
	return result
}

// resolveSkills collects and deduplicates skills from all providers.
func (b *CardBuilder) resolveSkills() []string {
	seen := make(map[string]struct{})
	var skills []string

	for _, provider := range b.providers {
		provSkills, ok := providerSkills[provider]
		if !ok {
			log.Printf("[a2a] unknown provider %q, using default skills", provider)
			provSkills = defaultProviderSkills
		}
		for _, s := range provSkills {
			if _, exists := seen[s]; !exists {
				seen[s] = struct{}{}
				skills = append(skills, s)
			}
		}
	}

	// Stable output for deterministic cards.
	sort.Strings(skills)
	return skills
}

// resolveExecutionLanes returns a stable explicit lane list for registration.
func (b *CardBuilder) resolveExecutionLanes() []string {
	lanes := b.executionLanes
	if len(lanes) == 0 {
		lanes = defaultExecutionLanes
	}

	seen := make(map[string]struct{}, len(lanes))
	result := make([]string, 0, len(lanes))
	for _, lane := range lanes {
		if lane == "" {
			continue
		}
		if _, ok := seen[lane]; ok {
			continue
		}
		seen[lane] = struct{}{}
		result = append(result, lane)
	}
	if len(result) == 0 {
		return append([]string(nil), defaultExecutionLanes...)
	}
	sort.Strings(result)
	return result
}

func (b *CardBuilder) resolveUnsupportedModelOverrides() []string {
	for _, provider := range b.providers {
		if provider == "codex" {
			return append([]string(nil), codexUnsupportedModelOverrides...)
		}
	}
	return nil
}

// RegistrationResult holds the parsed response from agent/register.
type RegistrationResult struct {
	Success  bool   `json:"success"`
	WorkerID string `json:"worker_id,omitempty"`
	Error    string `json:"error,omitempty"`
}

// ParseRegistrationResponse unmarshals a registration response payload.
func ParseRegistrationResponse(data []byte) (*RegistrationResult, error) {
	var result RegistrationResult
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse registration response: %w", err)
	}
	return &result, nil
}
