package cel

import (
	"fmt"

	"github.com/google/cel-go/cel"
	invopop "github.com/invopop/jsonschema"
	"ocm.software/open-component-model/bindings/go/cel/jsonschema"
)

type Organism struct {
	Name    string  `json:"name"`
	Address Address `json:"address"`
}

type Address struct {
	Street string `json:"street"`
	City   string `json:"city"`
}

//type Unstructured map[string]interface{}
//type Typed struct {
//	Typ          runtime.Type `json:"type"`
//	Unstructured `json:",inline"`
//}

func OcmCelEnv() error {
	reflector := invopop.Reflector{
		DoNotReference: true,
	}
	jsonSchema := reflector.Reflect(&Organism{})
	declType := SchemaDeclType(&jsonschema.Schema{JSONSchema: jsonSchema})
	declType = declType.MaybeAssignTypeName("__type_organism")

	//base := types.NewEmptyRegistry()
	stdEnv, err := cel.NewEnv()
	if err != nil {
		return fmt.Errorf("failed to create base cel environment: %v", err)
	}
	provider := NewDeclTypeProvider(declType)
	opts, err := provider.EnvOptions(stdEnv.CELTypeProvider())
	if err != nil {
		return fmt.Errorf("failed to create cel environment options: %v", err)
	}
	opts = append(opts, cel.Variable("organism", declType.CelType()))
	env, err := stdEnv.Extend(opts...)
	if err != nil {
		return fmt.Errorf("failed to extend cel environment: %v", err)
	}

	ast, iss := env.Compile("organism.address.street")
	if iss != nil && iss.Err() != nil {
		return fmt.Errorf("failed to compile expression: %v", iss.Err())
	}

	outputType := ast.OutputType()
	schema, _ := jsonSchema.Properties.Get("name")
	expectedType := SchemaDeclType(&jsonschema.Schema{JSONSchema: schema}).CelType()
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
