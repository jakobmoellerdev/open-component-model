package jsonschema

import (
	"reflect"

	"github.com/invopop/jsonschema"
)

type Schema struct {
	JSONSchema *jsonschema.Schema
}

func (s *Schema) Allows() bool {
	if s == nil {
		return false
	}

	v := reflect.ValueOf(s.JSONSchema).Elem() // dereference pointer to Schema
	field := v.FieldByName("boolean")
	if field.IsValid() && !field.IsNil() {
		return field.Elem().Bool() // return the boolean value
	}

	// Not a boolean schema; normal object schema allows fields
	return true
}

func (s *Schema) Type() string {
	return s.JSONSchema.Type
}

func (s *Schema) Format() string {
	return s.JSONSchema.Format
}

func (s *Schema) Pattern() string {
	return s.JSONSchema.Pattern
}

func (s *Schema) Items() *Schema {
	if s.JSONSchema.Items == nil {
		return nil
	}
	return &Schema{JSONSchema: s.JSONSchema.Items}
}

func (s *Schema) Properties() map[string]*Schema {
	if s.JSONSchema.Properties == nil {
		return nil
	}
	res := make(map[string]*Schema, s.JSONSchema.Properties.Len())
	for pair := s.JSONSchema.Properties.Oldest(); pair != nil; pair = pair.Next() {
		if pair.Value != nil {
			res[pair.Key] = &Schema{JSONSchema: pair.Value}
		}
	}
	return res
}

func (s *Schema) AdditionalProperties() *Schema {
	if s.JSONSchema.AdditionalProperties == nil {
		return nil
	}
	if IsBoolSchema(s.JSONSchema.AdditionalProperties, false) {
		return nil
	}
	return &Schema{JSONSchema: s.JSONSchema.AdditionalProperties}
}

func (s *Schema) Default() any {
	return s.JSONSchema.Default
}

func (s *Schema) Minimum() *float64 {
	f, err := s.JSONSchema.Minimum.Float64()
	if err != nil {
		panic(err)
	}
	return &f
}

func (s *Schema) IsExclusiveMinimum() bool {
	if s.JSONSchema.ExclusiveMinimum != "" {
		return true
	}
	return false
}

func (s *Schema) Maximum() *float64 {
	f, err := s.JSONSchema.Maximum.Float64()
	if err != nil {
		panic(err)
	}
	return &f
}

func (s *Schema) IsExclusiveMaximum() bool {
	if s.JSONSchema.ExclusiveMinimum != "" {
		return true
	}
	return false
}

func (s *Schema) MultipleOf() *float64 {
	f, err := s.JSONSchema.MultipleOf.Float64()
	if err != nil {
		panic(err)
	}
	return &f
}

func (s *Schema) UniqueItems() bool {
	return s.JSONSchema.UniqueItems
}

func (s *Schema) MinItems() *uint64 {
	if s.JSONSchema.MinItems == nil {
		return new(uint64)
	}
	return s.JSONSchema.MinItems
}

func (s *Schema) MaxItems() *uint64 {
	if s.JSONSchema.MaxItems == nil {
		return new(uint64)
	}
	return s.JSONSchema.MaxItems
}

func (s *Schema) MinLength() *uint64 {
	if s.JSONSchema.MinLength == nil {
		return new(uint64)
	}
	return s.JSONSchema.MinLength
}

func (s *Schema) MaxLength() *uint64 {
	if s.JSONSchema.MaxLength == nil {
		return new(uint64)
	}
	return s.JSONSchema.MaxLength
}

func (s *Schema) MinProperties() *uint64 {
	if s.JSONSchema.MinProperties == nil {
		return new(uint64)
	}
	return s.JSONSchema.MinProperties
}

func (s *Schema) MaxProperties() *uint64 {
	if s.JSONSchema.MaxProperties == nil {
		return new(uint64)
	}
	return s.JSONSchema.MaxProperties
}

func (s *Schema) Required() []string {
	return s.JSONSchema.Required
}

func (s *Schema) Enum() []any {
	return s.JSONSchema.Enum
}

func (s *Schema) AllOf() []*Schema {
	var res []*Schema
	for _, nestedSchema := range s.JSONSchema.AllOf {
		nestedSchema := *nestedSchema
		res = append(res, &Schema{&nestedSchema})
	}
	return res
}

func (s *Schema) AnyOf() []*Schema {
	var res []*Schema
	for _, nestedSchema := range s.JSONSchema.AnyOf {
		nestedSchema := *nestedSchema
		res = append(res, &Schema{&nestedSchema})
	}
	return res
}

func (s *Schema) OneOf() []*Schema {
	var res []*Schema
	for _, nestedSchema := range s.JSONSchema.OneOf {
		nestedSchema := *nestedSchema
		res = append(res, &Schema{&nestedSchema})
	}
	return res
}

func (s *Schema) Not() *Schema {
	if s.JSONSchema.Not == nil {
		return nil
	}
	return &Schema{s.JSONSchema.Not}
}
