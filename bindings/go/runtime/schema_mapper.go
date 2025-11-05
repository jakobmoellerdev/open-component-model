package runtime

import (
	"context"
	"fmt"
	"sync"

	invopop "github.com/invopop/jsonschema"
)

type SchemaMapper interface {
	RegisterSchemaForType(ctx context.Context, typ Type, schema *invopop.Schema) error
	RegisterSchemaForPrototype(ctx context.Context, prototype Typed) error
	SchemaForType(ctx context.Context, typ Type) (*invopop.Schema, error)
	SchemaForPrototype(ctx context.Context, prototype Typed) (*invopop.Schema, error)
}

// TODO(fabianburth): add alias registration, skipped for now because external
//
//	plugins do not seems to have a way to declare aliases yet.
type DefaultSchemaMapper struct {
	mu sync.RWMutex
	// scheme is used to
	// - resolve types for prototypes
	// schemas can ONLY be registered for types known to the scheme
	scheme       *Scheme
	typeToSchema map[Type]*invopop.Schema
}

func NewDefaultSchemaMapper(scheme *Scheme) *DefaultSchemaMapper {
	return &DefaultSchemaMapper{
		scheme:       scheme,
		typeToSchema: make(map[Type]*invopop.Schema),
	}
}

func (d *DefaultSchemaMapper) RegisterSchemaForType(ctx context.Context, typ Type, schema *invopop.Schema) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, exists := d.typeToSchema[typ]; exists {
		return fmt.Errorf("type %s already registered", typ)
	}
	d.typeToSchema[typ] = schema
	return nil
}

func (d *DefaultSchemaMapper) RegisterSchemaForPrototype(ctx context.Context, prototype Typed) error {
	_, _ = d.scheme.DefaultType(prototype)
	typ := prototype.GetType()
	if typ == (Type{}) {
		return fmt.Errorf("cannot determine type for prototype of %T", prototype)
	}
	if _, exists := d.typeToSchema[typ]; exists {
		return fmt.Errorf("type %s already registered", typ)
	}

	schema, err := GenerateJSONSchemaWithScheme(d.scheme, prototype)
	if err != nil {
		return fmt.Errorf("could not generate JSON schema for %s: %w", prototype, err)
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.typeToSchema[typ] = schema
	return nil
}

func (d *DefaultSchemaMapper) SchemaForType(ctx context.Context, typ Type) (*invopop.Schema, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	schema, exists := d.typeToSchema[typ]
	if !exists {
		return nil, fmt.Errorf("no schema registered for type %s", typ)
	}
	return schema, nil
}

func (d *DefaultSchemaMapper) SchemaForPrototype(ctx context.Context, prototype Typed) (*invopop.Schema, error) {
	_, _ = d.scheme.DefaultType(prototype)
	typ := prototype.GetType()
	if typ == (Type{}) {
		return nil, fmt.Errorf("cannot determine type for prototype of %T", prototype)
	}
	return d.SchemaForType(ctx, typ)
}
