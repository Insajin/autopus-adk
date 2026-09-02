package config

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// removedConfigKeys lists dotted autopus.yaml paths that the schema deliberately
// dropped. These are accepted and ignored so an existing workspace keeps
// loading after a cutover; Save then rewrites the file without them.
//
// This list is the only opt-out from strict decoding, and every entry is named
// explicitly. An unrecognised key is not an opt-out: it is a typo that must be
// reported, because a silently ignored key looks like a working setting while
// having no effect, and the next Save deletes it from disk.
var removedConfigKeys = []string{
	// Removed by the workflow team-mode cutover; see
	// pkg/config/workflow_cutover_test.go for the retained ignore contract.
	"workflow.team_default",
}

// decodeStrict decodes autopus.yaml and rejects any key the schema does not
// declare, reusing the KnownFields(true) precedent from
// internal/cli/workflow_context_runtime_managed_rpc_authority.go. Deliberately
// removed keys are pruned from the document first so only genuinely unknown
// keys fail.
func decodeStrict(data []byte, out any) error {
	var doc yaml.Node
	// Decode into a node rather than the target struct so a multi-document
	// file stays an error exactly as yaml.Unmarshal made it one.
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return err
	}
	if doc.Kind == 0 {
		// An empty file declares no keys at all. Validate reports the missing
		// required fields with a clearer message than a decoder error would.
		return nil
	}
	for _, key := range removedConfigKeys {
		pruneNodeKey(&doc, strings.Split(key, "."))
	}
	pruned, err := yaml.Marshal(&doc)
	if err != nil {
		return err
	}
	dec := yaml.NewDecoder(bytes.NewReader(pruned))
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("%w (unknown keys are rejected: fix the typo or delete the key)", err)
	}
	return nil
}

// pruneNodeKey deletes the mapping entry addressed by path, walking only
// mapping nodes so a scalar or sequence in the middle of the path is a miss
// rather than a panic.
func pruneNodeKey(doc *yaml.Node, path []string) {
	node := doc
	if node.Kind == yaml.DocumentNode {
		if len(node.Content) == 0 {
			return
		}
		node = node.Content[0]
	}
	for depth, segment := range path {
		if node.Kind != yaml.MappingNode {
			return
		}
		child := -1
		for i := 0; i+1 < len(node.Content); i += 2 {
			if node.Content[i].Value == segment {
				child = i
				break
			}
		}
		if child < 0 {
			return
		}
		if depth == len(path)-1 {
			node.Content = append(node.Content[:child], node.Content[child+2:]...)
			return
		}
		node = node.Content[child+1]
	}
}
