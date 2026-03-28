// Package handlers provides gRPC service implementations.
package handlers

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/rs/zerolog"

	"github.com/dacrypt/cortex/backend/internal/application/metrics"
	"github.com/dacrypt/cortex/backend/internal/application/pipeline"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/repository"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/config"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/persistence/sqlite"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/plugin"
	"github.com/dacrypt/cortex/backend/internal/interfaces/event"
)

// AdminHandler handles admin-related gRPC requests.
type AdminHandler struct {
	workspaceRepo     repository.WorkspaceRepository
	fileRepo          repository.FileRepository
	taskRepo          repository.TaskRepository
	pluginReg         *plugin.Registry
	config            *config.Config
	configPath        string
	configVersionRepo *sqlite.ConfigVersionRepository
	dashboardService  *metrics.Service
	startTime         time.Time
	version           string
	logger            zerolog.Logger
	shutdownFunc      func()
	subscriber        *event.Subscriber
	progressTracker   *pipeline.ProgressTracker
}

// AdminHandlerConfig holds configuration for the admin handler.
type AdminHandlerConfig struct {
	WorkspaceRepo     repository.WorkspaceRepository
	FileRepo          repository.FileRepository
	TaskRepo          repository.TaskRepository
	PluginReg         *plugin.Registry
	Config            *config.Config
	ConfigPath        string
	ConfigVersionRepo *sqlite.ConfigVersionRepository
	DashboardService  *metrics.Service
	Version           string
	Logger            zerolog.Logger
	ShutdownFunc      func()
	Subscriber        *event.Subscriber
	ProgressTracker   *pipeline.ProgressTracker
}

// NewAdminHandler creates a new admin handler.
func NewAdminHandler(cfg AdminHandlerConfig) *AdminHandler {
	handler := &AdminHandler{
		workspaceRepo:     cfg.WorkspaceRepo,
		fileRepo:          cfg.FileRepo,
		taskRepo:          cfg.TaskRepo,
		pluginReg:         cfg.PluginReg,
		config:            cfg.Config,
		configPath:        cfg.ConfigPath,
		configVersionRepo: cfg.ConfigVersionRepo,
		dashboardService:  cfg.DashboardService,
		startTime:         time.Now(),
		version:           cfg.Version,
		logger:            cfg.Logger.With().Str("handler", "admin").Logger(),
		shutdownFunc:      cfg.ShutdownFunc,
		subscriber:        cfg.Subscriber,
		progressTracker:   cfg.ProgressTracker,
	}

	// Set up progress tracker to listen to events if subscriber is available
	if cfg.ProgressTracker != nil && cfg.Subscriber != nil {
		sub := cfg.Subscriber.SubscribeToPipeline(context.Background())
		go func() {
			for evt := range sub.Channel {
				cfg.ProgressTracker.OnEvent(context.Background(), &evt)
			}
		}()
	}

	return handler
}

// GetConfig returns the loaded daemon configuration.
func (h *AdminHandler) GetConfig() (*config.Config, string) {
	return h.config, h.configPath
}

// UpdateConfig updates the daemon configuration.
func (h *AdminHandler) UpdateConfig(ctx context.Context, cfg *config.Config, persist bool) (*config.Config, error) {
	// Validate the new configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Update in-memory configuration
	h.config = cfg

	// Create version snapshot if repository is available
	if h.configVersionRepo != nil {
		_, err := h.configVersionRepo.Create(ctx, cfg, "admin", "Configuration updated via admin API", map[string]string{
			"change_type": "config_update",
		})
		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to create config version snapshot")
		}
	}

	// TODO: Persist to file if persist is true and configPath is set
	// This would require implementing config file writing logic
	if persist && h.configPath != "" {
		h.logger.Info().
			Str("config_path", h.configPath).
			Msg("Config update requested (file persistence not yet implemented)")
	}

	h.logger.Info().Msg("Configuration updated")
	return h.config, nil
}

// SubscribePipeline returns a subscription for pipeline events.
func (h *AdminHandler) SubscribePipeline(ctx context.Context) *event.Subscription {
	if h.subscriber == nil {
		return nil
	}
	return h.subscriber.SubscribeToPipeline(ctx)
}

// GetPipelineProgress returns the current pipeline progress snapshot.
func (h *AdminHandler) GetPipelineProgress() *pipeline.ProgressSnapshot {
	if h.progressTracker == nil {
		return nil
	}
	allProgress := h.progressTracker.GetAllProgress()
	stages := h.progressTracker.GetStages()
	stats := h.progressTracker.GetStats()

	return &pipeline.ProgressSnapshot{
		Files:  allProgress,
		Stages: stages,
		Stats:  stats,
	}
}

// GetStatus returns daemon status.
func (h *AdminHandler) GetStatus(ctx context.Context) (*StatusResponse, error) {
	// Get workspace count
	workspaceCount, err := h.workspaceRepo.Count(ctx)
	if err != nil {
		return nil, err
	}

	workspaces := []*entity.Workspace{}
	if workspaceCount > 0 {
		opts := repository.DefaultWorkspaceListOptions()
		opts.Limit = workspaceCount
		workspaces, err = h.workspaceRepo.List(ctx, opts)
		if err != nil {
			return nil, err
		}
	}

	fileCount, err := h.countFiles(ctx, workspaces)
	if err != nil {
		return nil, err
	}

	// Get task stats
	taskStats, err := h.taskRepo.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	// Get plugin count
	pluginCount := 0
	if h.pluginReg != nil {
		pluginCount = len(h.pluginReg.List())
	}

	// Build embedding status from config
	var embeddingStatus *EmbeddingStatus
	if h.config != nil {
		embeddingStatus = &EmbeddingStatus{
			Enabled:  h.config.LLM.Embeddings.Enabled,
			Endpoint: h.config.LLM.Embeddings.Endpoint,
			Model:    h.config.LLM.Embeddings.Model,
		}
	}

	return &StatusResponse{
		Version:         h.version,
		Uptime:          time.Since(h.startTime),
		Workspaces:      workspaceCount,
		Files:           fileCount,
		Tasks:           *taskStats,
		Plugins:         pluginCount,
		GoVersion:       runtime.Version(),
		GOOS:            runtime.GOOS,
		GOARCH:          runtime.GOARCH,
		EmbeddingStatus: embeddingStatus,
	}, nil
}

// Shutdown initiates a graceful shutdown.
func (h *AdminHandler) Shutdown() {
	h.logger.Info().Msg("Shutdown requested via admin handler")
	if h.shutdownFunc != nil {
		go h.shutdownFunc()
	}
}

// Reload reloads configuration.
func (h *AdminHandler) Reload(ctx context.Context) error {
	h.logger.Info().Msg("Reload requested")
	// TODO: Implement config reload
	return nil
}

// RegisterWorkspace registers a new workspace.
func (h *AdminHandler) RegisterWorkspace(ctx context.Context, path, name string) (*entity.Workspace, error) {
	// Check if already registered
	existing, _ := h.workspaceRepo.GetByPath(ctx, path)
	if existing != nil {
		h.logger.Debug().
			Str("path", path).
			Msg("Workspace already registered")
		return existing, nil
	}

	ws := entity.NewWorkspace(path, name)

	if err := h.workspaceRepo.Create(ctx, ws); err != nil {
		return nil, err
	}

	h.logger.Info().
		Str("workspace_id", ws.ID.String()).
		Str("path", path).
		Msg("Workspace registered")

	return ws, nil
}

// UnregisterWorkspace unregisters a workspace.
func (h *AdminHandler) UnregisterWorkspace(ctx context.Context, id string) error {
	if err := h.workspaceRepo.Delete(ctx, entity.WorkspaceID(id)); err != nil {
		return err
	}

	h.logger.Info().
		Str("workspace_id", id).
		Msg("Workspace unregistered")

	return nil
}

// ListWorkspaces lists all workspaces.
func (h *AdminHandler) ListWorkspaces(ctx context.Context) ([]*entity.Workspace, error) {
	opts := repository.DefaultWorkspaceListOptions()
	return h.workspaceRepo.List(ctx, opts)
}

// GetWorkspace retrieves a workspace by ID.
func (h *AdminHandler) GetWorkspace(ctx context.Context, id string) (*entity.Workspace, error) {
	return h.workspaceRepo.Get(ctx, entity.WorkspaceID(id))
}

// SetWorkspaceActive sets the active status of a workspace.
func (h *AdminHandler) SetWorkspaceActive(ctx context.Context, id string, active bool) error {
	return h.workspaceRepo.SetActive(ctx, entity.WorkspaceID(id), active)
}

// ListPlugins lists all loaded plugins.
func (h *AdminHandler) ListPlugins() []PluginInfo {
	if h.pluginReg == nil {
		return nil
	}

	plugins := h.pluginReg.List()
	infos := make([]PluginInfo, 0, len(plugins))

	for _, p := range plugins {
		info := p.Info()
		infos = append(infos, PluginInfo{
			ID:           info.ID,
			Name:         info.Name,
			Version:      info.Version,
			Type:         string(info.Type),
			Author:       info.Author,
			Description:  info.Description,
			Capabilities: info.Capabilities,
		})
	}

	return infos
}

// LoadPlugin loads a plugin from path.
func (h *AdminHandler) LoadPlugin(ctx context.Context, path string) error {
	if h.pluginReg == nil {
		return nil
	}
	return h.pluginReg.Load(ctx, path)
}

// UnloadPlugin unloads a plugin.
func (h *AdminHandler) UnloadPlugin(ctx context.Context, id string) error {
	if h.pluginReg == nil {
		return nil
	}
	return h.pluginReg.Unload(ctx, id)
}

// HealthCheck performs a health check.
func (h *AdminHandler) HealthCheck(ctx context.Context) (*HealthResponse, error) {
	checks := make(map[string]CheckResult)

	// Database check
	opts := repository.DefaultWorkspaceListOptions()
	if _, err := h.workspaceRepo.List(ctx, opts); err != nil {
		checks["database"] = CheckResult{
			Status:  "unhealthy",
			Message: err.Error(),
		}
	} else {
		checks["database"] = CheckResult{
			Status: "healthy",
		}
	}

	// Plugin check
	if h.pluginReg != nil {
		plugins := h.pluginReg.List()
		healthyPlugins := 0
		for _, p := range plugins {
			status := p.Health(ctx)
			if status.Healthy {
				healthyPlugins++
			}
		}
		checks["plugins"] = CheckResult{
			Status:  "healthy",
			Message: "",
		}
	}

	// Determine overall status
	overallStatus := "healthy"
	for _, check := range checks {
		if check.Status == "unhealthy" {
			overallStatus = "unhealthy"
			break
		}
		if check.Status == "degraded" && overallStatus == "healthy" {
			overallStatus = "degraded"
		}
	}

	return &HealthResponse{
		Status:    overallStatus,
		Checks:    checks,
		Timestamp: time.Now(),
	}, nil
}

// GetMetrics returns daemon metrics.
func (h *AdminHandler) GetMetrics(ctx context.Context) (*MetricsResponse, error) {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	taskStats, _ := h.taskRepo.GetStats(ctx)
	workspaces := []*entity.Workspace{}
	if workspaceCount, err := h.workspaceRepo.Count(ctx); err == nil && workspaceCount > 0 {
		opts := repository.DefaultWorkspaceListOptions()
		opts.Limit = workspaceCount
		workspaces, _ = h.workspaceRepo.List(ctx, opts)
	}
	fileCount, _ := h.countFiles(ctx, workspaces)

	return &MetricsResponse{
		Uptime:         time.Since(h.startTime),
		Goroutines:     runtime.NumGoroutine(),
		HeapAlloc:      m.HeapAlloc,
		HeapSys:        m.HeapSys,
		HeapObjects:    m.HeapObjects,
		GCCycles:       m.NumGC,
		FilesIndexed:   fileCount,
		TasksProcessed: taskStats.Completed,
		TasksPending:   taskStats.Pending,
		TasksFailed:    taskStats.Failed,
	}, nil
}

// EmbeddingStatus contains embedding service configuration.
type EmbeddingStatus struct {
	Enabled  bool
	Endpoint string
	Model    string
}

// StatusResponse contains daemon status information.
type StatusResponse struct {
	Version         string
	Uptime          time.Duration
	Workspaces      int
	Files           int
	Tasks           entity.TaskStats
	Plugins         int
	GoVersion       string
	GOOS            string
	GOARCH          string
	EmbeddingStatus *EmbeddingStatus
}

// HealthResponse contains health check results.
type HealthResponse struct {
	Status    string
	Checks    map[string]CheckResult
	Timestamp time.Time
}

// CheckResult contains a single check result.
type CheckResult struct {
	Status  string
	Message string
}

// MetricsResponse contains daemon metrics.
type MetricsResponse struct {
	Uptime         time.Duration
	Goroutines     int
	HeapAlloc      uint64
	HeapSys        uint64
	HeapObjects    uint64
	GCCycles       uint32
	FilesIndexed   int
	TasksProcessed int
	TasksPending   int
	TasksFailed    int
}

// PluginInfo contains plugin information.
type PluginInfo struct {
	ID           string
	Name         string
	Version      string
	Type         string
	Author       string
	Description  string
	Capabilities []string
}

func (h *AdminHandler) countFiles(ctx context.Context, workspaces []*entity.Workspace) (int, error) {
	total := 0
	for _, ws := range workspaces {
		count, err := h.fileRepo.Count(ctx, ws.ID)
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

// ConfigVersion represents a versioned configuration snapshot.
type ConfigVersion struct {
	VersionID   string
	CreatedAt   time.Time
	CreatedBy   string
	Description string
	Config      *config.Config
	Metadata    map[string]string
}

// GetConfigVersions returns configuration versions with pagination.
func (h *AdminHandler) GetConfigVersions(ctx context.Context, limit, offset int) ([]*ConfigVersion, int, error) {
	if h.configVersionRepo == nil {
		return []*ConfigVersion{}, 0, fmt.Errorf("config version repository not available")
	}

	versions, total, err := h.configVersionRepo.List(ctx, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list config versions: %w", err)
	}

	result := make([]*ConfigVersion, 0, len(versions))
	for _, v := range versions {
		result = append(result, &ConfigVersion{
			VersionID:   v.VersionID,
			CreatedAt:   v.CreatedAt,
			CreatedBy:   v.CreatedBy,
			Description: v.Description,
			Config:      v.Config,
			Metadata:    v.Metadata,
		})
	}

	return result, total, nil
}

// GetConfigVersion retrieves a configuration version by ID.
func (h *AdminHandler) GetConfigVersion(ctx context.Context, versionID string) (*ConfigVersion, error) {
	if h.configVersionRepo == nil {
		return nil, fmt.Errorf("config version repository not available")
	}

	version, err := h.configVersionRepo.Get(ctx, versionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get config version: %w", err)
	}

	return &ConfigVersion{
		VersionID:   version.VersionID,
		CreatedAt:   version.CreatedAt,
		CreatedBy:   version.CreatedBy,
		Description: version.Description,
		Config:      version.Config,
		Metadata:    version.Metadata,
	}, nil
}

// RestoreConfigVersion restores a configuration version.
func (h *AdminHandler) RestoreConfigVersion(ctx context.Context, versionID string, createBackup bool, description string) (*config.Config, error) {
	if h.configVersionRepo == nil {
		return nil, fmt.Errorf("config version repository not available")
	}

	// Get the version to restore
	version, err := h.configVersionRepo.Get(ctx, versionID)
	if err != nil {
		return nil, fmt.Errorf("failed to get config version: %w", err)
	}

	// Create backup if requested
	if createBackup {
		backupDesc := fmt.Sprintf("Backup before restoring version %s", versionID)
		if description != "" {
			backupDesc = fmt.Sprintf("%s: %s", backupDesc, description)
		}
		_, err := h.configVersionRepo.Create(ctx, h.config, "system", backupDesc, map[string]string{
			"restore_backup": "true",
			"restored_from":  versionID,
		})
		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to create backup before restore")
		}
	}

	// Restore the configuration
	h.config = version.Config

	// Save to file if config path is set
	if h.configPath != "" {
		// TODO: Implement config file save
		h.logger.Info().
			Str("version_id", versionID).
			Str("config_path", h.configPath).
			Msg("Configuration restored (file save not yet implemented)")
	}

	h.logger.Info().
		Str("version_id", versionID).
		Str("description", version.Description).
		Msg("Configuration version restored")

	return h.config, nil
}

// UpdatePrompts updates prompt templates.
func (h *AdminHandler) UpdatePrompts(ctx context.Context, prompts config.PromptsConfig, createVersion bool, description string) (config.PromptsConfig, error) {
	// Update prompts in current config
	h.config.LLM.Prompts = prompts

	// Create version if requested
	if createVersion && h.configVersionRepo != nil {
		versionDesc := description
		if versionDesc == "" {
			versionDesc = "Updated prompts via admin UI"
		}
		_, err := h.configVersionRepo.Create(ctx, h.config, "admin", versionDesc, map[string]string{
			"change_type": "prompts_update",
		})
		if err != nil {
			h.logger.Warn().Err(err).Msg("Failed to create config version")
		} else {
			h.logger.Info().
				Str("description", versionDesc).
				Msg("Configuration version created for prompts update")
		}
	}

	// Save to file if config path is set
	if h.configPath != "" {
		// TODO: Implement config file save
		h.logger.Info().
			Str("config_path", h.configPath).
			Msg("Prompts updated (file save not yet implemented)")
	}

	return prompts, nil
}

// ErrDashboardServiceNotAvailable is returned when the dashboard service is not configured.
var ErrDashboardServiceNotAvailable = fmt.Errorf("dashboard service not available")

// GetDashboardMetrics returns AI metadata quality dashboard metrics.
func (h *AdminHandler) GetDashboardMetrics(ctx context.Context, workspaceID string, periodHours int64) (*metrics.DashboardMetrics, error) {
	if h.dashboardService == nil {
		return nil, ErrDashboardServiceNotAvailable
	}
	period := time.Duration(periodHours) * time.Hour
	return h.dashboardService.GetDashboardMetrics(ctx, entity.WorkspaceID(workspaceID), period)
}

// GetConfidenceDistribution returns confidence score distribution by category.
func (h *AdminHandler) GetConfidenceDistribution(ctx context.Context, workspaceID string) (map[string]*metrics.ConfidenceDistribution, error) {
	if h.dashboardService == nil {
		return nil, ErrDashboardServiceNotAvailable
	}
	return h.dashboardService.GetConfidenceDistribution(ctx, entity.WorkspaceID(workspaceID))
}

// GetModelDriftReport returns model drift detection report.
func (h *AdminHandler) GetModelDriftReport(ctx context.Context, workspaceID string) (*metrics.DriftReport, error) {
	if h.dashboardService == nil {
		return nil, ErrDashboardServiceNotAvailable
	}
	return h.dashboardService.DetectModelDrift(ctx, entity.WorkspaceID(workspaceID))
}
