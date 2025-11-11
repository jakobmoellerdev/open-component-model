package v1alpha1

import (
	"context"

	"ocm.software/open-component-model/bindings/go/cel/jsonschema"
	"ocm.software/open-component-model/bindings/go/credentials"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/runtime"
	"ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1/meta"
)

type Transformation interface {
	GetTransformationMeta() *meta.TransformationMeta
	NestedTypedFields() []string
	NewDeclType(pm *manager.PluginManager, nestedTypedFields map[string]runtime.Type) (*jsonschema.DeclType, error)
	FromGeneric(generic *GenericTransformation) error
	Transform(ctx context.Context, pm *manager.PluginManager, credentialProvider credentials.GraphResolver) (map[string]any, error)
}

type Transformer interface {
	Transform(transformation Transformation)
}
