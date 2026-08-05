package cli

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"

	"github.com/insajin/autopus-adk/pkg/config"
)

func readOwnedAutopusConfig(root string) (string, []byte, os.FileMode, error) {
	rootInfo, err := os.Lstat(root)
	if err != nil || !rootInfo.IsDir() || rootInfo.Mode()&os.ModeSymlink != 0 {
		return "", nil, 0, errors.New("workspace_root_unsafe")
	}
	path := filepath.Join(root, "autopus.yaml")
	info, err := os.Lstat(path)
	if os.IsNotExist(err) {
		return "", nil, 0, errors.New("autopus_config_missing")
	}
	if err != nil {
		return "", nil, 0, errors.New("autopus_config_unreadable")
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return "", nil, 0, errors.New("autopus_config_unsafe")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", nil, 0, errors.New("autopus_config_unreadable")
	}
	return path, data, info.Mode().Perm(), nil
}

func verifyAutopusConfigSnapshot(path string, want []byte, mode os.FileMode) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Mode().Perm() != mode {
		return errors.New("autopus_config_changed")
	}
	data, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(data, want) {
		return errors.New("autopus_config_changed")
	}
	return nil
}

func marshalAutopusConfig(original []byte, cfg *config.HarnessConfig) ([]byte, error) {
	if cfg == nil || cfg.Validate() != nil {
		return nil, errors.New("autopus_config_invalid")
	}
	var document yaml.Node
	if yaml.Unmarshal(original, &document) != nil || len(document.Content) != 1 ||
		document.Content[0].Kind != yaml.MappingNode {
		return nil, errors.New("autopus_config_invalid")
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
	seen := make(map[string]struct{}, len(mapping.Content)/2)
	policyIndex := -1
	for index := 0; index < len(mapping.Content); index += 2 {
		key := mapping.Content[index]
		if key.Kind != yaml.ScalarNode || key.Value == "" {
			return nil, errors.New("autopus_config_invalid")
		}
		if _, duplicate := seen[key.Value]; duplicate {
			return nil, errors.New("autopus_config_duplicate_key")
		}
		seen[key.Value] = struct{}{}
		if key.Value == "role_model_policy" {
			policyIndex = index + 1
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
	encoded, err := yaml.Marshal(&document)
	if err != nil {
		return nil, errors.New("autopus_config_marshal_failed")
	}
	return encoded, nil
}

func atomicWriteAutopusConfig(root, path string, data []byte, mode os.FileMode) error {
	temp, err := os.CreateTemp(root, ".autopus.yaml.tmp-")
	if err != nil {
		return err
	}
	tempPath := temp.Name()
	defer func() {
		_ = temp.Close()
		_ = os.Remove(tempPath)
	}()
	if err := temp.Chmod(mode); err != nil {
		return err
	}
	if _, err := temp.Write(data); err != nil {
		return err
	}
	if err := temp.Sync(); err != nil {
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tempPath, path); err != nil {
		return err
	}
	return nil
}
