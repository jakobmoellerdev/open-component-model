package runtime

import (
	"errors"
	"fmt"
	"reflect"

	"github.com/invopop/jsonschema"
)

// GenerateJSONSchemaForType takes a Type and uses reflection to generate a JSON JSONSchema representation for it.
// It will also use the correct type representation as we don't marshal the type in object format.
func GenerateJSONSchemaForType(obj Typed) ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("cannot generate JSON schema for nil object")
	}

	switch obj.(type) {
	case *Unstructured, *Raw:
		return nil, fmt.Errorf("unstructured or raw object type is unsupported")
	}

	r := &jsonschema.Reflector{
		Mapper: func(i reflect.Type) *jsonschema.Schema {
			if i == reflect.TypeOf(Type{}) {
				return &jsonschema.Schema{
					Type:    "string",
					Pattern: `^([a-zA-Z0-9][a-zA-Z0-9.]*)(?:/(v[0-9]+(?:alpha[0-9]+|beta[0-9]+)?))?`,
				}
			}
			return nil
		},
	}

	schema, err := r.ReflectFromType(reflect.TypeOf(obj)).MarshalJSON()
	if err != nil {
		return nil, fmt.Errorf("failed to create json schema for object: %w", err)
	}

	return schema, nil
}

func GenerateJSONSchemaWithScheme(scheme *Scheme, obj any) (*jsonschema.Schema, error) {
	reflector := &jsonschema.Reflector{}

	anyTypedProps := jsonschema.NewProperties()
	anyTypedProps.Set("type", &jsonschema.Schema{
		Type:    "string",
		Pattern: `^([a-zA-Z0-9][a-zA-Z0-9.]*)(?:/(v[0-9]+(?:alpha[0-9]+|beta[0-9]+)?))?`,
	})
	anyTypedScheme := &jsonschema.Schema{
		Type:                 "object",
		Required:             []string{"type"},
		Properties:           anyTypedProps,
		AdditionalProperties: jsonschema.TrueSchema,
	}
	var retErr error
	f := func(r reflect.Type) *jsonschema.Schema {
		v := reflect.New(r).Interface()

		if _, ok := v.(*Typed); ok {
			// this can happen if we have a runtime.Typed nil interface pointer
			// (parent object has a field of type runtime.Typed)
			// Then we cannot derive any typing information
			return anyTypedScheme
		}

		val, ok := v.(Typed)
		if !ok {
			return nil
		}
		_, isRaw := val.(*Raw)
		_, isUnstructured := val.(*Unstructured)

		if isRaw || isUnstructured {
			return anyTypedScheme
		}

		typ, err := scheme.TypeForPrototype(val)
		if err != nil {
			errors.Join(retErr, fmt.Errorf("failed to get type for prototype %T: %w", val, err))
			return nil
		}

		prototype, err := scheme.NewObject(typ)
		if err != nil {
			errors.Join(retErr, fmt.Errorf("failed to create new object for type %s: %w", typ, err))
			return nil
		}
		enum, err := getTypeEnum(scheme, prototype)
		if err != nil {
			errors.Join(retErr, fmt.Errorf("failed to get enum for prototype %T: %w", val, err))
			return nil
		}

		typedReflector := &jsonschema.Reflector{
			DoNotReference: true,
			Anonymous:      true,
			Mapper: func(child reflect.Type) *jsonschema.Schema {
				if child == r {
					return nil
				}
				if child == reflect.TypeOf(Type{}) {
					return enum
				}
				parentReflect := reflector.ReflectFromType(child)
				return parentReflect
			},
		}
		schema := typedReflector.Reflect(prototype)
		return schema
	}
	reflector.Mapper = f
	reflector.Anonymous = true
	reflector.DoNotReference = true

	return reflector.ReflectFromType(reflect.TypeOf(obj)), retErr
}

func getTypeEnum(scheme *Scheme, obj Typed) (*jsonschema.Schema, error) {
	typs, ok := scheme.GetTypes()[obj.GetType()]
	if !ok {
		return nil, fmt.Errorf("cannot generate JSON schema for object with unknown type: %s", obj.GetType())
	}
	typEnum := make([]any, 0, len(typs)+1)
	typEnum = append(typEnum, obj.GetType().String())
	for _, typ := range typs {
		typEnum = append(typEnum, typ.String())
	}
	typScheme := &jsonschema.Schema{
		Type: "string",
		Enum: typEnum,
	}
	return typScheme, nil
}
