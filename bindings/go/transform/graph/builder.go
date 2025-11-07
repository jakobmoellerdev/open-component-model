package graph

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	invopop "github.com/invopop/jsonschema"
	"ocm.software/open-component-model/bindings/go/cel/ast"
	"ocm.software/open-component-model/bindings/go/cel/jsonschema"
	"ocm.software/open-component-model/bindings/go/cel/parser"
	"ocm.software/open-component-model/bindings/go/cel/resolver"
	"ocm.software/open-component-model/bindings/go/dag"
	syncdag "ocm.software/open-component-model/bindings/go/dag/sync"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/runtime"
	"ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1"
	transformations "ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1/transformations"
)

const (
	AttributeTransformationOrder = "transformation/order"
)

type Transformation struct {
	v1alpha1.GenericTransformation
	specSchema       *invopop.Schema
	fieldDescriptors []parser.FieldDescriptor
	expressions      []ast.ExpressionInspection
	order            int
}

type Builder struct {
	transformerScheme *runtime.Scheme
	// TODO reduce to interface
	pm *manager.PluginManager
}

func (b *Builder) NewTransferGraph(original *v1alpha1.TransformationGraphDefinition) (any, error) {
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
	/*
		ast, issues := celenv.Compile("environment.baseUrl")
		if issues.Err() != nil {
			return nil, issues.Err()
		}
		prog, err := celenv.Program(ast)
		if err != nil {
			return nil, err
		}
		val, _, err := prog.Eval(map[string]any{})
		if err != nil {
			return nil, err
		}
		_ = val*/

	builder, err := NewEnvBuilder(tgd.Environment.Data)
	if err != nil {
		return nil, err
	}
	env, err := builder.CurrentEnv()
	if err != nil {
		return nil, err
	}

	if err := discoverDependencies(graph, env); err != nil {
		return nil, fmt.Errorf("error discovering dependencies: %v", err)
	}

	synced := syncdag.ToSyncedGraph(graph)

	processor := syncdag.NewGraphProcessor(synced, &syncdag.GraphProcessorOptions[string, Transformation]{
		Processor: &Processor{
			builder:           builder,
			transformerScheme: b.transformerScheme,
			pluginManager:     b.pm,
		},
		Concurrency: 1,
	})

	if err := processor.Process(context.TODO()); err != nil {
		return nil, err
	}

	return nil, nil
}

type Processor struct {
	transformerScheme *runtime.Scheme
	pluginManager     *manager.PluginManager
	builder           *EnvBuilder
}

func (b *Processor) ProcessValue(ctx context.Context, transformation Transformation) error {
	env, err := b.builder.CurrentEnv()
	if err != nil {
		return err
	}

	expressionContext := map[string]any{}

	for _, fieldDescriptor := range transformation.fieldDescriptors {
		for _, expression := range fieldDescriptor.Expressions {
			ast, issues := env.Compile(expression)
			if issues.Err() != nil {
				return issues.Err()
			}

			prog, err := env.Program(ast)
			if err != nil {
				return err
			}
			val, _, err := prog.Eval(map[string]any{})
			if err != nil {
				// If eval fails here the program has a dynamic variable and we need to eval later.
				return err
			}
			expressionContext[expression] = val.Value()
		}
	}

	summary := resolver.NewResolver(transformation.Spec.Data, expressionContext).Resolve(transformation.fieldDescriptors)
	if err := summary.Error(); err != nil {
		return err
	}

	typ := transformation.GetType()
	if typ.IsEmpty() {
		return fmt.Errorf("transformation type after render is empty")
	}
	typedTransformation, err := b.transformerScheme.NewObject(typ)
	if err != nil {
		return fmt.Errorf("failed to create object for transformation type %s: %w", typ, err)
	}
	transformationJSONBytes, err := json.Marshal(&transformation)
	if err != nil {
		return fmt.Errorf("failed to marshal transformation json: %w", err)
	}
	rawTransformation := runtime.Raw{}
	if err := json.Unmarshal(transformationJSONBytes, &rawTransformation); err != nil {
		return fmt.Errorf("failed to unmarshal raw transformation: %w", err)
	}
	if err := b.transformerScheme.Convert(&rawTransformation, typedTransformation); err != nil {
		return fmt.Errorf("failed to convert transformation to typed object: %w", err)
	}

	switch typedTransformation := typedTransformation.(type) {
	case *transformations.DownloadComponentTransformation:
		specSchema, outputSchema, err := downloadComponentTransformationJSONSchema(b.pluginManager, typedTransformation)
		if err != nil {
			return fmt.Errorf("failed to get JSON schema for DownloadComponentTransformation: %w", err)
		}

		transformationSchema := &invopop.Schema{
			Type:       "object",
			Properties: invopop.NewProperties(),
			Required:   []string{"spec", "output"},
		}
		transformationSchema.Properties.Set("spec", specSchema)
		transformationSchema.Properties.Set("output", outputSchema)
		transformationDeclType := jsonschema.DeclTypeFromInvopop(transformationSchema)
		transformationDeclType = transformationDeclType.MaybeAssignTypeName("__type_" + transformation.ID)
		b.builder.RegisterDeclTypes(transformationDeclType)
		/*b.builder.RegisterEnvOption(cel.Variable(transformation.ID, transformationDeclType.CelType()))*/

		transformationVariable := cel.Constant(
			transformation.ID, transformationDeclType.CelType(), types.NewStringInterfaceMap(types.DefaultTypeAdapter, map[string]any{
				"spec":   transformation.Spec.Data,
				"output": make(map[string]interface{}),
			}),
		)
		b.builder.RegisterEnvOption(transformationVariable)
	default:
		return fmt.Errorf("unsupported transformation type %s", typ)
	}

	return nil
}

func downloadComponentTransformationJSONSchema(
	pluginManager *manager.PluginManager,
	typedTransformation *transformations.DownloadComponentTransformation,
) (*invopop.Schema, *invopop.Schema, error) {
	// first convert repos
	descriptorSchemas, err := pluginManager.ComponentVersionRepositoryRegistry.GetJSONSchema(context.TODO(), typedTransformation.Spec.Repository)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get JSON schema for repository %s: %w", typedTransformation.Spec.Repository, err)
	}
	reflector := invopop.Reflector{
		DoNotReference: true,
		Anonymous:      true,
		IgnoredTypes:   []any{&runtime.Raw{}},
	}
	transformationSpecJSONSchema := reflector.Reflect(typedTransformation.Spec)
	transformationSpecJSONSchema.Properties.Set("repository", descriptorSchemas.RepositorySchema)
	return transformationSpecJSONSchema, descriptorSchemas.DescriptorSchema, nil
}

type EnvBuilder struct {
	declTypes  []*jsonschema.DeclType
	envOptions []cel.EnvOption
}

func NewEnvBuilder(staticEnvironment map[string]interface{}) (*EnvBuilder, error) {
	schema, err := jsonschema.InferFromGoValue(staticEnvironment)
	if err != nil {
		return nil, err
	}
	envDeclType := jsonschema.DeclTypeFromInvopop(schema)
	envDeclType = envDeclType.MaybeAssignTypeName("__type_environment")
	staticEnvVal := types.DefaultTypeAdapter.NativeToValue(staticEnvironment)
	staticEnvConstant := cel.Constant("environment", envDeclType.CelType(), staticEnvVal)

	return &EnvBuilder{
		declTypes:  []*jsonschema.DeclType{envDeclType},
		envOptions: []cel.EnvOption{staticEnvConstant},
	}, nil
}

func (envBuilder *EnvBuilder) RegisterDeclTypes(declTypes ...*jsonschema.DeclType) *EnvBuilder {
	envBuilder.declTypes = append(envBuilder.declTypes, declTypes...)
	return envBuilder
}

func (envBuilder *EnvBuilder) RegisterEnvOption(envOptions ...cel.EnvOption) *EnvBuilder {
	envBuilder.envOptions = append(envBuilder.envOptions, envOptions...)
	return envBuilder
}

func (envBuilder *EnvBuilder) CurrentEnv() (*cel.Env, error) {
	baseEnv, err := cel.NewEnv()
	if err != nil {
		return nil, err
	}
	provider := jsonschema.NewDeclTypeProvider(envBuilder.declTypes...)
	opts, err := provider.EnvOptions(baseEnv.CELTypeProvider())
	if err != nil {
		return nil, err
	}
	newEnv, err := baseEnv.Extend(append(opts, envBuilder.envOptions...)...)
	if err != nil {
		return nil, err
	}
	return newEnv, nil
}
