package v1alpha1

import "ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1/meta"

type Transformation interface {
	GetTransformationMeta() meta.TransformationMeta
}

type Transformer interface {
	Transform(transformation Transformation)
}
