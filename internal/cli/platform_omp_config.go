package cli

import (
	"errors"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/insajin/autopus-adk/pkg/config"
)

var ompProfilePlaceholderPattern = regexp.MustCompile(`\$\{[^}]*\}`)

func validateOMPProfileSource(original []byte) error {
	_, policy, err := parseAutopusConfigDocument(original)
	if err != nil {
		return err
	}
	if policy != nil && yamlNodeContainsOMPProfilePlaceholder(policy, make(map[*yaml.Node]bool)) {
		return errors.New("role_model_policy_placeholder_unsupported")
	}
	return nil
}

func yamlNodeContainsOMPProfilePlaceholder(node *yaml.Node, visited map[*yaml.Node]bool) bool {
	if node == nil || visited[node] {
		return false
	}
	visited[node] = true
	if node.Kind == yaml.ScalarNode && ompProfilePlaceholderPattern.MatchString(node.Value) {
		return true
	}
	if yamlNodeContainsOMPProfilePlaceholder(node.Alias, visited) {
		return true
	}
	for _, child := range node.Content {
		if yamlNodeContainsOMPProfilePlaceholder(child, visited) {
			return true
		}
	}
	return false
}

func parseAutopusConfigDocument(original []byte) (*yaml.Node, *yaml.Node, error) {
	var document yaml.Node
	if yaml.Unmarshal(original, &document) != nil || len(document.Content) != 1 ||
		document.Content[0].Kind != yaml.MappingNode ||
		len(document.Content[0].Content)%2 != 0 {
		return nil, nil, errors.New("autopus_config_invalid")
	}
	mapping := document.Content[0]
	seen := make(map[string]struct{}, len(mapping.Content)/2)
	var policy *yaml.Node
	for index := 0; index < len(mapping.Content); index += 2 {
		key := mapping.Content[index]
		if key.Kind != yaml.ScalarNode || key.Value == "" {
			return nil, nil, errors.New("autopus_config_invalid")
		}
		if _, duplicate := seen[key.Value]; duplicate {
			return nil, nil, errors.New("autopus_config_duplicate_key")
		}
		seen[key.Value] = struct{}{}
		if key.Value == "role_model_policy" {
			policy = mapping.Content[index+1]
		}
	}
	return &document, policy, nil
}

func marshalAutopusConfig(original []byte, cfg *config.HarnessConfig) ([]byte, error) {
	if cfg == nil || cfg.Validate() != nil {
		return nil, errors.New("autopus_config_invalid")
	}
	document, _, err := parseAutopusConfigDocument(original)
	if err != nil {
		return nil, err
	}
	policyData, err := yaml.Marshal(cfg.RoleModelPolicy)
	if err != nil {
		return nil, errors.New("autopus_config_marshal_failed")
	}
	var policyDocument yaml.Node
	if yaml.Unmarshal(policyData, &policyDocument) != nil || len(policyDocument.Content) != 1 {
		return nil, errors.New("autopus_config_marshal_failed")
	}
	mapping := document.Content[0]
	policyIndex := -1
	for index := 0; index < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == "role_model_policy" {
			policyIndex = index + 1
			break
		}
	}
	if policyIndex >= 0 {
		mapping.Content[policyIndex] = policyDocument.Content[0]
	} else {
		mapping.Content = append(mapping.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "role_model_policy"},
			policyDocument.Content[0],
		)
	}
	encoded, err := yaml.Marshal(document)
	if err != nil {
		return nil, errors.New("autopus_config_marshal_failed")
	}
	return encoded, nil
}
