package jsonschema

import (
	"time"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	invopop "github.com/invopop/jsonschema"
)

const MaxRequestSizeBytes = uint64(3 * 1024 * 1024)

func SchemaDeclTypeForInvopop(s *invopop.Schema) *DeclType {
	return SchemaDeclType(&Schema{JSONSchema: s})
}

// SchemaDeclType converts the structural schema to a CEL declaration, or returns nil if the
// structural schema should not be exposed in CEL expressions.
// Set isResourceRoot to true for the root of a custom resource or embedded resource.
//
// Schemas with XPreserveUnknownFields not exposed unless they are objects. Array and "maps" schemas
// are not exposed if their items or additionalProperties schemas are not exposed. Object Properties are not exposed
// if their schema is not exposed.
//
// The CEL declaration for objects with XPreserveUnknownFields does not expose unknown fields.
func SchemaDeclType(s *Schema) *DeclType {
	if s == nil {
		return nil
	}

	typ := s.Type()
	_ = typ // for future use with multiple types

	switch s.Type() {
	case "array":
		if s.Items() != nil {
			itemsType := SchemaDeclType(s.Items())
			if itemsType == nil {
				return nil
			}
			var maxItems uint64
			if s.MaxItems() != nil {
				maxItems = *s.MaxItems()
			} else {
				maxItems = estimateMaxArrayItemsFromMinSize(itemsType.MinSerializedSize)
			}
			return NewListType(itemsType, maxItems)
		}
		return nil
	case "object":
		if s.AdditionalProperties() != nil {
			propsType := SchemaDeclType(s.AdditionalProperties())
			if propsType != nil {
				var maxProperties uint64
				if s.MaxProperties() != nil {
					maxProperties = *s.MaxProperties()
				} else {
					maxProperties = estimateMaxAdditionalPropertiesFromMinSize(propsType.MinSerializedSize)
				}
				return NewMapType(StringType, propsType, maxProperties)
			}
			return nil
		}
		fields := make(map[string]*DeclField, len(s.Properties()))

		required := map[string]bool{}
		if s.Required() != nil {
			for _, f := range s.Required() {
				required[f] = true
			}
		}
		// an object will always be serialized at least as {}, so account for that
		minSerializedSize := uint64(2)
		for name, prop := range s.Properties() {
			var enumValues []interface{}
			if prop.Enum() != nil {
				for _, e := range prop.Enum() {
					enumValues = append(enumValues, e)
				}
			}
			if fieldType := SchemaDeclType(prop); fieldType != nil {
				if propName, ok := Escape(name); ok {
					fields[propName] = NewDeclField(propName, fieldType, required[name], enumValues, prop.Default())
				}
				// the min serialized size for an object is 2 (for {}) plus the min size of all its required
				// properties
				// only include required properties without a default value; default values are filled in
				// server-side
				if required[name] && prop.Default() == nil {
					minSerializedSize += uint64(len(name)) + fieldType.MinSerializedSize + 4
				}
			}
		}
		objType := NewObjectType("object", fields)
		objType.MinSerializedSize = minSerializedSize
		return objType
	case "string":
		switch s.Format() {
		case "byte":
			byteWithMaxLength := NewSimpleTypeWithMinSize("bytes", cel.BytesType, types.Bytes([]byte{}), MinStringSize)
			if s.MaxLength() != nil {
				byteWithMaxLength.MaxElements = *s.MaxLength()
			} else {
				byteWithMaxLength.MaxElements = estimateMaxStringLengthPerRequest(s)
			}
			return byteWithMaxLength
		case "duration":
			durationWithMaxLength := NewSimpleTypeWithMinSize("duration", cel.DurationType, types.Duration{Duration: time.Duration(0)}, uint64(MinDurationSizeJSON))
			durationWithMaxLength.MaxElements = estimateMaxStringLengthPerRequest(s)
			return durationWithMaxLength
		case "date":
			timestampWithMaxLength := NewSimpleTypeWithMinSize("timestamp", cel.TimestampType, types.Timestamp{Time: time.Time{}}, uint64(JSONDateSize))
			timestampWithMaxLength.MaxElements = estimateMaxStringLengthPerRequest(s)
			return timestampWithMaxLength
		case "date-time":
			timestampWithMaxLength := NewSimpleTypeWithMinSize("timestamp", cel.TimestampType, types.Timestamp{Time: time.Time{}}, uint64(MinDatetimeSizeJSON))
			timestampWithMaxLength.MaxElements = estimateMaxStringLengthPerRequest(s)
			return timestampWithMaxLength
		}

		strWithMaxLength := NewSimpleTypeWithMinSize("string", cel.StringType, types.String(""), MinStringSize)
		if s.MaxLength() != nil {
			strWithMaxLength.MaxElements = estimateMaxElementsFromMaxLength(s)
		} else {
			if len(s.Enum()) > 0 {
				strWithMaxLength.MaxElements = estimateMaxStringEnumLength(s)
			} else {
				strWithMaxLength.MaxElements = estimateMaxStringLengthPerRequest(s)
			}
		}
		return strWithMaxLength
	case "boolean":
		return BoolType
	case "number":
		return DoubleType
	case "integer":
		return IntType
	}
	return nil
}

// estimateMaxStringLengthPerRequest estimates the maximum string length (in characters)
// of a string compatible with the format requirements in the provided schema.
// must only be called on schemas of type "string" or x-kubernetes-int-or-string: true
func estimateMaxStringLengthPerRequest(s *Schema) uint64 {
	switch s.Format() {
	case "duration":
		return MaxDurationSizeJSON
	case "date":
		return JSONDateSize
	case "date-time":
		return MaxDatetimeSizeJSON
	default:
		// subtract 2 to account for ""
		return MaxRequestSizeBytes - 2
	}
}

// estimateMaxStringLengthPerRequest estimates the maximum string length (in characters)
// that has a set of enum values.
// The result of the estimation is the length of the longest possible value.
func estimateMaxStringEnumLength(s *Schema) uint64 {
	var maxLength uint64
	for _, v := range s.Enum() {
		if s, ok := v.(string); ok && uint64(len(s)) > maxLength {
			maxLength = uint64(len(s))
		}
	}
	return maxLength
}

// estimateMaxArrayItemsPerRequest estimates the maximum number of array items with
// the provided minimum serialized size that can fit into a single request.
func estimateMaxArrayItemsFromMinSize(minSize uint64) uint64 {
	// subtract 2 to account for [ and ]
	return (MaxRequestSizeBytes - 2) / (minSize + 1)
}

// estimateMaxAdditionalPropertiesPerRequest estimates the maximum number of additional properties
// with the provided minimum serialized size that can fit into a single request.
func estimateMaxAdditionalPropertiesFromMinSize(minSize uint64) uint64 {
	// 2 bytes for key + "" + colon + comma + smallest possible value, realistically the actual keys
	// will all vary in length
	keyValuePairSize := minSize + 6
	// subtract 2 to account for { and }
	return (MaxRequestSizeBytes - 2) / keyValuePairSize
}

// estimateMaxElementsFromMaxLength estimates the maximum number of elements for a string schema
// that is bound with a maxLength constraint.
func estimateMaxElementsFromMaxLength(s *Schema) uint64 {
	// multiply the user-provided max length by 4 in the case of an otherwise-untyped string
	// we do this because the OpenAPIv3 spec indicates that maxLength is specified in runes/code points,
	// but we need to reason about length for things like request size, so we use bytes in this code (and an individual
	// unicode code point can be up to 4 bytes long)
	return *s.MaxLength() * 4
}
