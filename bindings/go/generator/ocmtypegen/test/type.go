package test

import (
	"github.com/jakobmoellerdev/open-component-model/bindings/go/runtime"
)

// +k8s:deepcopy-gen=true
// +ocm:typegen=true
type SampleType struct {
	Type runtime.Type `json:"type"`
}
