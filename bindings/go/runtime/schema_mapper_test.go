package runtime

import (
	"testing"

	invopop "github.com/invopop/jsonschema"
	"github.com/stretchr/testify/require"
	orderedmap "github.com/wk8/go-ordered-map/v2"
)

func TestSchemaMapper(t *testing.T) {
	r := require.New(t)

	t.Run("register with typed", func(t *testing.T) {
		scheme := NewScheme()
		r.NoError(scheme.RegisterWithAlias(&testTyped{}, NewUnversionedType("TestType"), NewVersionedType("TestType", "v1")))
		mapper := NewDefaultSchemaMapper(scheme)

		r.NoError(mapper.RegisterSchemaForPrototype(t.Context(), &testTyped{}))
		r.Error(mapper.RegisterSchemaForPrototype(t.Context(), &testTyped{}))

		schema, err := mapper.SchemaForType(t.Context(), NewUnversionedType("TestType"))
		r.NoError(err)
		r.NotNil(schema)

		_, ok := schema.Properties.Get("type")
		r.True(ok)
		_, ok = schema.Properties.Get("value")
		r.True(ok)
	})
	t.Run("register with type", func(t *testing.T) {
		scheme := NewScheme()
		mapper := NewDefaultSchemaMapper(scheme)

		typ := NewUnversionedType("AnotherTestType")
		schema := &invopop.Schema{
			Type: "object",
			Properties: orderedmap.New[string, *invopop.Schema](orderedmap.WithInitialData(
				[]orderedmap.Pair[string, *invopop.Schema]{
					{
						Key:   "type",
						Value: &invopop.Schema{Type: "string"},
					},
					{
						Key:   "value",
						Value: &invopop.Schema{Type: "string"},
					},
				}...)),
		}
		r.NoError(mapper.RegisterSchemaForType(t.Context(), typ, schema))
		r.Error(mapper.RegisterSchemaForType(t.Context(), typ, schema))

		schema, err := mapper.SchemaForType(t.Context(), typ)
		r.NoError(err)
		r.NotNil(schema)

		_, ok := schema.Properties.Get("type")
		r.True(ok)
		_, ok = schema.Properties.Get("value")
		r.True(ok)
	})
	t.Run("fail to register with unknown prototype", func(t *testing.T) {
		scheme := NewScheme()
		mapper := NewDefaultSchemaMapper(scheme)

		r.Error(mapper.RegisterSchemaForPrototype(t.Context(), &testTyped{}))
	})
}

type testTyped struct {
	Typ   Type   `json:"type"`
	Value string `json:"value"`
}

var _ Typed = (*testTyped)(nil)

func (t *testTyped) GetType() Type {
	return t.Typ
}

func (t *testTyped) SetType(typ Type) {
	t.Typ = typ
}

func (t *testTyped) DeepCopyTyped() Typed {
	typed := *t
	return &typed
}
