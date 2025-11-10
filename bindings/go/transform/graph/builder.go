package graph

import (
	"context"
	"fmt"

	"github.com/google/cel-go/cel"
	"ocm.software/open-component-model/bindings/go/cel/ast"
	"ocm.software/open-component-model/bindings/go/cel/environment"
	"ocm.software/open-component-model/bindings/go/cel/fieldpath"
	"ocm.software/open-component-model/bindings/go/cel/jsonschema"
	"ocm.software/open-component-model/bindings/go/cel/parser"
	"ocm.software/open-component-model/bindings/go/dag"
	syncdag "ocm.software/open-component-model/bindings/go/dag/sync"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/runtime"
	"ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1"
	"ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1/transformations"
)

const (
	AttributeTransformationOrder = "transformation/order"
)

type Transformation struct {
	v1alpha1.GenericTransformation
	fieldDescriptors []parser.FieldDescriptor
	expressions      []ast.ExpressionInspection
	order            int
	prototype        runtime.Typed

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

	pluginProcessor := &StaticPluginAnalysisProcessor{
		builder:                 builder,
		transformerScheme:       b.transformerScheme,
		pluginManager:           b.pm,
		analyzedTransformations: make(map[string]Transformation),
	}

	staticAnalysisProcessor := syncdag.NewGraphProcessor(synced, &syncdag.GraphProcessorOptions[string, Transformation]{
		Processor:   pluginProcessor,
		Concurrency: 1,
	})

	if err := staticAnalysisProcessor.Process(context.TODO()); err != nil {
		return nil, err
	}

	for _, vertex := range graph.Vertices {
		vertex.Attributes[syncdag.AttributeValue] = pluginProcessor.analyzedTransformations[vertex.ID]
	}

	runtimeEvaluationProcessor := syncdag.NewGraphProcessor(synced, &syncdag.GraphProcessorOptions[string, Transformation]{
		Processor: &RuntimeEvaluationProcessor{
			builder:       builder,
			pluginManager: b.pm,
		},
		Concurrency: 1,
	})
	if err := runtimeEvaluationProcessor.Process(context.TODO()); err != nil {
		return nil, err
	}

	return &Graph{
		checked: graph,
	}, nil
}

type RuntimeEvaluationProcessor struct {
	pluginManager   *manager.PluginManager
	builder         *EnvBuilder
	transformations map[string]Transformation
}

func (b *RuntimeEvaluationProcessor) ProcessValue(ctx context.Context, transformation Transformation) error {
	env, _, err := b.builder.CurrentEnv()
	if err != nil {
		return err
	}
	switch transformation.prototype.(type) {
	case *transformations.DownloadComponentTransformation:
		for _, fieldDescriptor := range transformation.fieldDescriptors {
			for _, expression := range fieldDescriptor.Expressions {
				program, err := env.Program(expression.AST)
				if err != nil {
					return fmt.Errorf(": %w", err)
				}
				result, _, err := program.Eval(map[string]interface{}{})
				if err != nil {
					return fmt.Errorf("failed to evaluate expression %q: %w", expression.String, err)
				}
				_ = result
				// 1) use result with resolver to replace cel expressions with values
				// 2) we now have the complete spec. call transformer with that spec and write the
				//    call result into output
				// 3) write whole variable (spec + output) into a variable and add it under the
				//    transformation ID to the expression context

			}
		}
		return nil
	default:
		return nil
	}

}

type StaticPluginAnalysisProcessor struct {
	transformerScheme       *runtime.Scheme
	pluginManager           *manager.PluginManager
	builder                 *EnvBuilder
	analyzedTransformations map[string]Transformation
}

func (b *StaticPluginAnalysisProcessor) ProcessValue(ctx context.Context, transformation Transformation) error {
	env, provider, err := b.builder.CurrentEnv()
	if err != nil {
		return err
	}

	for i, fieldDescriptor := range transformation.fieldDescriptors {
		for j, expression := range fieldDescriptor.Expressions {
			ast, issues := env.Compile(expression.String)
			if issues.Err() != nil {
				return issues.Err()
			}
			fieldDescriptor.Expressions[j].AST = ast
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
	transformation.prototype = typedTransformation

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

	specSchema, ok := declType.JSONSchema.Properties.Get("spec")
	if !ok {
		return fmt.Errorf("transformation declType has no spec property")
	}
	validatedFieldDescriptors, err := parser.ParseResource(transformation.Spec.Data, specSchema)
	if err != nil {
		return fmt.Errorf("validate transformation resource against schema: %w", err)
	}
	for i, fieldDescriptor := range transformation.fieldDescriptors {
		for j, expression := range fieldDescriptor.Expressions {
			if !environment.WouldMatchIfUnwrapped(expression.AST.OutputType(), validatedFieldDescriptors[i].ExpectedType) {
				return fmt.Errorf("expression output type %s is not assignable to expected type %s for path %s based on schema",
					expression.AST.OutputType().TypeName(),
					validatedFieldDescriptors[i].ExpectedType.TypeName(),
					fieldDescriptor.Path,
				)
			}
			validatedFieldDescriptors[i].Expressions[j].AST = expression.AST
		}
	}
	transformation.fieldDescriptors = validatedFieldDescriptors

	b.analyzedTransformations[transformation.ID] = transformation

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
					// TODO(fabianburth): check how or whether we want to deal with multiple expressions here
					childExpression, err := fieldpath.Parse(fd.Expressions[0].String)
					if err != nil {
						return nil, fmt.Errorf("parse child expression %q: %w", fd.Expressions[0].String, err)
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
		rt, err := GetValueFromPath(transformation.Spec.Data, fmt.Sprintf("%s.type", typedFields[0]))
		if err != nil {
			return nil, fmt.Errorf("failed to get static runtime type for transformation %q: %w", transformation.ID, err)
		}
		rtStr, ok := rt.(string)
		if !ok {
			return nil, fmt.Errorf("static runtime type for transformation %q is not a string", transformation.ID)
		}
		parsedType, err := runtime.TypeFromString(rtStr)
		if err != nil {
			return nil, fmt.Errorf("invalid static runtime type %q for transformation %q: %w", rtStr, transformation.ID, err)
		}
		typCandidate = &parsedType
	}

	if typCandidate == nil {
		return nil, fmt.Errorf("failed to resolve runtime type for transformation %q", transformation.ID)
	}

	// TODO in theory we need to pass out N types for n nested field types
	return map[string]runtime.Type{
		typedFields[0]: *typCandidate,
	}, nil
}
