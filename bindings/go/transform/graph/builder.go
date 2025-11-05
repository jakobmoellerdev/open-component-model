package graph

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/google/cel-go/cel"
	"github.com/invopop/jsonschema"
	"ocm.software/open-component-model/bindings/go/cel/ast"
	ocmcel "ocm.software/open-component-model/bindings/go/cel/environment"
	"ocm.software/open-component-model/bindings/go/cel/parser"
	"ocm.software/open-component-model/bindings/go/dag"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/runtime"
	"ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1"
	"ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1/meta"
	transfomations "ocm.software/open-component-model/bindings/go/transform/spec/v1alpha1/transformations"
	"sigs.k8s.io/yaml"
)

const (
	AttributeTransformationOrder = "transformation/order"
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
	meta           meta.TransformationMeta
	specSchema     *jsonschema.Schema
	originalObject map[string]interface{}
	variables      []*parser.TransformationField
	dependencies   []string
	order          int
}

// HasDependency checks if the resource has a dependency on another resource.
func (r *Transformation) HasDependency(dep string) bool {
	for _, d := range r.dependencies {
		if d == dep {
			return true
		}
	}
	return false
}

// AddDependency adds a dependency to the resource.
func (r *Transformation) addDependency(dep string) {
	if !r.HasDependency(dep) {
		r.dependencies = append(r.dependencies, dep)
	}
}

// addDependencies adds multiple dependencies to the resource.
func (r *Transformation) addDependencies(deps ...string) {
	for _, dep := range deps {
		r.addDependency(dep)
	}
}

type Builder struct {
	transformerScheme *runtime.Scheme
	// TODO reduce to interface
	pm *manager.PluginManager
}

func (b *Builder) NewTransferGraph(original *v1alpha1.TransformationGraphDefinition) (*Graph, error) {
	tgd := original.DeepCopy()

	// For each resource in the resource graph definition, we need to:
	// 1. Based on the transformer type, we need to load the transformations schema for the plugin.
	// 2. Extract the CEL expressions from the transformations + validate them.

	transformations := make(map[string]*Transformation)
	for order, transformation := range tgd.Transformations {
		t, err := b.buildTGTransformation(transformation, order)
		if err != nil {
			return nil, fmt.Errorf("failed to build transformation: %w", err)
		}
		id := t.meta.ID
		if transformations[id] != nil {
			return nil, fmt.Errorf("duplicate transformations id %q", id)
		}
		transformations[id] = t
	}

	err := validateTransformationGraphDefinitionNamingConventions(transformations)
	if err != nil {
		return nil, fmt.Errorf("failed to validate transformations graph definition: %w", err)
	}
	// TODO(fabianburth): i think we also need to generate a schema for the
	//  environment section, so that expressions suchs as ${env.someStruct.field}
	//  can be type checked properly.

	// collect all OpenAPI schemas for CEL type checking. This map will be used to
	// create a typed CEL environment that validates expressions against the actual
	// resource schemas.
	schemas := make(map[string]*jsonschema.Schema)
	for id, transformation := range transformations {
		if transformation.specSchema != nil {
			schemas[id] = transformation.specSchema
		}
	}

	dag, err := b.buildDependencyGraph(transformations)
	if err != nil {
		return nil, fmt.Errorf("failed to build dependency graph: %w", err)
	}

	topologicalOrder, err := dag.TopologicalSort()
	if err != nil {
		return nil, fmt.Errorf("failed to compute topological order: %w", err)
	}

	// Now that we know all resources are properly declared and dependencies are valid,
	// we can perform type checking on the CEL expressions.

	// Create a typed CEL environment with all resource schemas for template expressions
	templatesEnv, err := ocmcel.TypedEnvironment(schemas)
	if err != nil {
		return nil, fmt.Errorf("failed to create typed CEL environment: %w", err)
	}

	// Validate all CEL expressions for each resource node
	for _, transformation := range transformations {
		if err := validateNode(transformation, templatesEnv, nil, schemas[transformation.meta.ID]); err != nil {
			return nil, fmt.Errorf("failed to validate node %q: %w", transformation.meta.ID, err)
		}
	}

	transformationGraphDefinition := &Graph{
		DAG:              dag,
		Transformations:  transformations,
		TopologicalOrder: topologicalOrder,
	}
	return transformationGraphDefinition, nil
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
		jsonSchema, err := b.pm.ComponentVersionRepositoryRegistry.GetJSONSchema(context.Background(), ttransformation.Spec.Repository)
		if err != nil {
			return nil, fmt.Errorf("failed to get JSON schema for repository %s: %w", ttransformation.Spec.Repository, err)
		}
		reflector := jsonschema.Reflector{
			DoNotReference: true,
			Anonymous:      true,
			IgnoredTypes:   []any{&runtime.Raw{}},
		}
		transformationSpecJSONSchema := reflector.Reflect(ttransformation.Spec)
		raw, err := json.Marshal(transformationSpecJSONSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal transformation spec JSON schema: %w", err)
		}
		_ = raw
		transformationSpecJSONSchema.Properties.Set("repository", jsonSchema)
		raw, err = json.Marshal(transformationSpecJSONSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal transformation spec JSON schema: %w", err)
		}

		transformationSpecObject := map[string]interface{}{}
		rawSpec, err := yaml.Marshal(ttransformation.Spec)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal transformation spec to yaml: %w", err)
		}
		if err := yaml.Unmarshal(rawSpec, &transformationSpecObject); err != nil {
			return nil, fmt.Errorf("failed to unmarshal transformation spec to map: %w", err)
		}

		fieldDescriptors, err := parser.ParseResource(transformationSpecObject, transformationSpecJSONSchema)
		if err != nil {
			return nil, fmt.Errorf("failed to parse transformation spec for transformation of type %s: %w", typ, err)
		}
		var templateVariables []*parser.TransformationField
		for _, fieldDescriptor := range fieldDescriptors {
			templateVariables = append(templateVariables, &parser.TransformationField{
				Kind:            parser.ResourceVariableKindStatic,
				FieldDescriptor: fieldDescriptor,
			})
		}
		return &Transformation{
			meta:           ttransformation.GetTransformationMeta(),
			specSchema:     transformationSpecJSONSchema,
			originalObject: transformationSpecObject,
			variables:      templateVariables,
			order:          order,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported transformation type %s", typ)
	}
}

// buildDependencyGraph builds the dependency graph between the resources in the
// resource graph definition.
// The dependency graph is a directed acyclic graph that represents
// the relationships between the resources in the resource graph definition.
// The graph is used
// to determine the order in which the resources should be created in the cluster.
//
// This function returns the DAG, and a map of runtime variables per resource.
// Later
//
//	on, we'll use this map to resolve the runtime variables.
func (b *Builder) buildDependencyGraph(
	transformations map[string]*Transformation,
) (
	*dag.DirectedAcyclicGraph[string], // directed acyclic graph
	error,
) {

	transformationNames := slices.Collect(maps.Keys(transformations))
	// We also want to allow users to refer to the instance spec in their expressions.
	transformationNames = append(transformationNames, "schema")

	env, err := ocmcel.DefaultEnvironment()
	if err != nil {
		return nil, fmt.Errorf("failed to create CEL environment: %w", err)
	}

	directedAcyclicGraph := dag.NewDirectedAcyclicGraph[string]()
	// Set the vertices of the graph to be the resources defined in the resource graph definition.
	for _, transformation := range transformations {
		if err := directedAcyclicGraph.AddVertex(transformation.meta.ID, map[string]any{AttributeTransformationOrder: transformation.order}); err != nil {
			return nil, fmt.Errorf("failed to add vertex to graph: %w", err)
		}
	}

	for _, transformation := range transformations {
		for _, templateVariable := range transformation.variables {
			for _, expression := range templateVariable.Expressions {
				// We need to extract the dependencies from the expression.
				resourceDependencies, isStatic, err := extractDependencies(env, expression, transformationNames)
				if err != nil {
					return nil, fmt.Errorf("failed to extract dependencies: %w", err)
				}

				// Static until proven dynamic.
				//
				// This reads as: If the expression is dynamic and the template variable is
				// static, then we need to mark the template variable as dynamic.
				if !isStatic && templateVariable.Kind == parser.ResourceVariableKindStatic {
					templateVariable.Kind = parser.ResourceVariableKindDynamic
				}

				transformation.addDependencies(resourceDependencies...)
				templateVariable.AddDependencies(resourceDependencies...)
				// We need to add the dependencies to the graph.
				for _, dependency := range resourceDependencies {
					if err := directedAcyclicGraph.AddEdge(transformation.meta.ID, dependency); err != nil {
						return nil, err
					}
				}
			}
		}
	}

	return directedAcyclicGraph, nil
}

// extractDependencies extracts the dependencies from the given CEL expression.
// It returns a list of dependencies and a boolean indicating if the expression
// is static or not.
func extractDependencies(env *cel.Env, expression string, resourceNames []string) ([]string, bool, error) {
	// We also want to allow users to refer to the instance spec in their expressions.
	inspector := ast.NewInspectorWithEnv(env, resourceNames)

	// The CEL expression is valid if it refers to the resources defined in the
	// resource graph definition.
	inspectionResult, err := inspector.Inspect(expression)
	if err != nil {
		return nil, false, fmt.Errorf("failed to inspect expression: %w", err)
	}

	isStatic := true
	dependencies := make([]string, 0)
	for _, resource := range inspectionResult.ResourceDependencies {
		if resource.ID != "schema" && !slices.Contains(dependencies, resource.ID) {
			isStatic = false
			dependencies = append(dependencies, resource.ID)
		}
	}
	if len(inspectionResult.UnknownResources) > 0 {
		return nil, false, fmt.Errorf("found unknown resources in CEL expression: [%v]", inspectionResult.UnknownResources)
	}
	if len(inspectionResult.UnknownFunctions) > 0 {
		return nil, false, fmt.Errorf("found unknown functions in CEL expression: [%v]", inspectionResult.UnknownFunctions)
	}
	return dependencies, isStatic, nil
}

// validateNode validates all CEL expressions for a single resource node:
// - Template expressions (resource field values)
// - includeWhen expressions (conditional resource creation)
// - readyWhen expressions (resource readiness conditions)
func validateNode(resource *Transformation, templatesEnv, schemaEnv *cel.Env, transformationSpecSchema *jsonschema.Schema) error {
	// Validate template expressions
	if err := validateTemplateExpressions(templatesEnv, resource); err != nil {
		return err
	}

	return nil
}

// validateTemplateExpressions validates CEL template expressions for a single resource.
// It type-checks that expressions reference valid fields and return the expected types
// based on the OpenAPI schemas.
func validateTemplateExpressions(env *cel.Env, resource *Transformation) error {
	for _, templateVariable := range resource.variables {
		if len(templateVariable.Expressions) == 1 {
			// Single expression - validate against expected types
			expression := templateVariable.Expressions[0]

			checkedAST, err := parseAndCheckCELExpression(env, expression)
			if err != nil {
				return fmt.Errorf("failed to type-check template expression %q at path %q: %w", expression, templateVariable.Path, err)
			}

			outputType := checkedAST.OutputType()
			if err := validateExpressionType(outputType, templateVariable.ExpectedType, expression, resource.meta.ID, templateVariable.Path); err != nil {
				return err
			}
		} else if len(templateVariable.Expressions) > 1 {
			// Multiple expressions - all must be strings for concatenation
			for _, expression := range templateVariable.Expressions {
				checkedAST, err := parseAndCheckCELExpression(env, expression)
				if err != nil {
					return fmt.Errorf("failed to type-check template expression %q at path %q: %w", expression, templateVariable.Path, err)
				}

				outputType := checkedAST.OutputType()
				if err := validateExpressionType(outputType, templateVariable.ExpectedType, expression, resource.meta.ID, templateVariable.Path); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// validateExpressionType verifies that the CEL expression output type matches
// the expected type. Returns an error if there is a type mismatch.
func validateExpressionType(outputType, expectedType *cel.Type, expression, resourceID, path string) error {
	if expectedType.IsAssignableType(outputType) {
		return nil
	}

	// "dyn" is a special case. output type matches anything (x-kubernetes-int-or-string, etc)
	if outputType.String() == "dyn" {
		return nil
	}

	// Check if unwrapping would fix the type mismatch - provide helpful error message
	if ocmcel.WouldMatchIfUnwrapped(outputType, expectedType) {
		return fmt.Errorf(
			"type mismatch in resource %q at path %q: expression %q returns %q but field expects %q. "+
				"Use .orValue(defaultValue) to unwrap the optional type, e.g., %s.orValue(\"\")",
			resourceID, path, expression, outputType.String(), expectedType.String(), expression,
		)
	}

	// Type mismatch - construct helpful error message. This will surface to users.
	return fmt.Errorf(
		"type mismatch in resource %q at path %q: expression %q returns type %q but expected %q",
		resourceID, path, expression, outputType.String(), expectedType.String(),
	)
}

// parseAndCheckCELExpression parses and type-checks a CEL expression.
// Returns the checked AST on success, or the raw CEL error on failure.
// Callers should wrap the error with appropriate context.
func parseAndCheckCELExpression(env *cel.Env, expression string) (*cel.Ast, error) {
	parsedAST, issues := env.Parse(expression)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}

	checkedAST, issues := env.Check(parsedAST)
	if issues != nil && issues.Err() != nil {
		return nil, issues.Err()
	}

	return checkedAST, nil
}
