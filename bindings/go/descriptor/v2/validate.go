package v2

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sync"

	_ "embed"

	invopop "github.com/invopop/jsonschema"
	"github.com/santhosh-tekuri/jsonschema/v6"
	"ocm.software/open-component-model/bindings/go/runtime"
	"sigs.k8s.io/yaml"
)

// JSONSchema contains the embedded JSON schema for validating Open Component Model descriptors.
//
//go:embed resources/schema-2020-12.json
var JSONSchema []byte

type Schema struct {
	jsonschema.Schema
	Invopop invopop.Schema
}

// GetJSONSchema is a singleton that compiles the JSON schema once and caches it for reuse.
var GetJSONSchema = sync.OnceValues[*Schema, error](func() (*Schema, error) {
	return compile(JSONSchema)
})

// compile takes raw JSON schema data and compiles it into a jsonschema.JSONSchema object.
// It handles the compilation process including unmarshaling and resource registration.
func compile(data []byte) (*Schema, error) {
	const schemaFile = "resources/schema-2020-12.json"
	c := jsonschema.NewCompiler()
	unmarshaler, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal schema: %w", err)
	}
	var v any
	if err := json.Unmarshal(JSONSchema, &v); err != nil {
		return nil, err
	}
	if err := c.AddResource(schemaFile, unmarshaler); err != nil {
		return nil, fmt.Errorf("failed to add schema: %w", err)
	}
	sch, err := c.Compile(schemaFile)
	if err != nil {
		return nil, fmt.Errorf("failed to compile schema: %w", err)
	}
	invopopSchema, err := runtime.GenerateJSONSchemaWithScheme(runtime.NewScheme(runtime.WithAllowUnknown()), Descriptor{})
	if err != nil {
		return nil, fmt.Errorf("failed to generate invopop schema: %w", err)
	}
	return &Schema{
		Schema:  *sch,
		Invopop: *invopopSchema,
	}, nil
}

// Validate checks if the given descriptor conforms to the JSONSchema.
// It marshals the descriptor to JSON and validates it against the schema.
// Returns an error if validation fails or if there are issues with marshaling.
func Validate(desc *Descriptor) error {
	raw, err := json.Marshal(desc)
	if err != nil {
		return fmt.Errorf("failed to marshal descriptor: %w", err)
	}

	return ValidateRawJSON(raw)
}

// ValidateRawJSON validates raw JSON data against the Open Component Model schema.
// It unmarshals the JSON into a map and validates it against the schema.
// Returns an error if validation fails or if there are issues with unmarshaling.
func ValidateRawJSON(raw []byte) error {
	mm := map[string]any{}
	if err := json.Unmarshal(raw, &mm); err != nil {
		return fmt.Errorf("failed to unmarshal descriptor: %w", err)
	}

	schema, err := GetJSONSchema()
	if err != nil {
		return fmt.Errorf("failed to get schema: %w", err)
	}

	return schema.Validate(mm)
}

// ValidateRawYAML validates raw YAML data against the Open Component Model schema.
// It converts the YAML to JSON and validates it against the schema.
// Returns an error if validation fails or if there are issues with unmarshaling.
func ValidateRawYAML(raw []byte) error {
	mm := map[string]any{}
	if err := yaml.Unmarshal(raw, &mm); err != nil {
		return fmt.Errorf("failed to unmarshal descriptor: %w", err)
	}

	schema, err := GetJSONSchema()
	if err != nil {
		return fmt.Errorf("failed to get schema: %w", err)
	}

	return schema.Validate(mm)
}
