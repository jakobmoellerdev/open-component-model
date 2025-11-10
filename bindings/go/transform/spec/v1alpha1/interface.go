package v1alpha1

import (
	"ocm.software/open-component-model/bindings/go/cel/jsonschema"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/runtime"
	"ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1/meta"
)

type Transformation interface {
	GetTransformationMeta() *meta.TransformationMeta
	NestedTypedFields() []string
	NewDeclType(pm *manager.PluginManager, nestedTypedFields map[string]runtime.Type) (*jsonschema.DeclType, error)
}

type Transformer interface {
	Transform(transformation Transformation)
}
