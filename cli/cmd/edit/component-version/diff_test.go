package componentversion

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"ocm.software/open-component-model/bindings/go/descriptor/normalisation/json/v4alpha1"
	v2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
	"ocm.software/open-component-model/bindings/go/runtime"
)

// defaultAlgo is the normalisation algorithm used throughout the tests.
var defaultAlgo = v4alpha1.Algorithm

func baseDescriptor() *v2.Descriptor {
	return &v2.Descriptor{
		Meta: v2.Meta{Version: "v2"},
		Component: v2.Component{
			ComponentMeta: v2.ComponentMeta{
				ObjectMeta: v2.ObjectMeta{
					Name:    "ocm.software/test",
					Version: "1.0.0",
				},
				CreationTime: "2025-01-01T00:00:00Z",
			},
			Provider: "test-provider",
			Resources: []v2.Resource{
				{
					ElementMeta: v2.ElementMeta{
						ObjectMeta: v2.ObjectMeta{
							Name:    "my-image",
							Version: "1.0.0",
						},
					},
					Type:     "ociImage",
					Relation: v2.LocalRelation,
					Access:   &runtime.Raw{Data: []byte(`{"type":"localBlob"}`)},
				},
			},
			Sources: []v2.Source{
				{
					ElementMeta: v2.ElementMeta{
						ObjectMeta: v2.ObjectMeta{
							Name:    "my-source",
							Version: "1.0.0",
						},
					},
					Type:   "git",
					Access: &runtime.Raw{Data: []byte(`{"type":"gitHub"}`)},
				},
			},
			References: []v2.Reference{
				{
					ElementMeta: v2.ElementMeta{
						ObjectMeta: v2.ObjectMeta{
							Name:    "my-ref",
							Version: "2.0.0",
						},
					},
					Component: "ocm.software/dependency",
				},
			},
		},
		Signatures: []v2.Signature{
			{
				Name: "test-sig",
				Digest: v2.Digest{
					HashAlgorithm:          "SHA-256",
					NormalisationAlgorithm: "jsonNormalisation/v4alpha1",
					Value:                  "abc123",
				},
				Signature: v2.SignatureInfo{
					Algorithm: "RSASSA-PSS",
					Value:     "sig-value",
					MediaType: "application/octet-stream",
				},
			},
		},
	}
}

// --- isSignatureRelevant tests ---

func TestSignatureRelevant_NoChanges(t *testing.T) {
	desc := baseDescriptor()
	relevant, err := isSignatureRelevant(desc, desc, defaultAlgo)
	require.NoError(t, err)
	assert.False(t, relevant)
}

func TestSignatureRelevant_ProviderChange(t *testing.T) {
	original := baseDescriptor()
	edited := baseDescriptor()
	edited.Component.Provider = "new-provider"

	relevant, err := isSignatureRelevant(original, edited, defaultAlgo)
	require.NoError(t, err)
	assert.True(t, relevant)
}

func TestSignatureRelevant_ComponentNameChange(t *testing.T) {
	original := baseDescriptor()
	edited := baseDescriptor()
	edited.Component.Name = "ocm.software/other"

	relevant, err := isSignatureRelevant(original, edited, defaultAlgo)
	require.NoError(t, err)
	assert.True(t, relevant)
}

func TestSignatureRelevant_VersionChange(t *testing.T) {
	original := baseDescriptor()
	edited := baseDescriptor()
	edited.Component.Version = "2.0.0"

	relevant, err := isSignatureRelevant(original, edited, defaultAlgo)
	require.NoError(t, err)
	assert.True(t, relevant)
}

func TestSignatureRelevant_ResourceTypeChange(t *testing.T) {
	original := baseDescriptor()
	edited := baseDescriptor()
	edited.Component.Resources[0].Type = "helmChart"

	relevant, err := isSignatureRelevant(original, edited, defaultAlgo)
	require.NoError(t, err)
	assert.True(t, relevant)
}

func TestSignatureRelevant_ResourceAdded(t *testing.T) {
	original := baseDescriptor()
	edited := baseDescriptor()
	edited.Component.Resources = append(edited.Component.Resources, v2.Resource{
		ElementMeta: v2.ElementMeta{
			ObjectMeta: v2.ObjectMeta{Name: "new-resource", Version: "1.0.0"},
		},
		Type:     "blob",
		Relation: v2.LocalRelation,
		Access:   &runtime.Raw{Data: []byte(`{"type":"localBlob"}`)},
	})

	relevant, err := isSignatureRelevant(original, edited, defaultAlgo)
	require.NoError(t, err)
	assert.True(t, relevant)
}

func TestSignatureRelevant_ResourceRemoved(t *testing.T) {
	original := baseDescriptor()
	edited := baseDescriptor()
	edited.Component.Resources = nil

	relevant, err := isSignatureRelevant(original, edited, defaultAlgo)
	require.NoError(t, err)
	assert.True(t, relevant)
}

func TestSignatureRelevant_ReferenceComponentNameChange(t *testing.T) {
	original := baseDescriptor()
	edited := baseDescriptor()
	edited.Component.References[0].Component = "ocm.software/other-dep"

	relevant, err := isSignatureRelevant(original, edited, defaultAlgo)
	require.NoError(t, err)
	assert.True(t, relevant)
}

func TestSignatureRelevant_SigningLabelChange(t *testing.T) {
	original := baseDescriptor()
	original.Component.Labels = []v2.Label{
		{Name: "signing-label", Value: json.RawMessage(`"old"`), Signing: true},
	}
	edited := baseDescriptor()
	edited.Component.Labels = []v2.Label{
		{Name: "signing-label", Value: json.RawMessage(`"new"`), Signing: true},
	}

	relevant, err := isSignatureRelevant(original, edited, defaultAlgo)
	require.NoError(t, err)
	assert.True(t, relevant)
}

func TestSignatureRelevant_ResourceRelationChange(t *testing.T) {
	original := baseDescriptor()
	edited := baseDescriptor()
	edited.Component.Resources[0].Relation = v2.ExternalRelation

	relevant, err := isSignatureRelevant(original, edited, defaultAlgo)
	require.NoError(t, err)
	assert.True(t, relevant)
}

// --- Not signature-relevant (safe changes) ---

func TestNotSignatureRelevant_SchemaVersionChange(t *testing.T) {
	original := baseDescriptor()
	edited := baseDescriptor()
	edited.Meta.Version = "v3"

	relevant, err := isSignatureRelevant(original, edited, defaultAlgo)
	require.NoError(t, err)
	assert.False(t, relevant)
}

func TestNotSignatureRelevant_RepositoryContextsChange(t *testing.T) {
	original := baseDescriptor()
	edited := baseDescriptor()
	edited.Component.RepositoryContexts = []*runtime.Raw{
		{Data: []byte(`{"type":"oci","baseUrl":"ghcr.io"}`)},
	}

	relevant, err := isSignatureRelevant(original, edited, defaultAlgo)
	require.NoError(t, err)
	assert.False(t, relevant)
}

func TestNotSignatureRelevant_NonSigningLabelChange(t *testing.T) {
	original := baseDescriptor()
	original.Component.Labels = []v2.Label{
		{Name: "info-label", Value: json.RawMessage(`"old"`)},
	}
	edited := baseDescriptor()
	edited.Component.Labels = []v2.Label{
		{Name: "info-label", Value: json.RawMessage(`"new"`)},
	}

	relevant, err := isSignatureRelevant(original, edited, defaultAlgo)
	require.NoError(t, err)
	assert.False(t, relevant)
}

func TestNotSignatureRelevant_SrcRefsChange(t *testing.T) {
	original := baseDescriptor()
	edited := baseDescriptor()
	edited.Component.Resources[0].SourceRefs = []v2.SourceRef{
		{IdentitySelector: map[string]string{"name": "my-source"}},
	}

	relevant, err := isSignatureRelevant(original, edited, defaultAlgo)
	require.NoError(t, err)
	assert.False(t, relevant)
}

func TestNotSignatureRelevant_SignaturesChange(t *testing.T) {
	original := baseDescriptor()
	edited := baseDescriptor()
	edited.Signatures = nil

	relevant, err := isSignatureRelevant(original, edited, defaultAlgo)
	require.NoError(t, err)
	assert.False(t, relevant)
}

func TestNotSignatureRelevant_AccessChange(t *testing.T) {
	original := baseDescriptor()
	edited := baseDescriptor()
	edited.Component.Resources[0].Access = &runtime.Raw{Data: []byte(`{"type":"ociArtifact"}`)}

	relevant, err := isSignatureRelevant(original, edited, defaultAlgo)
	require.NoError(t, err)
	assert.False(t, relevant, "access is excluded from normalization")
}

// --- findAccessChanges tests ---

func TestAccessChanges_ResourceAccessChanged(t *testing.T) {
	original := baseDescriptor()
	edited := baseDescriptor()
	edited.Component.Resources[0].Access = &runtime.Raw{Data: []byte(`{"type":"ociArtifact","imageReference":"ghcr.io/test:1.0"}`)}

	changes := findAccessChanges(original, edited)

	assert.Len(t, changes, 1)
	assert.Equal(t, "component.resources[my-image].access", changes[0])
}

func TestAccessChanges_SourceAccessChanged(t *testing.T) {
	original := baseDescriptor()
	edited := baseDescriptor()
	edited.Component.Sources[0].Access = &runtime.Raw{Data: []byte(`{"type":"gitHub","repoUrl":"https://github.com/other"}`)}

	changes := findAccessChanges(original, edited)

	assert.Len(t, changes, 1)
	assert.Equal(t, "component.sources[my-source].access", changes[0])
}

func TestAccessChanges_NoChanges(t *testing.T) {
	desc := baseDescriptor()
	changes := findAccessChanges(desc, desc)
	assert.Empty(t, changes)
}

func TestAccessChanges_ResourceWithExtraIdentity(t *testing.T) {
	original := baseDescriptor()
	original.Component.Resources[0].ExtraIdentity = runtime.Identity{"platform": "linux/amd64"}

	edited := baseDescriptor()
	edited.Component.Resources[0].ExtraIdentity = runtime.Identity{"platform": "linux/amd64"}
	edited.Component.Resources[0].Access = &runtime.Raw{Data: []byte(`{"type":"ociArtifact"}`)}

	changes := findAccessChanges(original, edited)

	assert.Len(t, changes, 1)
	assert.Contains(t, changes[0], `my-image+{"platform":"linux/amd64"}`)
}
