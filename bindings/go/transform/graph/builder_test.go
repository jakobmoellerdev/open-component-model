package graph

import (
	"testing"

	"github.com/stretchr/testify/require"
	"ocm.software/open-component-model/bindings/go/oci/repository/provider"
	"ocm.software/open-component-model/bindings/go/oci/spec/repository"
	ociv1 "ocm.software/open-component-model/bindings/go/oci/spec/repository/v1/oci"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/plugin/manager/registries/componentversionrepository"
	"ocm.software/open-component-model/bindings/go/runtime"
	"ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1"
	"ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1/transformations"
	"sigs.k8s.io/yaml"
)

func TestGraphBuilder(t *testing.T) {
	r := require.New(t)

	transformerScheme := runtime.NewScheme()
	r.NoError(transformerScheme.RegisterWithAlias(&transformations.DownloadComponentTransformation{}, runtime.NewUnversionedType(transformations.DownloadComponentTransformationType)))

	pluginManagerScheme := runtime.NewScheme()
	repository.MustAddToScheme(pluginManagerScheme)

	pluginManager := manager.NewPluginManager(t.Context())
	CachingComponentVersionRepositoryProvider := provider.NewComponentVersionRepositoryProvider()

	r.NoError(componentversionrepository.RegisterInternalComponentVersionRepositoryPlugin(
		pluginManagerScheme,
		pluginManager.ComponentVersionRepositoryRegistry,
		CachingComponentVersionRepositoryProvider,
		&ociv1.Repository{},
	))

	builder := &Builder{
		transformerScheme: transformerScheme,
		pm:                pluginManager,
	}

	transformationGraphDefinition := `
transformations:
- id: download1
  type: ocm.software.download.component
  spec:
    repository:
      type: oci
      baseUrl: "ghcr.io/open-component-model/test-components"
    component: "mycomponent"
    version: "1.0.0"
- id: download2
  type: ocm.software.download.component
  spec:
    repository: "${download1.spec.repository}"
    component: "${download1.spec.component}"
    version: "${download1.spec.version}"
- id: download3
  type: ocm.software.download.component
  spec:
	repository: "${download2.spec.repository}"
	component: "${download2.spec.component}"
	version: "${download2.spec.version}"
`
	// discover and process all over again
	// 1) build graph with dependencies based on unstructured
	// 2) build each node in topological order with context of previous nodes
	// 3) execute
	tgd := &v1alpha1.TransformationGraphDefinition{}
	r.NoError(yaml.Unmarshal([]byte(transformationGraphDefinition), tgd))
	graph, err := builder.NewTransferGraph(tgd)
	r.NoError(err)
	r.NotNil(graph)
}
