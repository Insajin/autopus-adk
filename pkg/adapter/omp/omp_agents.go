package omp

import (
	"fmt"
	"path/filepath"

	contentfs "github.com/insajin/autopus-adk/content"
	"github.com/insajin/autopus-adk/pkg/adapter"
	"github.com/insajin/autopus-adk/pkg/config"
	pkgcontent "github.com/insajin/autopus-adk/pkg/content"
)

func (a *Adapter) prepareAgentMappings() ([]adapter.FileMapping, error) {
	sources, err := pkgcontent.LoadAgentSourcesFromFS(contentfs.FS, "agents")
	if err != nil {
		return nil, fmt.Errorf("agent source 로드 실패: %w", err)
	}
	files := make([]adapter.FileMapping, 0, len(sources))
	for _, src := range sources {
		content := pkgcontent.TransformAgentForOMP(src)
		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".omp", "agents", src.Meta.Name+".md"),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        adapter.Checksum(content),
			Content:         []byte(content),
		})
	}
	return files, nil
}

// prepareAgentMappingsWithProjection renders the opt-in role-model projection.
// Keeping this separate preserves the no-profile SPEC-OMP-002 byte surface.
func (a *Adapter) prepareAgentMappingsWithProjection(
	projection OMPModelProjection,
) ([]adapter.FileMapping, error) {
	sources, err := pkgcontent.LoadAgentSourcesFromFS(contentfs.FS, "agents")
	if err != nil {
		return nil, fmt.Errorf("agent source 로드 실패: %w", err)
	}
	roleSelectors, err := indexOMPModelRoleProjection(projection.ModelRoles)
	if err != nil {
		return nil, err
	}
	projected, err := indexOMPAgentProjection(projection.Agents, roleSelectors)
	if err != nil {
		return nil, err
	}
	if len(projected) != len(sources) {
		return nil, fmt.Errorf("agent_role_set_mismatch: projected=%d source=%d", len(projected), len(sources))
	}

	files := make([]adapter.FileMapping, 0, len(sources))
	for _, src := range sources {
		selection, ok := projected[src.Meta.Name]
		if !ok {
			return nil, fmt.Errorf("agent_role_unmapped: %q", src.Meta.Name)
		}
		content, transformErr := pkgcontent.TransformAgentForOMPWithModel(src,
			pkgcontent.OMPAgentModelSelection{Model: selection.Model, Thinking: selection.Thinking})
		if transformErr != nil {
			return nil, fmt.Errorf("agent %s projection invalid: %w", src.Meta.Name, transformErr)
		}
		files = append(files, adapter.FileMapping{
			TargetPath:      filepath.Join(".omp", "agents", src.Meta.Name+".md"),
			OverwritePolicy: adapter.OverwriteAlways,
			Checksum:        adapter.Checksum(content),
			Content:         []byte(content),
		})
	}
	return files, nil
}

func indexOMPAgentProjection(
	agents []OMPAgentModelProjection,
	roleSelectors map[string]string,
) (map[string]OMPAgentModelProjection, error) {
	result := make(map[string]OMPAgentModelProjection, len(agents))
	for _, agent := range agents {
		if _, exists := result[agent.Agent]; exists {
			return nil, fmt.Errorf("agent_duplicate: %q", agent.Agent)
		}
		expectedRole, roleErr := config.OMPAgentRole(agent.Agent)
		if roleErr != nil {
			return nil, fmt.Errorf("agent_role_unmapped: %q", agent.Agent)
		}
		if agent.Role != expectedRole || agent.Model != "@"+expectedRole {
			return nil, fmt.Errorf("role_capability_mismatch: agent=%s role=%s", agent.Agent, agent.Role)
		}
		roleSelector, ok := roleSelectors[agent.Role]
		_, roleThinking, splitErr := splitOMPProjectedSelector(roleSelector)
		if !ok || splitErr != nil || roleSelector != agent.EffectiveSelector || agent.Thinking != roleThinking {
			return nil, fmt.Errorf("agent_projection_mismatch: agent=%s role=%s", agent.Agent, agent.Role)
		}
		result[agent.Agent] = agent
	}
	return result, nil
}

func indexOMPModelRoleProjection(
	roles []OMPModelRoleProjection,
) (map[string]string, error) {
	if len(roles) != len(ompProjectionRoleSpecs) {
		return nil, fmt.Errorf("model_role_set_mismatch: projected=%d expected=%d",
			len(roles), len(ompProjectionRoleSpecs))
	}
	result := make(map[string]string, len(roles))
	for index, role := range roles {
		expected := ompProjectionRoleSpecs[index]
		if role.Role != expected.role || role.Capability != expected.capability {
			return nil, fmt.Errorf("model_role_order_mismatch: index=%d role=%s", index, role.Role)
		}
		selector, thinking, splitErr := splitOMPProjectedSelector(role.Selector)
		if splitErr != nil {
			return nil, splitErr
		}
		if validateErr := validateOMPProjectedSelector(selector, thinking); validateErr != nil {
			return nil, validateErr
		}
		result[role.Role] = role.Selector
	}
	return result, nil
}
