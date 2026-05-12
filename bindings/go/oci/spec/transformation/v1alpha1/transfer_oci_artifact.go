package v1alpha1

import (
	v2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
	"ocm.software/open-component-model/bindings/go/runtime"
)

const TransferOCIArtifactType = "TransferOCIArtifact"

// TransferOCIArtifact is a fused transformation that streams an OCI artifact
// directly from a source registry to a target registry without creating an
// intermediate tar file. It replaces the separate GetOCIArtifact + AddOCIArtifact
// pair when both endpoints support streaming.
type TransferOCIArtifact struct {
	Type   runtime.Type               `json:"type"`
	ID     string                     `json:"id"`
	Spec   *TransferOCIArtifactSpec   `json:"spec"`
	Output *TransferOCIArtifactOutput `json:"output,omitempty"`
}

func (t *TransferOCIArtifact) SetType(typ runtime.Type) { t.Type = typ }
func (t *TransferOCIArtifact) GetType() runtime.Type    { return t.Type }
func (t *TransferOCIArtifact) DeepCopyTyped() runtime.Typed {
	if t == nil {
		return nil
	}
	out := *t
	if t.Spec != nil {
		s := *t.Spec
		if t.Spec.Resource != nil {
			r := *t.Spec.Resource
			s.Resource = &r
		}
		if t.Spec.TargetResource != nil {
			r := *t.Spec.TargetResource
			s.TargetResource = &r
		}
		out.Spec = &s
	}
	if t.Output != nil {
		o := *t.Output
		if t.Output.Resource != nil {
			r := *t.Output.Resource
			o.Resource = &r
		}
		out.Output = &o
	}
	return &out
}

// TransferOCIArtifactSpec is the input specification for the
// TransferOCIArtifact transformation.
type TransferOCIArtifactSpec struct {
	// Resource is the source resource descriptor with OCI image access.
	Resource *v2.Resource `json:"resource"`
	// TargetResource is the target resource descriptor with the destination OCI image reference.
	TargetResource *v2.Resource `json:"targetResource"`
}

// TransferOCIArtifactOutput is the output specification for the
// TransferOCIArtifact transformation.
type TransferOCIArtifactOutput struct {
	// Resource is the updated resource descriptor with target access and pinned digest.
	Resource *v2.Resource `json:"resource"`
}
