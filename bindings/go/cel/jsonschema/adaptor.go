package jsonschema

import (
	"math"
	"reflect"

	"github.com/google/cel-go/common/types/ref"
	"github.com/invopop/jsonschema"
	apiservercel "k8s.io/apiserver/pkg/cel"
	"k8s.io/apiserver/pkg/cel/common"
)

var _ common.Schema = (*Schema)(nil)
var _ common.SchemaOrBool = (*Schema)(nil)

type Schema struct {
	JSONSchema *jsonschema.Schema
}

func (s *Schema) Schema() common.Schema {
	return s
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

func (s *Schema) Items() common.Schema {
	if s.JSONSchema.Items == nil {
		return nil
	}
	return &Schema{JSONSchema: s.JSONSchema.Items}
}

func (s *Schema) Properties() map[string]common.Schema {
	if s.JSONSchema.Properties == nil {
		return nil
	}
	res := make(map[string]common.Schema, s.JSONSchema.Properties.Len())
	for pair := s.JSONSchema.Properties.Oldest(); pair != nil; pair = pair.Next() {
		s := *pair.Value
		res[pair.Key] = &Schema{JSONSchema: &s}
	}
	return res
}

func (s *Schema) AdditionalProperties() common.SchemaOrBool {
	if s.JSONSchema.AdditionalProperties == nil {
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

func (s *Schema) MinItems() *int64 {
	if *s.JSONSchema.MinItems > math.MaxInt64 {
		panic("min items cannot be parsed to int64")
	}
	minItems := int64(*s.JSONSchema.MinItems)
	return &minItems
}

func (s *Schema) MaxItems() *int64 {
	if *s.JSONSchema.MaxItems > math.MaxInt64 {
		panic("min items cannot be parsed to int64")
	}
	maxItems := int64(*s.JSONSchema.MaxItems)
	return &maxItems
}

func (s *Schema) MinLength() *int64 {
	if *s.JSONSchema.MinLength > math.MaxInt64 {
		panic("min items cannot be parsed to int64")
	}
	minLength := int64(*s.JSONSchema.MinLength)
	return &minLength
}

func (s *Schema) MaxLength() *int64 {
	if *s.JSONSchema.MaxLength > math.MaxInt64 {
		panic("min items cannot be parsed to int64")
	}
	maxLength := int64(*s.JSONSchema.MaxLength)
	return &maxLength
}

func (s *Schema) MinProperties() *int64 {
	if *s.JSONSchema.MinProperties > math.MaxInt64 {
		panic("min items cannot be parsed to int64")
	}
	minProperties := int64(*s.JSONSchema.MinProperties)
	return &minProperties
}

func (s *Schema) MaxProperties() *int64 {
	if *s.JSONSchema.MaxProperties > math.MaxInt64 {
		panic("min items cannot be parsed to int64")
	}
	minProperties := int64(*s.JSONSchema.MaxProperties)
	return &minProperties
}

func (s *Schema) Required() []string {
	return s.JSONSchema.Required
}

func (s *Schema) Enum() []any {
	return s.JSONSchema.Enum
}

func (s *Schema) Nullable() bool {
	return s.JSONSchema.Nullable
}

func (s *Schema) AllOf() []common.Schema {
	var res []common.Schema
	for _, nestedSchema := range s.JSONSchema.AllOf {
		nestedSchema := *nestedSchema
		res = append(res, &Schema{&nestedSchema})
	}
	return res
}

func (s *Schema) AnyOf() []common.Schema {
	var res []common.Schema
	for _, nestedSchema := range s.JSONSchema.AnyOf {
		nestedSchema := *nestedSchema
		res = append(res, &Schema{&nestedSchema})
	}
	return res
}

func (s *Schema) OneOf() []common.Schema {
	var res []common.Schema
	for _, nestedSchema := range s.JSONSchema.OneOf {
		nestedSchema := *nestedSchema
		res = append(res, &Schema{&nestedSchema})
	}
	return res
}

func (s *Schema) Not() common.Schema {
	if s.JSONSchema.Not == nil {
		return nil
	}
	return &Schema{s.JSONSchema.Not}
}

func (s *Schema) IsXIntOrString() bool {
	return isXIntOrString(s.Schema)
}

func (s *Schema) IsXEmbeddedResource() bool {
	return isXEmbeddedResource(s.Schema)
}

func (s *Schema) IsXPreserveUnknownFields() bool {
	return isXPreserveUnknownFields(s.Schema)
}

func (s *Schema) XListType() string {
	return getXListType(s.Schema)
}

func (s *Schema) XMapType() string {
	return getXMapType(s.Schema)
}

func (s *Schema) XListMapKeys() []string {
	return getXListMapKeys(s.Schema)
}

func (s *Schema) XValidations() []common.ValidationRule {
	return getXValidations(s.Schema)
}

func (s *Schema) WithTypeAndObjectMeta() common.Schema {
	return &Schema{common.WithTypeAndObjectMeta(s.Schema)}
}

func UnstructuredToVal(unstructured any, schema *spec.Schema) ref.Val {
	return common.UnstructuredToVal(unstructured, &Schema{schema})
}

func SchemaDeclType(s *spec.Schema, isResourceRoot bool) *apiservercel.DeclType {
	return common.SchemaDeclType(&Schema{JSONSchema: s}, isResourceRoot)
}

func MakeMapList(sts *spec.Schema, items []interface{}) (rv common.MapList) {
	return common.MakeMapList(&Schema{JSONSchema: sts}, items)
}
