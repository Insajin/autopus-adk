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

func (c codexRenderContext) CodexAgentModel(agentName, fallbackTier, declaredEffort string) string {
	requested := c.Quality.CodexAgentProfile(agentName, fallbackTier, declaredEffort)
	return c.resolve(requested).Effective.Model
}

func (c codexRenderContext) CodexAgentEffort(agentName, fallbackTier, declaredEffort string) string {
	requested := c.Quality.CodexAgentProfile(agentName, fallbackTier, declaredEffort)
	return c.resolve(requested).Effective.Effort
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
	selected := "runtime-default"
	if resolution.Effective.Model != "" {
		selected = resolution.Effective.Model
		if resolution.Effective.Effort != "" {
			selected += "/" + resolution.Effective.Effort
		}
	}
	key := strings.Join([]string{
		resolution.Requested.Model,
		resolution.Requested.Effort,
		selected,
		string(resolution.Reason),
	}, "|")
	if a.codexFallbackSeen == nil {
		a.codexFallbackSeen = make(map[string]struct{})
	}
	if _, exists := a.codexFallbackSeen[key]; exists {
		return
	}
	a.codexFallbackSeen[key] = struct{}{}
	fmt.Fprintf(a.codexFallbackWriter,
		"Codex model fallback: requested=%s/%s selected=%s reason=%s\n",
		resolution.Requested.Model,
		resolution.Requested.Effort,
		selected,
		resolution.Reason,
	)
}
