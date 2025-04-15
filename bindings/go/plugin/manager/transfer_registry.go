package manager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"slices"
	"sync"

	"ocm.software/open-component-model/bindings/go/runtime"
)

// internalComponentVersionRepositoryPlugins contains all plugins that have been registered using internally import statement.
var internalComponentVersionRepositoryPlugins map[runtime.Type]PluginBase

var internalComponentVersionRepositoryScheme = runtime.NewScheme()

// RegisterInternalComponentVersionRepositoryPlugin can be called by actual implementations in the source.
// It will register any implementations directly for a given type and capability.
func RegisterInternalComponentVersionRepositoryPlugin[T runtime.Typed](scheme *runtime.Scheme, p ReadWriteOCMRepositoryPluginContract[T], prototype T) error {
	if internalComponentVersionRepositoryPlugins == nil {
		internalComponentVersionRepositoryPlugins = make(map[runtime.Type]PluginBase)
	}
	typ, err := scheme.TypeForPrototype(prototype)
	if err != nil {
		return fmt.Errorf("failed to get type for prototype %T: %w", prototype, err)
	}

	internalComponentVersionRepositoryPlugins[typ] = p

	if err := internalComponentVersionRepositoryScheme.RegisterWithAlias(prototype, typ); err != nil {
		return fmt.Errorf("failed to register prototype %T: %w", prototype, err)
	}

	return nil
}

// ComponentVersionRepositoryRegistry holds all plugins that implement capabilities corresponding to ComponentVersionRepository operations.
type ComponentVersionRepositoryRegistry struct {
	mu                 sync.Mutex
	registry           map[runtime.Type]*Plugin
	constructedPlugins map[string]*constructedPlugin
	logger             *slog.Logger
}

// Shutdown will loop through all _STARTED_ plugins and will send an Interrupt signal to them.
// All plugins should handle interrupt signals gracefully. For Go, this is done automatically by
// the plugin SDK.
func (r *ComponentVersionRepositoryRegistry) Shutdown(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	var errs error
	for _, p := range r.constructedPlugins {
		// The plugins should handle the Interrupt signal for shutdowns.
		// TODO: Use context to wait for the plugin to actually shut down.
		if perr := p.cmd.Process.Signal(os.Interrupt); perr != nil {
			errs = errors.Join(errs, perr)
		}
	}

	return errs
}

// AddPlugin takes a plugin discovered by the manager and adds it to the stored plugin registry.
// This function will return an error if the given capability + type already has a registered plugin.
// Multiple plugins for the same cap+typ is not allowed.
func (r *ComponentVersionRepositoryRegistry) AddPlugin(plugin *Plugin, caps *Capabilities) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	var reads []runtime.Type
	var writes []runtime.Type
	for _, c := range caps.Capabilities[ComponentVersionRepositoryPlugin] {
		if c.Capability == ReadComponentVersionRepositoryCapability {
			reads = append(reads, c.Type)
		}
		if c.Capability == WriteComponentVersionRepositoryCapability {
			writes = append(reads, c.Type)
		}
	}
	candidates := make(map[runtime.Type]struct{}, len(reads))
	for _, read := range reads {
		if slices.Contains(writes, read) {
			candidates[read] = struct{}{}
		}
	}

	for candidate := range candidates {
		if p, ok := r.registry[candidate]; ok {
			return fmt.Errorf("plugin already has a type %s with plugin ID: %s", candidate, p.ID)
		}

		r.registry[candidate] = plugin
	}

	return nil
}

// GetPlugin finds a specific plugin the registry. Taking a capability and a type for that capability
// it will find and return a registered plugin.
// On the first call, it will initialize and start the plugin. On any consecutive calls it will return the
// existing plugin that has already been started.
func (r *ComponentVersionRepositoryRegistry) GetPlugin(ctx context.Context, typ runtime.Type) (*RepositoryPlugin, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, ok := r.registry[typ]; !ok {
		return nil, fmt.Errorf("ComponentVersionRepository plugin for typ %s not found", typ)
	}

	plugin := r.registry[typ]
	if existingPlugin, ok := r.constructedPlugins[plugin.ID]; ok {
		return existingPlugin.Plugin, nil
	}

	if err := plugin.cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start plugin: %s, %w", plugin.ID, err)
	}

	client, err := waitForPlugin(ctx, plugin.ID, plugin.config.Location, plugin.config.Type)
	if err != nil {
		return nil, fmt.Errorf("failed to wait for plugin to start: %w", err)
	}

	// create the base plugin backed by a concrete implementation of plugin interfaces.
	// TODO: Figure out the right context here. -> Should be the base context from the plugin manager.
	repoPlugin := NewRepositoryPlugin(context.Background(), r.logger, client, plugin.ID, plugin.path, plugin.config)

	r.constructedPlugins[repoPlugin.ID] = &constructedPlugin{
		Plugin: repoPlugin,
		cmd:    plugin.cmd,
	}

	return repoPlugin, nil
}

// getInternalComponentVersionRepositoryPlugin looks in the internally registered plugins first if we have any plugins that have
// been added.
func getInternalComponentVersionRepositoryPlugin(typ runtime.Type) (PluginBase, bool) {
	if _, ok := internalComponentVersionRepositoryPlugins[typ]; !ok {
		return nil, false
	}

	return internalComponentVersionRepositoryPlugins[typ], true
}

// GetReadWriteComponentVersionRepository gets a plugin that registered for this given capability.
func GetReadWriteComponentVersionRepository[T runtime.Typed](ctx context.Context, pm *PluginManager, prototype T) (ReadWriteOCMRepositoryPluginContract[T], error) {
	typ, err := internalComponentVersionRepositoryScheme.TypeForPrototype(prototype)
	if err != nil {
		return nil, fmt.Errorf("failed to get type for prototype %T: %w", prototype, err)
	}

	if v, ok := getInternalComponentVersionRepositoryPlugin(typ); ok {
		p, ok := v.(ReadWriteOCMRepositoryPluginContract[T])
		if !ok {
			return nil, fmt.Errorf("read-write component version repository does not implement ReadWriteOCMRepositoryPluginContract but was: %T", v)
		}

		return p, nil
	}

	// return nil, fmt.Errorf("no internal plugin registered for type %s", prototype.GetType())
	// TODO adjust binary based plugin to be type safe
	p, err := pm.ComponentVersionRepositoryRegistry.GetPlugin(ctx, typ)
	if err != nil {
		return nil, fmt.Errorf("error getting ComponentVersionRepository plugin for capability %s and %s with type %s: %w", ReadComponentVersionRepositoryCapability, WriteComponentVersionRepositoryCapability, typ, err)
	}

	pt := NewTypedRepositoryPlugin[T](p)

	return pt, nil
}

// NewTransferRegistry creates a new registry and initializes maps.
func NewTransferRegistry() *ComponentVersionRepositoryRegistry {
	return &ComponentVersionRepositoryRegistry{
		registry:           make(map[runtime.Type]*Plugin),
		constructedPlugins: make(map[string]*constructedPlugin),
	}
}
