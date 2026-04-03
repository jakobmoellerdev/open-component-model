package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	filesystemv1alpha1 "ocm.software/open-component-model/bindings/go/configuration/filesystem/v1alpha1/spec"
	genericv1 "ocm.software/open-component-model/bindings/go/configuration/generic/v1/spec"
	"ocm.software/open-component-model/bindings/go/credentials"
	credentialsRuntime "ocm.software/open-component-model/bindings/go/credentials/spec/config/runtime"
	"ocm.software/open-component-model/bindings/go/plugin/manager"
	"ocm.software/open-component-model/bindings/go/runtime"
	"ocm.software/open-component-model/mcp/tools"
)

func main() {
	// All logs go to stderr — stdout is reserved for MCP JSON-RPC messages.
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelWarn})))

	ctx := context.Background()

	ocmCtx, err := initOCMContext(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize OCM context: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := ocmCtx.PluginManager.Shutdown(shutdownCtx); err != nil {
			slog.ErrorContext(shutdownCtx, "plugin manager shutdown error", "error", err)
		}
	}()

	srv := NewServer(ocmCtx)

	scanner := bufio.NewScanner(os.Stdin)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024) // 1 MiB max line
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		resp := srv.Handle(ctx, line)
		if resp == nil {
			continue
		}
		out, err := json.Marshal(resp)
		if err != nil {
			slog.ErrorContext(ctx, "failed to marshal response", "error", err)
			continue
		}
		os.Stdout.Write(out)
		os.Stdout.Write([]byte("\n"))
	}
	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "stdin read error: %v\n", err)
		os.Exit(1)
	}
}

// OCMContext holds the initialized OCM dependencies for the MCP server.
type OCMContext struct {
	PluginManager   *manager.PluginManager
	CredentialGraph credentials.Resolver
	Config          *genericv1.Config
}

func initOCMContext(ctx context.Context) (*OCMContext, error) {
	cfg, err := loadOCMConfig()
	if err != nil {
		slog.WarnContext(ctx, "could not load OCM config, using empty config", "error", err)
		cfg = &genericv1.Config{}
	}

	pm := manager.NewPluginManager(ctx)

	// Register builtin OCI plugin so we can talk to OCI registries and CTF archives
	// without needing external plugin binaries.
	fsConfig := &filesystemv1alpha1.Config{}
	if err := registerBuiltinPlugins(pm, fsConfig); err != nil {
		return nil, fmt.Errorf("registering builtin plugins: %w", err)
	}

	// Discover external plugins from the default plugin directory.
	pluginDir := os.Getenv("OCM_PLUGIN_DIRECTORY")
	if pluginDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			pluginDir = home + "/.config/ocm/plugins"
		}
	}
	if pluginDir != "" {
		if err := pm.RegisterPlugins(ctx, pluginDir); err != nil && err != manager.ErrNoPluginsFound {
			slog.WarnContext(ctx, "could not register external plugins", "dir", pluginDir, "error", err)
		}
	}

	// Build credential graph from config.
	opts := credentials.Options{
		RepositoryPluginProvider: pm.CredentialRepositoryRegistry,
		CredentialPluginProvider: credentials.GetCredentialPluginFn(
			func(_ context.Context, typed runtime.Typed) (credentials.CredentialPlugin, error) {
				return nil, fmt.Errorf("no credential plugin found for type %s", typed)
			},
		),
		CredentialRepositoryTypeScheme: pm.CredentialRepositoryRegistry.RepositoryScheme(),
	}

	var credCfg *credentialsRuntime.Config
	if cfg != nil {
		if credCfg, err = credentialsRuntime.LookupCredentialConfig(cfg); err != nil {
			slog.WarnContext(ctx, "could not load credential config", "error", err)
		}
	}
	if credCfg == nil {
		credCfg = &credentialsRuntime.Config{}
	}

	graph, err := credentials.ToGraph(ctx, credCfg, opts)
	if err != nil {
		return nil, fmt.Errorf("creating credential graph: %w", err)
	}

	return &OCMContext{
		PluginManager:   pm,
		CredentialGraph: graph,
		Config:          cfg,
	}, nil
}

// NewServer creates the MCP server with the OCM tool handlers registered.
func NewServer(ocmCtx *OCMContext) *Server {
	toolDeps := tools.Deps{
		PluginManager:   ocmCtx.PluginManager,
		CredentialGraph: ocmCtx.CredentialGraph,
		Config:          ocmCtx.Config,
	}
	return newServer(toolDeps)
}
