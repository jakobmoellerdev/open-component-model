// Package oci provides functionality for storing and retrieving Open Component Model (OCM) components
// using the Open Container Initiative (OCI) registry format. It implements the OCM repository interface
// using OCI registries as the underlying storage mechanism.
//
// Package Structure:
//
// The package is organized into several key components and subpackages:
//
//  1. Core Repository Implementation
//     The main repository.go file implements the core OCI repository functionality:
//     - Component version storage and retrieval
//     - Resource management
//     - OCI manifest handling
//     - Layer management
//
//     Every Repository is based on a Resolver which in turn provides the Resolver.StoreForReference.
//     This means that for every OCI reference, there is a Store implementation that backs it.
//
//     The Store abstracts OCI Operations from the Repository and provides methods for
//     - Fetching OCI Descriptors (and checking their Existence)
//     - Pushing OCI Descriptors
//     - Tagging OCI Descriptors and resolving those tags
//
//     As long as a store provides these abstractions (which are lent from ORAS) the repository
//     will be able to interact with the underlying storage as if it was an OCI registry.
//
//  2. Subpackages:
//     - access/v1: Provides version 1 of the OCI image access specification
//     - digest/v1: Handles content addressing and digest operations
//     - tar/: Manages TAR archive operations for OCI layouts
//     - ctf/: Common Transport Format Store implementation that can be used to work with CTFs as if they were OCI registires
//     - integration/: Integration tests
//
//  3. Supporting Types and Utilities:
//     - LocalBlobMemory: Manages temporary storage of local blobs
//     - ComponentConfig: Stores component-specific configuration
//     - ArtifactAnnotation: Handles OCI artifact annotations
//
//     Resources are managed through multiple types:
//     - LocalBlob: For temporary storage of resources
//     - ResourceBlob: For resource-specific operations (a blob described by an OCM resource)
//     - DescriptorBlob: For OCI descriptor management (a blob described by an OCI descriptor)
//
// Usage Example:
//
//	resolver := NewResolver(...)
//	memory := NewLocalBlobMemory()
//	repo := RepositoryFromResolverAndMemory(resolver, memory)
//
//	// Add a component version
//	err := repo.AddComponentVersion(ctx, descriptor)
//
//	// Add a local resource
//	newRes, err := repo.AddLocalResource(ctx, "component", "v1", resource, content)
//
//	// Get a component version
//	desc, err := repo.GetComponentVersion(ctx, "component", "v1")
//
//	// Get a local resource
//	blob, err := repo.GetLocalResource(ctx, "component", "v1", newRes.ElementMeta.ToIdentity())
//
// Media Types:
//
// The package defines media types for OCM components:
//   - MediaTypeComponentDescriptorV2: Media type for version 2 OCM component descriptors
//
// Annotations:
//
// The package uses specific annotations for OCI manifests:
//   - AnnotationOCMComponentVersion: Identifies the component version
//   - AnnotationOCMCreator: Identifies the creator of the OCM component
//
// Error Handling:
//
// The package provides detailed error information for various failure scenarios:
//   - Invalid component versions or resources
//   - OCI registry communication issues
//   - Resource access and storage problems
//
// Dependencies:
//
// The package relies on several external packages:
//   - github.com/opencontainers/go-digest: For content addressing
//   - github.com/opencontainers/image-spec: For OCI image specifications
//   - oras.land/oras-go: For OCI registry operations
package oci
