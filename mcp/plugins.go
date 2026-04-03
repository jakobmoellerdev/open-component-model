package main

import (
	filesystemv1alpha1 "ocm.software/open-component-model/bindings/go/configuration/filesystem/v1alpha1/spec"
	ocicredentials "ocm.software/open-component-model/bindings/go/oci/credentials"
	"ocm.software/open-component-model/bindings/go/oci/repository/provider"
	ocicredentialsspec "ocm.software/open-component-model/bindings/go/oci/spec/credentials"
	v1 "ocm.software/open-component-model/bindings/go/oci/spec/credentials/identity/v1"
	ocicredentialsspecv1 "ocm.software/open-component-model/bindings/go/oci/spec/credentials/v1"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/runtime"
)

// registerBuiltinPlugins registers the essential built-in plugins required for the MCP server
// to access OCI registries and CTF archives without needing external plugin binaries.
func registerBuiltinPlugins(pm *manager.PluginManager, _ *filesystemv1alpha1.Config) error {
	// Register the OCI component version repository plugin (reads/writes OCI registries and CTF archives).
	ociProvider := provider.NewComponentVersionRepositoryProvider()
	if err := pm.ComponentVersionRepositoryRegistry.RegisterInternalComponentVersionRepositoryPlugin(ociProvider); err != nil {
		return err
	}

	// Register the OCI credential repository plugin (reads Docker config.json for registry credentials).
	scheme := runtime.NewScheme()
	scheme.MustRegisterWithAlias(&ocicredentialsspecv1.DockerConfig{}, ocicredentialsspec.CredentialRepositoryConfigType)
	return pm.CredentialRepositoryRegistry.RegisterInternalCredentialRepositoryPlugin(
		&ocicredentials.OCICredentialRepository{},
		[]runtime.Type{v1.Type},
	)
}
