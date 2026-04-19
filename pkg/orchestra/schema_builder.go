package orchestra

import (
	"encoding/json"
	"fmt"
	"os"
	"reflect"
)

// SchemaBuilder generates JSON schemas from output struct definitions.
type SchemaBuilder struct{}

// roleStructs maps role names to their output struct types.
var roleStructs = map[string]reflect.Type{
	"debater_r1": reflect.TypeOf(DebaterR1Output{}),
	"debater_r2": reflect.TypeOf(DebaterR2Output{}),
	"judge":      reflect.TypeOf(JudgeOutput{}),
	"reviewer":   reflect.TypeOf(ReviewerOutput{}),
}

// Generate returns a JSON Schema string for the given role.
func (sb *SchemaBuilder) Generate(role string) (string, error) {
	t, ok := roleStructs[role]
	if !ok {
		return "", fmt.Errorf("unknown role: %q", role)
	}
	schema := buildSchema(t)
	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal schema: %w", err)
	}
	return string(data), nil
}

// WriteToFile writes the schema to a temp file and returns the path.
// The caller must invoke cleanup to remove the file.
func (sb *SchemaBuilder) WriteToFile(role string) (path string, cleanup func(), err error) {
	schema, err := sb.Generate(role)
	if err != nil {
		return "", nil, err
	}
	f, err := os.CreateTemp("", fmt.Sprintf("schema-%s-*.json", role))
	if err != nil {
		return "", nil, fmt.Errorf("create temp file: %w", err)
	}
	fpath := f.Name()
	if _, err := f.WriteString(schema); err != nil {
		_ = f.Close()
		removeIfExists(fpath)
		return "", nil, fmt.Errorf("write schema: %w", err)
	}
	if err := f.Close(); err != nil {
		removeIfExists(fpath)
		return "", nil, fmt.Errorf("close schema file: %w", err)
	}
	return fpath, func() { removeIfExists(fpath) }, nil
}

// EmbedInPrompt returns the schema as a compact JSON string for prompt embedding.
func (sb *SchemaBuilder) EmbedInPrompt(role string) (string, error) {
	t, ok := roleStructs[role]
	if !ok {
		return "", fmt.Errorf("unknown role: %q", role)
	}
	schema := buildSchema(t)
	data, err := json.Marshal(schema)
	if err != nil {
		return "", fmt.Errorf("marshal schema: %w", err)
	}
	return string(data), nil
}

// buildSchema creates a JSON Schema map from a Go struct type.
func buildSchema(t reflect.Type) map[string]any {
	props := map[string]any{}
	required := []string{}
	for i := range t.NumField() {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "" || tag == "-" {
			continue
		}
		name := tag
		required = append(required, name)
		props[name] = fieldSchema(f.Type)
	}
	return map[string]any{
		"$schema":    "http://json-schema.org/draft-07/schema#",
		"type":       "object",
		"properties": props,
		"required":   required,
	}
}

// fieldSchema returns a JSON Schema fragment for a Go type.
func fieldSchema(t reflect.Type) map[string]any {
	switch t.Kind() {
	case reflect.String:
		return map[string]any{"type": "string"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return map[string]any{"type": "integer"}
	case reflect.Float32, reflect.Float64:
		return map[string]any{"type": "number"}
	case reflect.Bool:
		return map[string]any{"type": "boolean"}
	case reflect.Slice:
		return map[string]any{
			"type":  "array",
			"items": fieldSchema(t.Elem()),
		}
	case reflect.Struct:
		props := map[string]any{}
		req := []string{}
		for i := range t.NumField() {
			f := t.Field(i)
			tag := f.Tag.Get("json")
			if tag == "" || tag == "-" {
				continue
			}
			props[tag] = fieldSchema(f.Type)
			req = append(req, tag)
		}
		return map[string]any{
			"type":       "object",
			"properties": props,
			"required":   req,
		}
	default:
		return map[string]any{"type": "string"}
	}
}

func removeIfExists(path string) {
	_ = os.Remove(path)
}
