package jsonschema

import (
	"fmt"

	invopop "github.com/invopop/jsonschema"
)

func InferFromGoValue(goRuntimeVal interface{}) (*invopop.Schema, error) {
	switch goRuntimeVal := goRuntimeVal.(type) {
	case bool:
		return &invopop.Schema{
			Type: "boolean",
		}, nil
	case int64:
		return &invopop.Schema{
			Type: "integer",
		}, nil
	case uint64:
		return &invopop.Schema{
			Type: "integer",
		}, nil
	case float64:
		return &invopop.Schema{
			Type: "number",
		}, nil
	case string:
		return &invopop.Schema{
			Type: "string",
		}, nil
	case []interface{}:
		return inferArraySchema(goRuntimeVal)
	case map[string]interface{}:
		return inferObjectSchema(goRuntimeVal)
	default:
		return nil, fmt.Errorf("unsupported type: %T", goRuntimeVal)
	}
}

func inferArraySchema(arr []interface{}) (*invopop.Schema, error) {
	schema := &invopop.Schema{
		Type: "array",
	}

	if len(arr) > 0 {
		itemSchema, err := InferFromGoValue(arr[0])
		if err != nil {
			return nil, fmt.Errorf("failed to infer schema for array item: %w", err)
		}
		schema.Items = itemSchema
	}

	return schema, nil
}

func inferObjectSchema(obj map[string]interface{}) (*invopop.Schema, error) {
	schema := &invopop.Schema{
		Type:       "object",
		Properties: invopop.NewProperties(),
	}

	for key, value := range obj {
		propSchema, err := InferFromGoValue(value)
		if err != nil {
			return nil, fmt.Errorf("failed to infer schema for property %s: %w", key, err)
		}
		schema.Properties.Set(key, propSchema)
	}

	return schema, nil
}
