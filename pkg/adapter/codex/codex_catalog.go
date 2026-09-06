package codex

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/insajin/autopus-adk/pkg/codexruntime"
	"github.com/insajin/autopus-adk/pkg/config"
)

const codexCatalogTimeout = 5 * time.Second

type codexRenderContext struct {
	*config.HarnessConfig
	adapter *Adapter
}

func (a *Adapter) prepareCodexCatalog(ctx context.Context) {
	if a.codexCatalogProbed {
		return
	}
	a.codexCatalogProbed = true
	output, err := codexruntime.ProbeModelCatalog(ctx, cliBinary, codexCatalogTimeout)
	if err != nil {
		a.codexCatalogJSON = nil
		return
	}
	a.codexCatalogJSON = output
}

func (a *Adapter) codexRenderData(cfg *config.HarnessConfig) any {
	if !a.codexCatalogProbed {
		return cfg
	}
	return codexRenderContext{HarnessConfig: cfg, adapter: a}
}

func (c codexRenderContext) CodexSupervisorModel() string {
	if !c.Quality.ManagesSupervisorModel() {
		return ""
	}
	return c.resolve(c.Quality.CodexSupervisorProfile()).Effective.Model
}

func (c codexRenderContext) CodexSupervisorEffort() string {
	if !c.Quality.ManagesSupervisorModel() {
		return ""
	}
	return c.resolve(c.Quality.CodexSupervisorProfile()).Effective.Effort
}

// CodexAgentModel and CodexAgentEffort answer the agent TOML templates. Both
// return an error so a catalog that rejects the native balanced placement stops
// template execution, and with it the whole surface preparation, before any
// mapping reaches the transaction.
func (c codexRenderContext) CodexAgentModel(agentName, fallbackTier, declaredEffort string) (string, error) {
	resolution, err := c.resolveAgent(agentName, fallbackTier, declaredEffort)
	if err != nil {
		return "", err
	}
	return resolution.Effective.Model, nil
}

func (c codexRenderContext) CodexAgentEffort(agentName, fallbackTier, declaredEffort string) (string, error) {
	resolution, err := c.resolveAgent(agentName, fallbackTier, declaredEffort)
	if err != nil {
		return "", err
	}
	return resolution.Effective.Effort, nil
}

// resolveAgent resolves one managed agent profile against the probed catalog.
// The standard native balanced placement is a routing decision rather than a
// preference: answering it with a lower model or effort would hand the agent a
// weaker rung than the policy names, so that path refuses substitution. Every
// other profile — Ultra, a hand-edited tier, a non-canonical agent — keeps the
// ordered availability fallback it has always had.
func (c codexRenderContext) resolveAgent(
	agentName, fallbackTier, declaredEffort string,
) (config.CodexProfileResolution, error) {
	requested := c.Quality.CodexAgentProfile(agentName, fallbackTier, declaredEffort)
	if _, placed := c.Quality.NativeBalancedAgentCandidate(config.QualityProviderCodex, agentName); placed {
		return c.adapter.resolveCodexPlacement(requested)
	}
	return c.resolve(requested), nil
}

func (c codexRenderContext) resolve(requested config.CodexProfile) config.CodexProfileResolution {
	resolution := config.ResolveCodexProfile(requested, c.adapter.codexCatalogJSON)
	c.adapter.reportCodexFallback(resolution)
	return resolution
}

func (a *Adapter) reportCodexFallback(resolution config.CodexProfileResolution) {
	if !resolution.Fallback || a.codexFallbackWriter == nil {
		return
	}
	selected := describeCodexProfile(resolution.Effective)
	key := strings.Join([]string{
		"fallback",
		resolution.Requested.Model,
		resolution.Requested.Effort,
		selected,
		string(resolution.Reason),
	}, "|")
	if !a.claimCodexDiagnostic(key) {
		return
	}
	fmt.Fprintf(a.codexFallbackWriter,
		"Codex model fallback: requested=%s/%s selected=%s reason=%s\n",
		resolution.Requested.Model,
		resolution.Requested.Effort,
		selected,
		resolution.Reason,
	)
}

// claimCodexDiagnostic reports whether this diagnostic is new. Sixteen agent
// templates each ask for a model and an effort, so the same profile is resolved
// many times per run and must still be reported once.
func (a *Adapter) claimCodexDiagnostic(key string) bool {
	if a.codexFallbackSeen == nil {
		a.codexFallbackSeen = make(map[string]struct{})
	}
	if _, exists := a.codexFallbackSeen[key]; exists {
		return false
	}
	a.codexFallbackSeen[key] = struct{}{}
	return true
}
