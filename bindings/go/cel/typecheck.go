package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
	invopop "github.com/invopop/jsonschema"
	"ocm.software/open-component-model/bindings/go/cel/jsonschema"
	"ocm.software/open-component-model/bindings/go/cel/parser"
)

type Specification struct {
	Organism Organism `json:"organism"`
}
type Organism struct {
	Name     string `json:"name"`
	LastName string `json:"lastName"`
}

//type Species struct {
//	Type   string  `json:"type"`
//	Height int     `json:"height"`
//	Weight float64 `json:"weight"`
//}

func OcmCelEnv() error {
	reflector := invopop.Reflector{
		DoNotReference: true,
	}
	jsonSchema := reflector.Reflect(&Specification{})
	declType := jsonschema.SchemaDeclType(&jsonschema.Schema{JSONSchema: jsonSchema})
	declType = declType.MaybeAssignTypeName("__type_organism")

	stdEnv, err := cel.NewEnv()
	if err != nil {
		return fmt.Errorf("failed to create base cel environment: %v", err)
	}
	provider := jsonschema.NewDeclTypeProvider(declType)
	opts, err := provider.EnvOptions(stdEnv.CELTypeProvider())
	if err != nil {
		return fmt.Errorf("failed to create cel environment options: %v", err)
	}
	opts = append(opts, cel.Variable("spec", declType.CelType()))
	env, err := stdEnv.Extend(opts...)
	if err != nil {
		return fmt.Errorf("failed to extend cel environment: %v", err)
	}

	resourceObject := map[string]interface{}{
		"organism": map[string]interface{}{
			"name":     "fabian",
			"lastName": "${organism.name}",
		},
	}

	fieldDesc, err := parser.ParseResource(resourceObject, jsonSchema)
	_ = fieldDesc

	ast, iss := env.Compile(`spec.organism.lastName`)
	if iss != nil && iss.Err() != nil {
		return fmt.Errorf("failed to compile expression: %v", iss.Err())
	}

	outputType := ast.OutputType()
	expectedType := fieldDesc[0].ExpectedType
	if !expectedType.IsAssignableType(outputType) {
		return fmt.Errorf("expected type %v but got %v", expectedType, outputType)
	}

	//prog, err := env.Program(ast)
	//if err != nil {
	//	return fmt.Errorf("failed to create cel program: %v", err)
	//}
	//
	//val, details, err := prog.Eval(map[string]interface{}{
	//	"person": map[string]interface{}{
	//		"name": "John Doe",
	//	},
	//})
	//if err != nil {
	//	return fmt.Errorf("failed to evaluate cel program: %v", err)
	//}
	//_, _ = val, details
	return nil
}
