package transformations

import (
	"context"
	"fmt"

	invopop "github.com/invopop/jsonschema"
	"ocm.software/open-component-model/bindings/go/cel/jsonschema"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/runtime"
	"ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1/meta"
)

const DownloadComponentTransformationType = "ocm.software.download.component"

// +k8s:deepcopy-gen:interfaces=ocm.software/open-component-model/bindings/go/runtime.Typed
// +k8s:deepcopy-gen=true
// +ocm:typegen=true
type DownloadComponentTransformation struct {
	meta.TransformationMeta `json:",inline"`
	Spec                    DownloadComponentTransformationSpec `json:"spec"`
}

func (t *DownloadComponentTransformation) GetTransformationMeta() *meta.TransformationMeta {
	return &t.TransformationMeta
}

// +k8s:deepcopy-gen=true
type DownloadComponentTransformationSpec struct {
	Repository *runtime.Raw `json:"repository"`
	Component  string       `json:"component"`
	Version    string       `json:"version"`
}

func (*DownloadComponentTransformation) NestedTypedFields() []string {
	return []string{"repository"}
}

func (t *DownloadComponentTransformation) NewDeclType(pm *manager.PluginManager, nestedFieldTypes map[string]runtime.Type) (*jsonschema.DeclType, error) {
	repoFieldType, ok := nestedFieldTypes["repository"]
	if !ok {
		return nil, fmt.Errorf("missing nested field type for spec.repository")
	}

	specSchema, outSchema, err := downloadComponentTransformationJSONSchema(pm, repoFieldType)
	if err != nil {
		return nil, fmt.Errorf("get JSON schema for %s: %w", repoFieldType.String(), err)
	}
	s := &invopop.Schema{
		Type:       "object",
		Properties: invopop.NewProperties(),
		Required:   []string{"spec", "output"},
	}
	s.Properties.Set("spec", specSchema)
	s.Properties.Set("output", outSchema)

	decl := jsonschema.DeclTypeFromInvopop(s)
	decl = decl.MaybeAssignTypeName("__type_" + t.ID)
	return decl, nil
}

func downloadComponentTransformationJSONSchema(
	pluginManager *manager.PluginManager,
	typ runtime.Type,
) (*invopop.Schema, *invopop.Schema, error) {
	// first convert repos
	descriptorSchemas, err := pluginManager.ComponentVersionRepositoryRegistry.GetJSONSchema(context.TODO(), typ)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get JSON schema for repository %s: %w", typ, err)
	}
	reflector := invopop.Reflector{
		DoNotReference: true,
		Anonymous:      true,
		IgnoredTypes:   []any{&runtime.Raw{}},
	}
	transformationSpecJSONSchema := reflector.Reflect(&DownloadComponentTransformationSpec{})
	transformationSpecJSONSchema.Properties.Set("repository", descriptorSchemas.RepositorySchema)
	return transformationSpecJSONSchema, descriptorSchemas.DescriptorSchema, nil
}
