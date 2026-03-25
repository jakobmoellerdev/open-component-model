package transfer

import (
	"context"

	"ocm.software/open-component-model/bindings/go/repository"
	"ocm.software/open-component-model/bindings/go/repository/component/resolvers"
	"ocm.software/open-component-model/bindings/go/runtime"
)

// ComponentID identifies a single component version to transfer.
type ComponentID struct {
	// Component is the component name (e.g., "ocm.software/mycomponent").
	Component string

	// Version is the semantic version (e.g., "1.0.0").
	Version string
}

// String returns the "component:version" key form used internally for DAG roots.
func (c ComponentID) String() string {
	return c.Component + ":" + c.Version
}

// ComponentLister enumerates component versions to be transferred.
// Implementations may list from a CTF, a registry catalog, a file, etc.
type ComponentLister interface {
	// ListComponentVersions calls fn with batches of component versions to transfer.
	// Iteration stops when fn returns an error or all components have been listed.
	ListComponentVersions(ctx context.Context, fn func(ids []ComponentID) error) error
}

// ComponentListerFunc adapts a function to the [ComponentLister] interface.
type ComponentListerFunc func(ctx context.Context, fn func(ids []ComponentID) error) error

// ListComponentVersions calls the underlying function.
func (f ComponentListerFunc) ListComponentVersions(ctx context.Context, fn func(ids []ComponentID) error) error {
	return f(ctx, fn)
}

// Mapping pairs source components with a target repository and a resolver.
type Mapping struct {
	// Components specifies the source component versions.
	Components []ComponentID

	// ComponentLister dynamically enumerates source component versions.
	// Cannot be combined with Components.
	ComponentLister ComponentLister

	// Target is the target repository specification.
	Target runtime.Typed

	// Resolver resolves component versions from the source repository.
	Resolver resolvers.ComponentVersionRepositoryResolver
}

// TransferOption configures a [Mapping].
type TransferOption func(*Mapping)

// Component adds a source component version to a transfer mapping.
func Component(component, version string) TransferOption {
	return func(m *Mapping) {
		m.Components = append(m.Components, ComponentID{Component: component, Version: version})
	}
}

// ToRepositorySpec sets the target repository specification for a transfer mapping.
func ToRepositorySpec(target runtime.Typed) TransferOption {
	return func(m *Mapping) {
		m.Target = target
	}
}

// FromResolver sets an explicit resolver for this mapping's source components.
func FromResolver(r resolvers.ComponentVersionRepositoryResolver) TransferOption {
	return func(m *Mapping) {
		m.Resolver = r
	}
}

// FromRepository wraps a [repository.ComponentVersionRepository] in a simple resolver
// and sets it on the mapping. This is a convenience for simple single-repository sources.
func FromRepository(repo repository.ComponentVersionRepository) TransferOption {
	return func(m *Mapping) {
		m.Resolver = &repoResolver{repo: repo}
	}
}

// repoResolver wraps a single ComponentVersionRepository as a ComponentVersionRepositoryResolver.
type repoResolver struct {
	repo repository.ComponentVersionRepository
}

func (r *repoResolver) GetComponentVersionRepositoryForComponent(_ context.Context, _, _ string) (repository.ComponentVersionRepository, error) {
	return r.repo, nil
}

func (r *repoResolver) GetComponentVersionRepositoryForSpecification(_ context.Context, _ runtime.Typed) (repository.ComponentVersionRepository, error) {
	return r.repo, nil
}

func (r *repoResolver) GetRepositorySpecificationForComponent(_ context.Context, _, _ string) (runtime.Typed, error) {
	// The source repo spec will be determined by the resolver used during discovery.
	// For a simple repo wrapper, we return nil — the internal resolver handles this.
	return nil, nil
}

var _ resolvers.ComponentVersionRepositoryResolver = (*repoResolver)(nil)
