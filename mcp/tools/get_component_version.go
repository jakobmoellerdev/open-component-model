package tools

import (
	"context"
	"encoding/json"
	"fmt"

	descruntime "ocm.software/open-component-model/bindings/go/descriptor/runtime"
	descriptorv2 "ocm.software/open-component-model/bindings/go/descriptor/v2"
	"ocm.software/open-component-model/bindings/go/oci/compref"
	ocirepository "ocm.software/open-component-model/bindings/go/oci/spec/repository"
	"ocm.software/open-component-model/bindings/go/repository/component/resolvers"
	"ocm.software/open-component-model/bindings/go/runtime"
)

// GetComponentVersion fetches a complete component descriptor and returns it as JSON text.
func GetComponentVersion(ctx context.Context, deps Deps, input json.RawMessage) (string, error) {
	var args struct {
		Reference string `json:"reference"`
	}
	if err := json.Unmarshal(input, &args); err != nil {
		return "", fmt.Errorf("invalid arguments: %w", err)
	}
	if args.Reference == "" {
		return "", fmt.Errorf("reference is required")
	}

	desc, err := fetchDescriptor(ctx, deps, args.Reference)
	if err != nil {
		return "", err
	}

	v2desc, err := descruntime.ConvertToV2(descriptorv2.Scheme, desc)
	if err != nil {
		return "", fmt.Errorf("converting descriptor to v2: %w", err)
	}

	out, err := json.MarshalIndent(v2desc, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshaling descriptor: %w", err)
	}
	return string(out), nil
}

// fetchDescriptor is a shared helper that resolves a component ref and fetches its descriptor.
func fetchDescriptor(ctx context.Context, deps Deps, reference string) (*descruntime.Descriptor, error) {
	ref, err := compref.Parse(reference, compref.IgnoreSemverCompatibility())
	if err != nil {
		return nil, fmt.Errorf("parsing reference %q: %w", reference, err)
	}

	resolver, err := newResolver(ctx, deps, ref)
	if err != nil {
		return nil, fmt.Errorf("creating resolver: %w", err)
	}

	repo, err := resolver.GetComponentVersionRepositoryForComponent(ctx, ref.Component, ref.Version)
	if err != nil {
		return nil, fmt.Errorf("accessing repository for %q: %w", reference, err)
	}

	desc, err := repo.GetComponentVersion(ctx, ref.Component, ref.Version)
	if err != nil {
		return nil, fmt.Errorf("getting component version %q: %w", reference, err)
	}
	return desc, nil
}

// newResolver creates a component version repository resolver from a parsed reference,
// mirroring the behavior of cli/internal/repository/ocm.NewComponentVersionRepositoryForComponentProvider.
func newResolver(ctx context.Context, deps Deps, ref *compref.Ref) (resolvers.ComponentVersionRepositoryResolver, error) {
	// ExtractResolvers returns (fallbackResolvers, pathMatcherResolvers, error)
	fallbackResolvers, pathMatchers, err := resolvers.ExtractResolvers(deps.Config, ocirepository.Scheme)
	if err != nil {
		return nil, fmt.Errorf("extracting resolvers from config: %w", err)
	}

	opts := resolvers.Options{
		RepoProvider:      deps.PluginManager.ComponentVersionRepositoryRegistry,
		CredentialGraph:   deps.CredentialGraph,
		PathMatchers:      pathMatchers,
		FallbackResolvers: fallbackResolvers,
	}

	if ref != nil && ref.Component != "" {
		opts.ComponentPatterns = []string{ref.Component}
	}

	var baseRepo runtime.Typed
	if ref != nil && ref.Repository != nil {
		baseRepo = ref.Repository
	}

	return resolvers.New(ctx, opts, baseRepo)
}
