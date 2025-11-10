package graph

import (
	"context"
	"fmt"

	"github.com/google/cel-go/cel"
	"ocm.software/open-component-model/bindings/go/cel/ast"
	"ocm.software/open-component-model/bindings/go/cel/fieldpath"
	"ocm.software/open-component-model/bindings/go/cel/jsonschema"
	"ocm.software/open-component-model/bindings/go/cel/parser"
	"ocm.software/open-component-model/bindings/go/dag"
	syncdag "ocm.software/open-component-model/bindings/go/dag/sync"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/runtime"
	"ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1"
)

const (
	AttributeTransformationOrder = "transformation/order"
)

type Transformation struct {
	v1alpha1.GenericTransformation
	fieldDescriptors []parser.FieldDescriptor
	expressions      []ast.ExpressionInspection
	order            int

	declType *jsonschema.DeclType
}

type Builder struct {
	transformerScheme *runtime.Scheme
	// TODO reduce to interface
	pm *manager.PluginManager
}

type Graph struct {
	checked *dag.DirectedAcyclicGraph[string]
}

func (b *Builder) NewTransferGraph(original *v1alpha1.TransformationGraphDefinition) (*Graph, error) {
	tgd := original.DeepCopy()

	nodes, err := getTransformationNodes(tgd)
	if err != nil {
		return nil, err
	}

	graph := dag.NewDirectedAcyclicGraph[string]()
	for _, node := range nodes {
		if err := graph.AddVertex(node.ID, map[string]any{syncdag.AttributeValue: node}); err != nil {
			return nil, err
		}
	}

	builder, err := NewEnvBuilder(tgd.GetEnvironmentData())
	if err != nil {
		return nil, err
	}
	env, _, err := builder.CurrentEnv()
	if err != nil {
		return nil, err
	}

	if err := discoverDependencies(graph, env); err != nil {
		return nil, fmt.Errorf("error discovering dependencies: %v", err)
	}

	synced := syncdag.ToSyncedGraph(graph)

	processor := syncdag.NewGraphProcessor(synced, &syncdag.GraphProcessorOptions[string, Transformation]{
		Processor: &StaticPluginAnalysisProcessor{
			builder:           builder,
			transformerScheme: b.transformerScheme,
			pluginManager:     b.pm,
		},
		Concurrency: 1,
	})

	if err := processor.Process(context.TODO()); err != nil {
		return nil, err
	}

	return &Graph{
		checked: graph,
	}, nil
}

type StaticPluginAnalysisProcessor struct {
	transformerScheme *runtime.Scheme
	pluginManager     *manager.PluginManager
	builder           *EnvBuilder
}

func (b *StaticPluginAnalysisProcessor) ProcessValue(ctx context.Context, transformation Transformation) error {
	env, provider, err := b.builder.CurrentEnv()
	if err != nil {
		return err
	}

	for i, fieldDescriptor := range transformation.fieldDescriptors {
		if len(fieldDescriptor.Expressions) > 1 {
			fieldDescriptor.ExpectedType = cel.StringType
		} else {
			for _, expression := range fieldDescriptor.Expressions {
				ast, issues := env.Compile(expression)
				if issues.Err() != nil {
					return issues.Err()
				}
				fieldDescriptor.ExpectedType = ast.OutputType()
			}
		}
		transformation.fieldDescriptors[i] = fieldDescriptor
	}

	typ := transformation.GetType()
	if typ.IsEmpty() {
		return fmt.Errorf("transformation type after render is empty")
	}
	typedTransformation, err := b.transformerScheme.NewObject(typ)
	if err != nil {
		return fmt.Errorf("failed to create object for transformation type %s: %w", typ, err)
	}

	v1alpha1Transformation, ok := typedTransformation.(v1alpha1.Transformation)
	if !ok {
		return fmt.Errorf("transformation type %s is not a valid spec transformation", typ)
	}
	v1alpha1Transformation.GetTransformationMeta().ID = transformation.ID

	runtimeTypes, err := runtimeTypesFromTransformation(env, transformation, v1alpha1Transformation, provider)
	if err != nil {
		return err
	}

	// Shared schema construction + registration.
	declType, err := v1alpha1Transformation.NewDeclType(b.pluginManager, runtimeTypes)
	if err != nil {
		return err
	}
	b.builder.RegisterDeclTypes(declType)
	b.builder.RegisterEnvOption(cel.Variable(transformation.ID, declType.CelType()))
	transformation.declType = declType

	return nil
}

// ResolveRuntimeType determines the runtime.Type for a typed field
// given a declType schema, the typed field path, the descriptor path, and their match relation.
// - For matchEqual or matchPrefix: reads the discriminator from the typed field schema.
// - For matchChild: reads the discriminator from the parent of the child field (i.e. descriptor[:-1]).
// - Returns nil for other relations.
func ResolveRuntimeType(
	decl *jsonschema.DeclType,
) (*runtime.Type, error) {
	schemaNode := decl.JSONSchema
	disc, err := discriminatorConstAt(schemaNode)
	if err != nil {
		return nil, fmt.Errorf("read discriminator: %w", err)
	}

	rt, err := runtime.TypeFromString(disc)
	if err != nil {
		return nil, fmt.Errorf("invalid runtime type %q: %w", disc, err)
	}

	return &rt, nil
}

func runtimeTypesFromTransformation(
	env *cel.Env,
	transformation Transformation,
	v1alpha1 v1alpha1.Transformation,
	declTypeProvider *jsonschema.DeclTypeProvider,
) (map[string]runtime.Type, error) {
	var (
		typCandidate    *runtime.Type
		foundDependency bool
	)

	typedFields := v1alpha1.NestedTypedFields()

	for _, typedField := range typedFields {
		typedSegs, err := fieldpath.Parse(typedField)
		if err != nil {
			return nil, fmt.Errorf("parse typed field %q: %w", typedField, err)
		}

		var (
			best     *cel.Type
			bestRank = matchNone
		)

		for i := range transformation.fieldDescriptors {
			fd := &transformation.fieldDescriptors[i]
			descSegs, err := fieldpath.Parse(fd.Path)
			if err != nil {
				return nil, fmt.Errorf("parse descriptor %q: %w", fd.Path, err)
			}

			if mr := matchSegments(typedSegs, descSegs); mr != matchNone && mr > bestRank {
				if mr == matchChild {
					childExpression, err := fieldpath.Parse(fd.Expressions[0])
					if err != nil {
						return nil, fmt.Errorf("parse child expression %q: %w", fd.Expressions[0], err)
					}
					parentExpression := fieldpath.Build(childExpression[:len(childExpression)-1])
					ast, issues := env.Compile(parentExpression)
					if issues.Err() != nil {
						return nil, issues.Err()
					}
					best = ast.OutputType()
				} else {
					best = fd.ExpectedType
				}
				bestRank = mr
			}
		}

		if best == nil {
			continue
		}
		foundDependency = true

		declTyp, ok := declTypeProvider.FindDeclType(best.TypeName())
		if !ok {
			return nil, fmt.Errorf("no declType for %q", best.TypeName())
		}

		rt, err := ResolveRuntimeType(declTyp)
		if err != nil {
			return nil, fmt.Errorf("resolve runtime type for %q: %w", typedField, err)
		}
		if rt == nil {
			continue
		}

		typCandidate = rt
		break // first valid dependency is enough
	}

	// No dependency ⇒ use static type from transformation itself.
	if !foundDependency {
		// TODO use static type by going into the unstructured transformation and reading the field descriptor
	}

	if typCandidate == nil {
		return nil, fmt.Errorf("failed to resolve runtime type for transformation %q", transformation.ID)
	}

	// TODO in theory we need to pass out N types for n nested field types
	return map[string]runtime.Type{
		typedFields[0]: *typCandidate,
	}, nil
}
