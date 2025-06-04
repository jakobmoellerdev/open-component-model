package input

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"ocm.software/open-component-model/bindings/go/plugin/manager/contracts/input/v1"
	"ocm.software/open-component-model/bindings/go/plugin/manager/registries/plugins"
	"ocm.software/open-component-model/bindings/go/plugin/manager/types"
	"ocm.software/open-component-model/bindings/go/runtime"
)

// NewInputRepositoryRegistry creates a new registry and initializes maps.
func NewInputRepositoryRegistry(ctx context.Context) *RepositoryRegistry {
	return &RepositoryRegistry{
		ctx:                       ctx,
		registry:                  make(map[runtime.Type]types.Plugin),
		repositoryScheme:          runtime.NewScheme(runtime.WithAllowUnknown()),
		internalRepositoryPlugins: make(map[runtime.Type]v1.InputPluginContract),
		constructedPlugins:        make(map[string]*constructedPlugin),
	}
}

// RepositoryRegistry holds all plugins that implement capabilities corresponding to RepositoryPlugin operations.
type RepositoryRegistry struct {
	ctx                       context.Context
	mu                        sync.Mutex
	registry                  map[runtime.Type]types.Plugin
	internalRepositoryPlugins map[runtime.Type]v1.InputPluginContract
	repositoryScheme          *runtime.Scheme
	constructedPlugins        map[string]*constructedPlugin // running plugins
}

// InputRepositoryScheme returns the scheme used by the ResourceInput registry.
func (r *RepositoryRegistry) InputRepositoryScheme() *runtime.Scheme {
	return r.repositoryScheme
}

// AddPlugin takes a plugin discovered by the manager and puts it into the relevant internal map for
// tracking the plugin.
func (r *RepositoryRegistry) AddPlugin(plugin types.Plugin, constructionType runtime.Type) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if plugin, ok := r.registry[constructionType]; ok {
		return fmt.Errorf("plugin for construction type %q already registered with ID: %s", constructionType, plugin.ID)
	}

	r.registry[constructionType] = plugin

	return nil
}

// GetResourceInputPlugin returns ResourceInput plugins for a specific type.
func (r *RepositoryRegistry) GetResourceInputPlugin(ctx context.Context, spec runtime.Typed) (v1.ResourceInputPluginContract, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	plugin, err := r.getPlugin(ctx, spec)
	if err != nil {
		return nil, err
	}

	return plugin, nil
}

// GetSourceInputPlugin returns SourceInput plugins for a specific type.
func (r *RepositoryRegistry) GetSourceInputPlugin(ctx context.Context, spec runtime.Typed) (v1.SourceInputPluginContract, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	plugin, err := r.getPlugin(ctx, spec)
	if err != nil {
		return nil, err
	}

	return plugin, nil
}

// getPlugin returns a Construction plugin for a given type using a specific plugin storage map. It will also first look
// for existing registered internal plugins based on the type and the same registry name.
func (r *RepositoryRegistry) getPlugin(ctx context.Context, spec runtime.Typed) (v1.InputPluginContract, error) {
	if _, err := r.repositoryScheme.DefaultType(spec); err != nil {
		return nil, fmt.Errorf("failed to default type for prototype %T: %w", spec, err)
	}
	// if we find the type has been registered internally, we look for internal plugins for it.
	if typ, err := r.repositoryScheme.TypeForPrototype(spec); err == nil {
		p, ok := r.internalRepositoryPlugins[typ]
		if !ok {
			return nil, fmt.Errorf("no internal plugin registered for type %v", typ)
		}

		return p, nil
	}

	// if we don't find the type registered internally, we look for external plugins by using the type
	// from the specification.
	typ := spec.GetType()
	if typ.IsEmpty() {
		return nil, fmt.Errorf("external plugins can not be fetched without a type %T", spec)
	}

	plugin, ok := r.registry[typ]
	if !ok {
		return nil, fmt.Errorf("failed to get plugin for typ %q", typ)
	}

	if existingPlugin, ok := r.constructedPlugins[plugin.ID]; ok {
		return existingPlugin.Plugin, nil
	}

	return startAndReturnPlugin(ctx, r, &plugin)
}

// RegisterInternalInputPlugin is called to register an internal implementation for an input plugin.
func RegisterInternalInputPlugin(
	scheme *runtime.Scheme,
	r *RepositoryRegistry,
	plugin v1.InputPluginContract,
	proto runtime.Typed,
) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	typ, err := scheme.TypeForPrototype(proto)
	if err != nil {
		return fmt.Errorf("failed to get type for prototype %T: %w", proto, err)
	}

	r.internalRepositoryPlugins[typ] = plugin

	if err := r.repositoryScheme.RegisterWithAlias(proto, typ); err != nil {
		return fmt.Errorf("failed to register type %T with alias %s: %w", proto, typ, err)
	}

	return nil
}

type constructedPlugin struct {
	Plugin v1.InputPluginContract
	cmd    *exec.Cmd
}

// Shutdown will loop through all _STARTED_ plugins and will send an Interrupt signal to them.
// All plugins should handle interrupt signals gracefully. For Go, this is done automatically by
// the plugin SDK.
func (r *RepositoryRegistry) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var errs error
	for _, p := range r.constructedPlugins {
		// The plugins should handle the Interrupt signal for shutdowns.
		// TODO(Skarlso): Use context to wait for the plugin to actually shut down.
		if perr := p.cmd.Process.Signal(os.Interrupt); perr != nil {
			errs = errors.Join(errs, perr)
		}
	}

	return errs
}

func startAndReturnPlugin(ctx context.Context, r *RepositoryRegistry, plugin *types.Plugin) (v1.InputPluginContract, error) {
	if err := plugin.Cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start plugin: %s, %w", plugin.ID, err)
	}

	client, loc, err := plugins.WaitForPlugin(ctx, plugin)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for plugin to start: %w", err)
	}

	// start log streaming once the plugin is up and running.
	// use the baseCtx here from the manager here so the streaming isn't stopped when the request is stopped.
	go plugins.StartLogStreamer(r.ctx, plugin)

	// think about this better, we have a single json schema, maybe even have different maps for different types + schemas?
	var jsonSchema []byte
loop:
	for _, tps := range plugin.Types {
		for _, tp := range tps {
			jsonSchema = tp.JSONSchema
			break loop
		}
	}

	repoPlugin := NewConstructionRepositoryPlugin(client, plugin.ID, plugin.Path, plugin.Config, loc, jsonSchema)
	r.constructedPlugins[plugin.ID] = &constructedPlugin{
		Plugin: repoPlugin,
		cmd:    plugin.Cmd,
	}

	// wrap the untyped internal plugin into a typed representation.
	return repoPlugin, nil
}
