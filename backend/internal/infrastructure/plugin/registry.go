// Package plugin provides plugin loading and management.
package plugin

import (
	"context"
	"fmt"
	"sync"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/infrastructure/plugin/sdk"
)

// Registry manages loaded plugins.
type Registry struct {
	plugins map[string]sdk.Plugin
	config  sdk.PluginConfig
	logger  zerolog.Logger
	mu      sync.RWMutex
}

// NewRegistry creates a new plugin registry.
func NewRegistry(config sdk.PluginConfig, logger zerolog.Logger) *Registry {
	return &Registry{
		plugins: make(map[string]sdk.Plugin),
		config:  config,
		logger:  logger.With().Str("component", "plugin_registry").Logger(),
	}
}

// Register registers a plugin.
func (r *Registry) Register(ctx context.Context, p sdk.Plugin) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	info := p.Info()

	// Check for duplicate
	if _, exists := r.plugins[info.ID]; exists {
		return fmt.Errorf("plugin already registered: %s", info.ID)
	}

	// Initialize plugin
	if err := p.Init(ctx, r.config); err != nil {
		return fmt.Errorf("failed to initialize plugin %s: %w", info.ID, err)
	}

	// Start plugin
	if err := p.Start(ctx); err != nil {
		return fmt.Errorf("failed to start plugin %s: %w", info.ID, err)
	}

	r.plugins[info.ID] = p

	r.logger.Info().
		Str("plugin_id", info.ID).
		Str("plugin_name", info.Name).
		Str("plugin_version", info.Version).
		Str("plugin_type", string(info.Type)).
		Msg("Plugin registered")

	return nil
}

// Unregister unregisters a plugin.
func (r *Registry) Unregister(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	p, exists := r.plugins[id]
	if !exists {
		return fmt.Errorf("plugin not found: %s", id)
	}

	// Stop plugin
	if err := p.Stop(ctx); err != nil {
		r.logger.Warn().
			Err(err).
			Str("plugin_id", id).
			Msg("Error stopping plugin")
	}

	delete(r.plugins, id)

	r.logger.Info().
		Str("plugin_id", id).
		Msg("Plugin unregistered")

	return nil
}

// Get retrieves a plugin by ID.
func (r *Registry) Get(id string) (sdk.Plugin, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	p, exists := r.plugins[id]
	return p, exists
}

// List returns all registered plugins.
func (r *Registry) List() []sdk.Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	plugins := make([]sdk.Plugin, 0, len(r.plugins))
	for _, p := range r.plugins {
		plugins = append(plugins, p)
	}
	return plugins
}

// ListByType returns plugins of a specific type.
func (r *Registry) ListByType(pluginType sdk.PluginType) []sdk.Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var plugins []sdk.Plugin
	for _, p := range r.plugins {
		if p.Info().Type == pluginType {
			plugins = append(plugins, p)
		}
	}
	return plugins
}

// GetIndexers returns all indexer plugins.
func (r *Registry) GetIndexers() []sdk.IndexerPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var indexers []sdk.IndexerPlugin
	for _, p := range r.plugins {
		if indexer, ok := p.(sdk.IndexerPlugin); ok {
			indexers = append(indexers, indexer)
		}
	}
	return indexers
}

// GetProcessors returns all processor plugins.
func (r *Registry) GetProcessors() []sdk.ProcessorPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var processors []sdk.ProcessorPlugin
	for _, p := range r.plugins {
		if processor, ok := p.(sdk.ProcessorPlugin); ok {
			processors = append(processors, processor)
		}
	}
	return processors
}

// GetHooks returns all hook plugins.
func (r *Registry) GetHooks() []sdk.HookPlugin {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var hooks []sdk.HookPlugin
	for _, p := range r.plugins {
		if hook, ok := p.(sdk.HookPlugin); ok {
			hooks = append(hooks, hook)
		}
	}
	return hooks
}

// Load loads a plugin from a path.
// Note: Go plugin loading requires CGO, so this is a placeholder
// for dynamic loading. In practice, plugins would be compiled in
// or loaded via other mechanisms.
func (r *Registry) Load(ctx context.Context, path string) error {
	r.logger.Warn().
		Str("path", path).
		Msg("Dynamic plugin loading not implemented (requires CGO)")
	return fmt.Errorf("dynamic plugin loading not supported")
}

// Unload unloads a plugin.
func (r *Registry) Unload(ctx context.Context, id string) error {
	return r.Unregister(ctx, id)
}

// StopAll stops all plugins.
func (r *Registry) StopAll(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, p := range r.plugins {
		if err := p.Stop(ctx); err != nil {
			r.logger.Warn().
				Err(err).
				Str("plugin_id", id).
				Msg("Error stopping plugin")
		}
	}

	r.plugins = make(map[string]sdk.Plugin)
	r.logger.Info().Msg("All plugins stopped")
}

// HealthCheck checks the health of all plugins.
func (r *Registry) HealthCheck(ctx context.Context) map[string]sdk.HealthStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	statuses := make(map[string]sdk.HealthStatus)
	for id, p := range r.plugins {
		statuses[id] = p.Health(ctx)
	}
	return statuses
}
