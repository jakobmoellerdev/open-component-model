package transformations

import (
	"ocm.software/open-component-model/bindings/go/runtime"
	"ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1/meta"
)

// +k8s:deepcopy-gen=true
type DownloadComponentTransformation struct {
	meta.TransformationMeta `json:",inline"`
	Spec                    DownloadComponentTransformationSpec `json:"spec"`
}

func (t *DownloadComponentTransformation) GetTransformationMeta() meta.TransformationMeta {
	return t.TransformationMeta
}

// +k8s:deepcopy-gen=true
type DownloadComponentTransformationSpec struct {
	Repository *runtime.Raw `json:"repository"`
	Component  string       `json:"component"`
	Version    string       `json:"version"`
}
