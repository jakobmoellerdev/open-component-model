package jsonschemagen

import (
	"encoding/json"

	"github.com/google/jsonschema-go/jsonschema"
	"ocm.software/open-component-model/bindings/go/runtime"
)

const BuiltinComment = "this core runtime schema was automatically included by the ocm schema generation tool to allow introspection"

func (g *Generator) builtinRuntimeRaw() *jsonschema.Schema {
	var raw jsonschema.Schema
	if err := json.Unmarshal(runtime.Raw{}.JSONSchema(), &raw); err != nil {
		panic(err)
	}
	raw.Comment = BuiltinComment
	return &raw
}

func (g *Generator) builtinRuntimeType() *jsonschema.Schema {
	var raw jsonschema.Schema
	if err := json.Unmarshal(runtime.Type{}.JSONSchema(), &raw); err != nil {
		panic(err)
	}
	raw.Comment = BuiltinComment
	return &raw
}

func (g *Generator) builtinRuntimeTyped() *jsonschema.Schema {
	return &jsonschema.Schema{
		Schema:      JSONSchemaDraft202012URL,
		Comment:     BuiltinComment,
		Title:       "Typed",
		Description: "Typed is used to hold arbitrary typed objects identified by their Type field",
		Ref:         "#/$defs/ocm.software.open-component-model.bindings.go.runtime.Raw",
		Defs: map[string]*jsonschema.Schema{
			"ocm.software.open-component-model.bindings.go.runtime.Raw": g.builtinRuntimeRaw(),
		},
	}
}
