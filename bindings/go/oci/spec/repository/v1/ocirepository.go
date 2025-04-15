package v1

import (
	"path"

	"ocm.software/open-component-model/bindings/go/runtime"
)

const (
	LegacyRegistryType = "OCIRegistry"
	Type               = "OCIRepository"
)

// OCIRepository is a type that represents an OCI repository.
//
// +k8s:deepcopy-gen:interfaces=ocm.software/open-component-model/bindings/go/runtime.Typed
// +k8s:deepcopy-gen=true
// +ocm:typegen=true
type OCIRepository struct {
	Type    runtime.Type `json:"type"`
	BaseUrl string       `json:"baseUrl"`
	SubPath string       `json:"subPath"`
}

func (a *OCIRepository) String() string {
	return path.Join(a.BaseUrl, a.SubPath)
}
