package graph

import (
	"fmt"

	"ocm.software/open-component-model/bindings/go/cel/parser"
	"ocm.software/open-component-model/bindings/go/dag"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/runtime"
	"ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1"
	"ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1/meta"
	transfomations "ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1/transformations"
)

type Graph struct {
	// DAG is the directed acyclic graph representation of the resource graph definition.
	DAG *dag.DirectedAcyclicGraph[string]
	// Transformations
	Transformations map[string]*Transformation
	// TopologicalOrder is the topological order of the resources in the resource graph definition.
	TopologicalOrder []string
}

type Transformation struct {
	meta meta.TransformationMeta
}

type Builder struct {
	transformerScheme *runtime.Scheme
	// TODO reduce to interface
	pm *manager.PluginManager
}

func (b *Builder) NewTransferGraph(original *v1alpha1.TransformationGraphDefinition) (*Graph, error) {
	tgd := original.DeepCopy()
	err := validateTransformationGraphDefinitionNamingConventions(tgd)
	if err != nil {
		return nil, fmt.Errorf("failed to validate transformations graph definition: %w", err)
	}

	// For each resource in the resource graph definition, we need to:
	// 1. Based the transformer type, we need to load the transformations schema for the plugin.
	// 2. Extract the CEL expressions from the transformations + validate them.

	transformations := make(map[string]*Transformation)
	for i, transformation := range tgd.Transformations {
		r, err := b.buildTGTransformation(transformation, i)
		if err != nil {
			return nil, fmt.Errorf("failed to build transformations %q: %w", id, err)
		}
	}
}

// buildTGTransformation builds a transformations from the given specification.
// It provides a high-level understanding of the transformations, by extracting the
// schema, and extracting the cel expressions from the schema.
func (b *Builder) buildTGTransformation(transformation *runtime.Raw, order int) (*Transformation, error) {
	typ := transformation.GetType()
	if typ.IsEmpty() {
		return nil, fmt.Errorf("transformations type is empty")
	}
	obj, err := b.transformerScheme.NewObject(typ)
	if err != nil {
		return nil, fmt.Errorf("failed to create object for transformations type %s: %w", typ, err)
	}
	if err := b.transformerScheme.Convert(transformation, obj); err != nil {
		return nil, fmt.Errorf("failed to convert transformations of type %s to object of type %s: %w", transformation.GetType(), typ, err)
	}
	ttransformation, ok := obj.(v1alpha1.Transformation)
	if !ok {
		return nil, fmt.Errorf("object of type %s is not a transformations", typ)
	}

	switch ttransformation := ttransformation.(type) {
	case *transfomations.DownloadComponentTransformation:
		// first convert repos
		jsonSchema, err := b.pm.ComponentVersionRepositoryRegistry.GetJSONSchema(ttransformation.Spec.Repository)
		parser.ParseResource()
	default:
		return nil, fmt.Errorf("unsupported transformation type %s", typ)
	}

	return &Transformation{
		order: order,
		meta:  m,
	}, nil
}
