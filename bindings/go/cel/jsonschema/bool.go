package jsonschema

import (
	"reflect"

	invopop "github.com/invopop/jsonschema"
)

func IsBoolSchema(schema *invopop.Schema, expected bool) bool {
	if schema == nil {
		return false
	}
	val := reflect.ValueOf(schema).Elem()
	boolField := val.FieldByName("boolean")
	if !boolField.IsValid() {
		return false
	}
	if boolField.IsNil() || !boolField.IsValid() {
		return false
	}
	boolDeref := boolField.Elem()
	actual := boolDeref.Bool()
	return actual == expected
}
