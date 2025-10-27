package openapi

import "ocm.software/open-component-model/bindings/go/runtime"

// Person
// +k8s:openapi-gen=true
type Person struct {
	Name string `json:"name"`
	Age  int    `json:"age"`
}

// GenericOrganism
// +k8s:openapi-gen=true
type GenericOrganism struct {
	Name       string       `json:"name"`
	Properties *runtime.Raw `json:"properties"`
}

// Funghus
// +k8s:openapi-gen=true
type Funghus struct {
	Name       string            `json:"name"`
	Properties FunghusProperties `json:"properties"`
}

// FunghusProperties
// +ocm:typegen=true
// +k8s:openapi-gen=true
type FunghusProperties struct {
	Typ      runtime.Type `json:"typ"`
	Size     int          `json:"size"`
	Softness string       `json:"softness"`
}
