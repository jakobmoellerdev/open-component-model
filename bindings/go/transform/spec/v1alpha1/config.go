package v1alpha1

import (
	"ocm.software/open-component-model/bindings/go/runtime"
)

// +k8s:deepcopy-gen=true
type TransformationGraphDefinition struct {
	Environment     *runtime.Unstructured `json:"environment"`
	Transformations []*runtime.Raw        `json:"transformations"`
}

//type Environment *runtime.Unstructured
