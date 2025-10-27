package cel

import (
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/getkin/kin-openapi/openapi3gen"
	"github.com/google/cel-go/cel"
	"github.com/google/cel-go/common/types"
	apiservercel "k8s.io/apiserver/pkg/cel"
	celopenapi "k8s.io/apiserver/pkg/cel/openapi"
	"k8s.io/apiserver/pkg/cel/openapi/resolver"
	"k8s.io/kube-openapi/pkg/validation/spec"
	"ocm.software/open-component-model/bindings/go/cel/openapi"
)

func OcmCelEnv() error {

	openapi3gen.NewSchemaRefForValue()
	refCallback := func(path string) spec.Ref {
		return spec.MustCreateRef("#/definitions/" + path)
	}

	openAPI := openapi.GetOpenAPIDefinitions(refCallback)

	s, err := resolver.PopulateRefs(func(ref string) (*spec.Schema, bool) {
		// find the schema by the ref string, and return a deep copy
		def, ok := openAPI[strings.TrimPrefix("#/definitions/", ref)]
		if !ok {
			return nil, false
		}
		s := def.Schema
		return &s, true
	}, ref)
	if err != nil {
		return nil, err
	}

	declTypes := make(map[string]*apiservercel.DeclType)
	for name, typ := range openAPI {
		declTypes[name] = celopenapi.SchemaDeclType(&typ.Schema, false)
	}
	declTypesList := slices.Collect(maps.Values(declTypes))

	base := types.NewEmptyRegistry()
	provider := apiservercel.NewDeclTypeProvider(declTypesList...)
	provider, err := provider.WithTypeProvider(base)

	envopts := []cel.EnvOption{
		cel.CustomTypeProvider(provider),
		cel.CustomTypeAdapter(provider),
		cel.Variable("funghus", declTypes["./openapi.Funghus"].CelType()),
	}

	celenv, err := cel.NewEnv(envopts...)
	if err != nil {
		return fmt.Errorf("failed to initialize new cel environment: %v", err)
	}
	ast, iss := celenv.Compile(`funghus.properties.softness == "fabian"`)
	if iss.Err() != nil {
		return fmt.Errorf("failed to compile: %v", iss.Err())
	}
	_ = ast
	return nil
}
