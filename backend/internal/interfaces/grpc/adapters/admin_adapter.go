// Package adapters provides gRPC service adapters that bridge between
// protobuf definitions and internal handlers.
package adapters

import (
	"context"
	"os"
	"runtime"
	"strings"
	"time"

	cortexv1 "github.com/dacrypt/cortex/backend/api/gen/cortex/v1"
	"github.com/dacrypt/cortex/backend/internal/application/metrics"
	"github.com/dacrypt/cortex/backend/internal/domain/event"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/config"
	"github.com/dacrypt/cortex/backend/internal/interfaces/grpc/handlers"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// AdminServiceAdapter implements cortexv1.AdminServiceServer.
type AdminServiceAdapter struct {
	cortexv1.UnimplementedAdminServiceServer
	handler *handlers.AdminHandler
}

// NewAdminServiceAdapter creates a new admin service adapter.
func NewAdminServiceAdapter(handler *handlers.AdminHandler) *AdminServiceAdapter {
	return &AdminServiceAdapter{handler: handler}
}

// GetStatus returns daemon status.
func (a *AdminServiceAdapter) GetStatus(ctx context.Context, req *cortexv1.GetStatusRequest) (*cortexv1.DaemonStatus, error) {
	status, err := a.handler.GetStatus(ctx)
	if err != nil {
		return nil, err
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	// Build embedding status if available
	var embeddingStatus *cortexv1.EmbeddingStatus
	if status.EmbeddingStatus != nil {
		embeddingStatus = &cortexv1.EmbeddingStatus{
			Enabled:  status.EmbeddingStatus.Enabled,
			Endpoint: status.EmbeddingStatus.Endpoint,
			Model:    status.EmbeddingStatus.Model,
		}
	}

	return &cortexv1.DaemonStatus{
		Version:        status.Version,
		UptimeSeconds:  int64(status.Uptime.Seconds()),
		WorkspaceCount: int32(status.Workspaces),
		IndexedFiles:   int32(status.Files),
		Resources: &cortexv1.ResourceUsage{
			MemoryBytes:    int64(m.HeapAlloc),
			MemoryMaxBytes: int64(m.HeapSys),
			GoroutineCount: int32(runtime.NumGoroutine()),
		},
		QueueStats: &cortexv1.QueueStats{
			Pending:   int32(status.Tasks.Pending),
			Running:   int32(status.Tasks.Running),
			Completed: int32(status.Tasks.Completed),
			Failed:    int32(status.Tasks.Failed),
		},
		EmbeddingStatus: embeddingStatus,
	}, nil
}

// Shutdown initiates graceful shutdown.
func (a *AdminServiceAdapter) Shutdown(ctx context.Context, req *cortexv1.ShutdownRequest) (*cortexv1.ShutdownResult, error) {
	a.handler.Shutdown()
	msg := "Shutdown initiated"
	return &cortexv1.ShutdownResult{
		Success: true,
		Message: &msg,
	}, nil
}

// Reload reloads configuration.
func (a *AdminServiceAdapter) Reload(ctx context.Context, req *cortexv1.ReloadRequest) (*cortexv1.ReloadResult, error) {
	err := a.handler.Reload(ctx)
	if err != nil {
		return &cortexv1.ReloadResult{
			Success: false,
			Errors:  []string{err.Error()},
		}, nil
	}
	return &cortexv1.ReloadResult{
		Success:  true,
		Reloaded: []string{"config"},
	}, nil
}

// GetConfig returns current configuration.
func (a *AdminServiceAdapter) GetConfig(ctx context.Context, req *cortexv1.GetConfigRequest) (*cortexv1.Config, error) {
	cfg, path := a.handler.GetConfig()
	return configToProto(cfg, path), nil
}

// UpdateConfig updates configuration.
func (a *AdminServiceAdapter) UpdateConfig(ctx context.Context, req *cortexv1.UpdateConfigRequest) (*cortexv1.Config, error) {
	if req.Config == nil {
		cfg, path := a.handler.GetConfig()
		return configToProto(cfg, path), nil
	}

	// Convert proto config to internal config
	cfg := configFromProto(req.Config)

	// Update configuration via handler
	updatedCfg, err := a.handler.UpdateConfig(ctx, cfg, req.GetPersist())
	if err != nil {
		return nil, err
	}

	_, path := a.handler.GetConfig()
	return configToProto(updatedCfg, path), nil
}

// GetEnv returns the daemon environment variables.
func (a *AdminServiceAdapter) GetEnv(ctx context.Context, req *cortexv1.GetEnvRequest) (*cortexv1.EnvVars, error) {
	vars := make(map[string]string)
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		vars[key] = value
	}
	return &cortexv1.EnvVars{Vars: vars}, nil
}

// GetLogs returns the tail of the configured log file.
func (a *AdminServiceAdapter) GetLogs(ctx context.Context, req *cortexv1.GetLogsRequest) (*cortexv1.LogLines, error) {
	cfg, _ := a.handler.GetConfig()
	if cfg == nil || cfg.LogFile == "" {
		return &cortexv1.LogLines{
			Lines:  []string{"log_file is not configured"},
			Source: "not_configured",
		}, nil
	}

	lines, err := tailFile(cfg.LogFile, int(req.GetTailLines()))
	if err != nil {
		return &cortexv1.LogLines{
			Lines:  []string{err.Error()},
			Source: cfg.LogFile,
		}, nil
	}

	return &cortexv1.LogLines{
		Lines:  lines,
		Source: cfg.LogFile,
	}, nil
}

// StreamPipeline streams pipeline events to the client.
func (a *AdminServiceAdapter) StreamPipeline(req *cortexv1.StreamPipelineRequest, stream cortexv1.AdminService_StreamPipelineServer) error {
	sub := a.handler.SubscribePipeline(stream.Context())
	if sub == nil {
		return status.Error(codes.Unavailable, "pipeline events not available")
	}

	for {
		select {
		case evt, ok := <-sub.Channel:
			if !ok {
				// Subscription channel closed - normal termination
				return nil
			}
			if err := a.handlePipelineEvent(req, evt, stream); err != nil {
				return err
			}
		case <-stream.Context().Done():
			// Client closed connection - return context error (will be logged as debug)
			return stream.Context().Err()
		}
	}
}

// handlePipelineEvent processes a single pipeline event and sends it to the stream.
func (a *AdminServiceAdapter) handlePipelineEvent(
	req *cortexv1.StreamPipelineRequest,
	evt event.Event,
	stream cortexv1.AdminService_StreamPipelineServer,
) error {
	// Extract payload
	payload, ok := evt.Data.(event.PipelineEventData)
	if !ok {
		if ptr, ok := evt.Data.(*event.PipelineEventData); ok && ptr != nil {
			payload = *ptr
		} else {
			return nil // Skip events we can't process
		}
	}

	// Filter by workspace if requested
	if req != nil && req.GetWorkspaceId() != "" && evt.WorkspaceID != nil {
		if req.GetWorkspaceId() != evt.WorkspaceID.String() {
			return nil // Skip events for other workspaces
		}
	}

	// Build message
	msg := a.buildPipelineEventMessage(evt, payload)

	// Check if stream context is done before sending
	select {
	case <-stream.Context().Done():
		return stream.Context().Err()
	default:
		if err := stream.Send(msg); err != nil {
			return err
		}
	}
	return nil
}

// buildPipelineEventMessage converts an event to a proto message.
func (a *AdminServiceAdapter) buildPipelineEventMessage(evt event.Event, payload event.PipelineEventData) *cortexv1.PipelineEvent {
	msg := &cortexv1.PipelineEvent{
		Id:            evt.ID,
		Type:          string(evt.Type),
		FilePath:      payload.FilePath,
		Stage:         payload.Stage,
		TimestampUnix: evt.Timestamp.Unix(),
	}
	if payload.Error != nil {
		msg.Error = payload.Error
	}
	if evt.WorkspaceID != nil {
		workspaceID := evt.WorkspaceID.String()
		msg.WorkspaceId = &workspaceID
	}
	return msg
}

func configToProto(cfg *config.Config, configPath string) *cortexv1.Config {
	if cfg == nil {
		return &cortexv1.Config{}
	}
	providers := make([]*cortexv1.ProviderConfig, 0, len(cfg.LLM.Providers))
	for _, provider := range cfg.LLM.Providers {
		providers = append(providers, &cortexv1.ProviderConfig{
			Id:       provider.ID,
			Type:     provider.Type,
			Endpoint: provider.Endpoint,
			ApiKey:   stringPtrOrNil(provider.APIKey),
			Options:  provider.Options,
		})
	}

	return &cortexv1.Config{
		GrpcAddress:        cfg.GRPCAddress,
		HttpAddress:        stringPtrOrNil(cfg.HTTPAddress),
		DataDir:            cfg.DataDir,
		PluginDir:          cfg.PluginDir,
		WorkerCount:        int32(cfg.WorkerCount),
		MaxConcurrentTasks: int32(cfg.MaxConcurrentTasks),
		LogLevel:           cfg.LogLevel,
		LogFile:            stringPtrOrNil(cfg.LogFile),
		ConfigPath:         stringPtrOrNil(configPath),
		Mirror: &cortexv1.MirrorConfig{
			MaxFileSizeMb: cfg.Mirror.MaxFileSizeMB,
		},
		Llm: &cortexv1.LLMConfig{
			Enabled:          cfg.LLM.Enabled,
			DefaultProvider:  cfg.LLM.DefaultProvider,
			DefaultModel:     cfg.LLM.DefaultModel,
			MaxContextTokens: int32(cfg.LLM.MaxContextTokens),
			RequestTimeoutMs: int32(cfg.LLM.RequestTimeoutMs),
			Providers:        providers,
			AutoSummary: &cortexv1.AutoSummaryConfig{
				Enabled:     cfg.LLM.AutoSummary.Enabled,
				MaxFileSize: cfg.LLM.AutoSummary.MaxFileSize,
			},
			AutoIndex: &cortexv1.AutoIndexConfig{
				Enabled:              cfg.LLM.AutoIndex.Enabled,
				ApplyTags:            cfg.LLM.AutoIndex.ApplyTags,
				ApplyProjects:        cfg.LLM.AutoIndex.ApplyProjects,
				UseSuggestedContexts: cfg.LLM.AutoIndex.UseSuggestedContexts,
				MaxFileSize:          cfg.LLM.AutoIndex.MaxFileSize,
				MaxTags:              int32(cfg.LLM.AutoIndex.MaxTags),
				EnableCategories:     cfg.LLM.AutoIndex.EnableCategories,
				EnableRelated:        cfg.LLM.AutoIndex.EnableRelated,
				MaxRelatedResults:    int32(cfg.LLM.AutoIndex.MaxRelatedResults),
				RelatedCandidates:    int32(cfg.LLM.AutoIndex.RelatedCandidates),
			},
			Prompts: promptsToProto(cfg.LLM.Prompts),
		},
	}
}

func promptsToProto(prompts config.PromptsConfig) *cortexv1.PromptsConfig {
	// Use defaults if prompts are empty
	defaults := config.DefaultPromptsConfig()

	return &cortexv1.PromptsConfig{
		SuggestTags:                getPromptOrDefault(prompts.SuggestTags, defaults.SuggestTags),
		SuggestProject:             getPromptOrDefault(prompts.SuggestProject, defaults.SuggestProject),
		GenerateSummary:            getPromptOrDefault(prompts.GenerateSummary, defaults.GenerateSummary),
		ExtractKeyTerms:            getPromptOrDefault(prompts.ExtractKeyTerms, defaults.ExtractKeyTerms),
		RagAnswer:                  getPromptOrDefault(prompts.RAGAnswer, defaults.RAGAnswer),
		ClassifyCategory:           getPromptOrDefault(prompts.ClassifyCategory, defaults.ClassifyCategory),
		SuggestProjectNature:       getPromptOrDefault(prompts.SuggestProjectNature, defaults.SuggestProjectNature),
		GenerateProjectDescription: getPromptOrDefault(prompts.GenerateProjectDescription, defaults.GenerateProjectDescription),
		ValidateProjectName:        getPromptOrDefault(prompts.ValidateProjectName, defaults.ValidateProjectName),
	}
}

func getPromptOrDefault(value, defaultValue string) string {
	if value == "" {
		return defaultValue
	}
	return value
}

func promptsFromProto(prompts *cortexv1.PromptsConfig) config.PromptsConfig {
	if prompts == nil {
		return config.DefaultPromptsConfig()
	}
	return config.PromptsConfig{
		SuggestTags:                prompts.SuggestTags,
		SuggestProject:             prompts.SuggestProject,
		GenerateSummary:            prompts.GenerateSummary,
		ExtractKeyTerms:            prompts.ExtractKeyTerms,
		RAGAnswer:                  prompts.RagAnswer,
		ClassifyCategory:           prompts.ClassifyCategory,
		SuggestProjectNature:       prompts.SuggestProjectNature,
		GenerateProjectDescription: prompts.GenerateProjectDescription,
		ValidateProjectName:        prompts.ValidateProjectName,
	}
}

// configFromProto converts a proto Config to internal config.Config.
func configFromProto(protoCfg *cortexv1.Config) *config.Config {
	if protoCfg == nil {
		return config.DefaultConfig()
	}

	providers := make([]config.ProviderConfig, 0, len(protoCfg.GetLlm().GetProviders()))
	for _, p := range protoCfg.GetLlm().GetProviders() {
		providers = append(providers, config.ProviderConfig{
			ID:       p.GetId(),
			Type:     p.GetType(),
			Endpoint: p.GetEndpoint(),
			APIKey:   p.GetApiKey(),
			Options:  p.GetOptions(),
		})
	}

	var llmCfg config.LLMConfig
	if protoCfg.GetLlm() != nil {
		llm := protoCfg.GetLlm()
		llmCfg = config.LLMConfig{
			Enabled:          llm.GetEnabled(),
			DefaultProvider:  llm.GetDefaultProvider(),
			DefaultModel:     llm.GetDefaultModel(),
			MaxContextTokens: int(llm.GetMaxContextTokens()),
			RequestTimeoutMs: int(llm.GetRequestTimeoutMs()),
			Providers:        providers,
			Prompts:          promptsFromProto(llm.GetPrompts()),
		}
		if llm.GetAutoSummary() != nil {
			llmCfg.AutoSummary = config.AutoSummaryConfig{
				Enabled:     llm.GetAutoSummary().GetEnabled(),
				MaxFileSize: llm.GetAutoSummary().GetMaxFileSize(),
			}
		}
		if llm.GetAutoIndex() != nil {
			ai := llm.GetAutoIndex()
			llmCfg.AutoIndex = config.AutoIndexConfig{
				Enabled:              ai.GetEnabled(),
				ApplyTags:            ai.GetApplyTags(),
				ApplyProjects:        ai.GetApplyProjects(),
				UseSuggestedContexts: ai.GetUseSuggestedContexts(),
				MaxFileSize:          ai.GetMaxFileSize(),
				MaxTags:              int(ai.GetMaxTags()),
				EnableCategories:     ai.GetEnableCategories(),
				EnableRelated:        ai.GetEnableRelated(),
				MaxRelatedResults:    int(ai.GetMaxRelatedResults()),
				RelatedCandidates:    int(ai.GetRelatedCandidates()),
			}
		}
		// Note: Embeddings config is not in proto yet, using defaults
		llmCfg.Embeddings = config.DefaultConfig().LLM.Embeddings
	}

	var mirrorCfg config.MirrorConfig
	if protoCfg.GetMirror() != nil {
		mirrorCfg = config.MirrorConfig{
			MaxFileSizeMB: protoCfg.GetMirror().GetMaxFileSizeMb(),
		}
	}

	return &config.Config{
		GRPCAddress:        protoCfg.GetGrpcAddress(),
		HTTPAddress:        protoCfg.GetHttpAddress(),
		DataDir:            protoCfg.GetDataDir(),
		PluginDir:          protoCfg.GetPluginDir(),
		WorkerCount:        int(protoCfg.GetWorkerCount()),
		MaxConcurrentTasks: int(protoCfg.GetMaxConcurrentTasks()),
		LogLevel:           protoCfg.GetLogLevel(),
		LogFile:            protoCfg.GetLogFile(),
		LLM:                llmCfg,
		Mirror:             mirrorCfg,
	}
}

func stringPtrOrNil(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func tailFile(path string, maxLines int) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	raw := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if maxLines <= 0 || len(raw) <= maxLines {
		return raw, nil
	}
	return raw[len(raw)-maxLines:], nil
}

// RegisterWorkspace registers a new workspace.
func (a *AdminServiceAdapter) RegisterWorkspace(ctx context.Context, req *cortexv1.RegisterWorkspaceRequest) (*cortexv1.Workspace, error) {
	ws, err := a.handler.RegisterWorkspace(ctx, req.Path, req.GetName())
	if err != nil {
		return nil, err
	}

	return workspaceToProto(ws), nil
}

// UnregisterWorkspace unregisters a workspace.
func (a *AdminServiceAdapter) UnregisterWorkspace(ctx context.Context, req *cortexv1.UnregisterWorkspaceRequest) (*cortexv1.UnregisterResult, error) {
	err := a.handler.UnregisterWorkspace(ctx, req.WorkspaceId)
	if err != nil {
		msg := err.Error()
		return &cortexv1.UnregisterResult{
			Success: false,
			Message: &msg,
		}, nil
	}
	msg := "Workspace unregistered"
	return &cortexv1.UnregisterResult{
		Success: true,
		Message: &msg,
	}, nil
}

// ListWorkspaces lists all workspaces.
func (a *AdminServiceAdapter) ListWorkspaces(req *cortexv1.ListWorkspacesRequest, stream cortexv1.AdminService_ListWorkspacesServer) error {
	workspaces, err := a.handler.ListWorkspaces(stream.Context())
	if err != nil {
		return err
	}

	for _, ws := range workspaces {
		if err := stream.Send(workspaceToProto(ws)); err != nil {
			return err
		}
	}

	return nil
}

// GetWorkspace retrieves a workspace.
func (a *AdminServiceAdapter) GetWorkspace(ctx context.Context, req *cortexv1.GetWorkspaceRequest) (*cortexv1.Workspace, error) {
	ws, err := a.handler.GetWorkspace(ctx, req.GetWorkspaceId())
	if err != nil {
		return nil, err
	}
	if ws == nil {
		return nil, nil
	}

	return workspaceToProto(ws), nil
}

// ListPlugins lists all plugins.
func (a *AdminServiceAdapter) ListPlugins(req *cortexv1.ListPluginsRequest, stream cortexv1.AdminService_ListPluginsServer) error {
	plugins := a.handler.ListPlugins()

	for _, p := range plugins {
		if err := stream.Send(&cortexv1.Plugin{
			Id:           p.ID,
			Name:         p.Name,
			Version:      p.Version,
			Capabilities: p.Capabilities,
			Enabled:      true,
		}); err != nil {
			return err
		}
	}

	return nil
}

// LoadPlugin loads a plugin.
func (a *AdminServiceAdapter) LoadPlugin(ctx context.Context, req *cortexv1.LoadPluginRequest) (*cortexv1.Plugin, error) {
	err := a.handler.LoadPlugin(ctx, req.Path)
	if err != nil {
		return nil, err
	}

	return &cortexv1.Plugin{
		Id:      req.Path,
		Enabled: true,
	}, nil
}

// UnloadPlugin unloads a plugin.
func (a *AdminServiceAdapter) UnloadPlugin(ctx context.Context, req *cortexv1.UnloadPluginRequest) (*cortexv1.UnloadPluginResult, error) {
	err := a.handler.UnloadPlugin(ctx, req.PluginId)
	if err != nil {
		msg := err.Error()
		return &cortexv1.UnloadPluginResult{
			Success: false,
			Message: &msg,
		}, nil
	}

	msg := "Plugin unloaded"
	return &cortexv1.UnloadPluginResult{
		Success: true,
		Message: &msg,
	}, nil
}

// GetPluginConfig returns plugin configuration.
func (a *AdminServiceAdapter) GetPluginConfig(ctx context.Context, req *cortexv1.GetPluginConfigRequest) (*cortexv1.PluginConfig, error) {
	return &cortexv1.PluginConfig{
		PluginId: req.PluginId,
		Settings: make(map[string]string),
	}, nil
}

// UpdatePluginConfig updates plugin configuration.
func (a *AdminServiceAdapter) UpdatePluginConfig(ctx context.Context, req *cortexv1.UpdatePluginConfigRequest) (*cortexv1.PluginConfig, error) {
	return &cortexv1.PluginConfig{
		PluginId: req.PluginId,
		Settings: make(map[string]string),
	}, nil
}

// HealthCheck performs a health check.
func (a *AdminServiceAdapter) HealthCheck(ctx context.Context, req *cortexv1.HealthCheckRequest) (*cortexv1.HealthCheckResult, error) {
	health, err := a.handler.HealthCheck(ctx)
	if err != nil {
		return nil, err
	}

	components := make(map[string]*cortexv1.ComponentHealth)
	for name, check := range health.Checks {
		ch := &cortexv1.ComponentHealth{
			Healthy: check.Status == "healthy",
			Status:  check.Status,
		}
		if check.Message != "" {
			ch.Error = &check.Message
		}
		components[name] = ch
	}

	return &cortexv1.HealthCheckResult{
		Healthy:    health.Status == "healthy",
		Status:     health.Status,
		Components: components,
	}, nil
}

// GetMetrics returns daemon metrics.
func (a *AdminServiceAdapter) GetMetrics(ctx context.Context, req *cortexv1.GetMetricsRequest) (*cortexv1.Metrics, error) {
	metrics, err := a.handler.GetMetrics(ctx)
	if err != nil {
		return nil, err
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return &cortexv1.Metrics{
		Timestamp: time.Now().Unix(),
		Resources: &cortexv1.ResourceUsage{
			MemoryBytes:    int64(m.HeapAlloc),
			MemoryMaxBytes: int64(m.HeapSys),
			GoroutineCount: int32(runtime.NumGoroutine()),
		},
		Counters: map[string]int64{
			"files_indexed":   int64(metrics.FilesIndexed),
			"tasks_processed": int64(metrics.TasksProcessed),
			"tasks_pending":   int64(metrics.TasksPending),
			"tasks_failed":    int64(metrics.TasksFailed),
			"gc_cycles":       int64(metrics.GCCycles),
			"uptime_seconds":  int64(metrics.Uptime.Seconds()),
		},
		Gauges: map[string]float64{
			"goroutines":   float64(metrics.Goroutines),
			"heap_alloc":   float64(metrics.HeapAlloc),
			"heap_sys":     float64(metrics.HeapSys),
			"heap_objects": float64(metrics.HeapObjects),
		},
	}, nil
}

// GetConfigVersions returns a list of configuration versions.
func (a *AdminServiceAdapter) GetConfigVersions(ctx context.Context, req *cortexv1.GetConfigVersionsRequest) (*cortexv1.ConfigVersionsList, error) {
	versions, total, err := a.handler.GetConfigVersions(ctx, int(req.GetLimit()), int(req.GetOffset()))
	if err != nil {
		return nil, err
	}

	protoVersions := make([]*cortexv1.ConfigVersion, 0, len(versions))
	for _, v := range versions {
		protoVersions = append(protoVersions, configVersionToProto(v))
	}

	return &cortexv1.ConfigVersionsList{
		Versions: protoVersions,
		Total:    int32(total),
	}, nil
}

// GetConfigVersion returns a specific configuration version.
func (a *AdminServiceAdapter) GetConfigVersion(ctx context.Context, req *cortexv1.GetConfigVersionRequest) (*cortexv1.ConfigVersion, error) {
	version, err := a.handler.GetConfigVersion(ctx, req.GetVersionId())
	if err != nil {
		return nil, err
	}
	return configVersionToProto(version), nil
}

// RestoreConfigVersion restores a configuration version.
func (a *AdminServiceAdapter) RestoreConfigVersion(ctx context.Context, req *cortexv1.RestoreConfigVersionRequest) (*cortexv1.Config, error) {
	cfg, err := a.handler.RestoreConfigVersion(ctx, req.GetVersionId(), req.GetCreateBackup(), req.GetDescription())
	if err != nil {
		return nil, err
	}
	_, path := a.handler.GetConfig()
	return configToProto(cfg, path), nil
}

// UpdatePrompts updates prompt templates.
func (a *AdminServiceAdapter) UpdatePrompts(ctx context.Context, req *cortexv1.UpdatePromptsRequest) (*cortexv1.PromptsConfig, error) {
	prompts, err := a.handler.UpdatePrompts(ctx, promptsFromProto(req.GetPrompts()), req.GetCreateVersion(), req.GetDescription())
	if err != nil {
		return nil, err
	}
	return promptsToProto(prompts), nil
}

func configVersionToProto(v *handlers.ConfigVersion) *cortexv1.ConfigVersion {
	if v == nil {
		return nil
	}
	return &cortexv1.ConfigVersion{
		VersionId:   v.VersionID,
		CreatedAt:   v.CreatedAt.Unix(),
		CreatedBy:   v.CreatedBy,
		Description: v.Description,
		Config:      configToProto(v.Config, ""),
		Metadata:    v.Metadata,
	}
}

// GetDashboardMetrics returns AI metadata quality dashboard metrics.
func (a *AdminServiceAdapter) GetDashboardMetrics(ctx context.Context, req *cortexv1.GetDashboardMetricsRequest) (*cortexv1.DashboardMetrics, error) {
	periodHours := req.GetPeriodHours()
	if periodHours == 0 {
		periodHours = 24 // Default to 24 hours
	}

	m, err := a.handler.GetDashboardMetrics(ctx, req.GetWorkspaceId(), periodHours)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get dashboard metrics: %v", err)
	}

	return dashboardMetricsToProto(m), nil
}

// GetConfidenceDistribution returns confidence score distributions.
func (a *AdminServiceAdapter) GetConfidenceDistribution(ctx context.Context, req *cortexv1.GetConfidenceDistributionRequest) (*cortexv1.ConfidenceDistributionResponse, error) {
	dists, err := a.handler.GetConfidenceDistribution(ctx, req.GetWorkspaceId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get confidence distribution: %v", err)
	}

	protoDists := make(map[string]*cortexv1.ConfidenceDistribution)
	for category, dist := range dists {
		protoDists[category] = confidenceDistributionToProto(dist)
	}

	return &cortexv1.ConfidenceDistributionResponse{
		Distributions: protoDists,
	}, nil
}

// GetModelDriftReport returns model drift detection report.
func (a *AdminServiceAdapter) GetModelDriftReport(ctx context.Context, req *cortexv1.GetModelDriftReportRequest) (*cortexv1.ModelDriftReport, error) {
	report, err := a.handler.GetModelDriftReport(ctx, req.GetWorkspaceId())
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to get model drift report: %v", err)
	}

	return driftReportToProto(report), nil
}

func dashboardMetricsToProto(m *metrics.DashboardMetrics) *cortexv1.DashboardMetrics {
	if m == nil {
		return &cortexv1.DashboardMetrics{}
	}

	// Convert confidence histogram
	confHist := make(map[string]int32)
	for k, v := range m.ConfidenceHistogram {
		confHist[k] = int32(v)
	}

	// Convert model versions
	modelVersions := make(map[string]int32)
	for k, v := range m.ModelVersions {
		modelVersions[k] = int32(v)
	}

	// Convert token usage
	tokenUsage := make(map[string]int64)
	for k, v := range m.TokenUsageByModel {
		tokenUsage[k] = v
	}

	// Convert cost by model
	costByModel := make(map[string]float64)
	for k, v := range m.CostByModel {
		costByModel[k] = v
	}

	// Convert hourly counts
	hourlyCounts := make([]int32, len(m.HourlyExtractionCounts))
	for i, v := range m.HourlyExtractionCounts {
		hourlyCounts[i] = int32(v)
	}

	return &cortexv1.DashboardMetrics{
		ExtractionSuccessRate:   m.ExtractionSuccessRate,
		ExtractionFailureCount:  int32(m.ExtractionFailureCount),
		AvgExtractionLatencyMs:  m.AvgExtractionLatencyMs,
		ConfidenceHistogram:     confHist,
		LowConfidenceCount:      int32(m.LowConfidenceCount),
		ModelVersions:           modelVersions,
		TokenUsageByModel:       tokenUsage,
		CostByModel:             costByModel,
		MissingMetadataCount:    int32(m.MissingMetadataCount),
		StaleMetadataCount:      int32(m.StaleMetadataCount),
		OrphanedRecordsCount:    int32(m.OrphanedRecordsCount),
		HourlyExtractionCounts:  hourlyCounts,
		DailyErrorRates:         m.DailyErrorRates,
		GeneratedAt:             m.GeneratedAt.Unix(),
		PeriodStart:             m.PeriodStart.Unix(),
		PeriodEnd:               m.PeriodEnd.Unix(),
	}
}

func confidenceDistributionToProto(d *metrics.ConfidenceDistribution) *cortexv1.ConfidenceDistribution {
	if d == nil {
		return &cortexv1.ConfidenceDistribution{}
	}

	dist := make(map[string]float64)
	for k, v := range d.Distribution {
		dist[k] = v
	}

	return &cortexv1.ConfidenceDistribution{
		Category:     d.Category,
		Distribution: dist,
		Mean:         d.Mean,
		Median:       d.Median,
		StdDev:       d.StdDev,
	}
}

func driftReportToProto(r *metrics.DriftReport) *cortexv1.ModelDriftReport {
	if r == nil {
		return &cortexv1.ModelDriftReport{}
	}

	indicators := make([]*cortexv1.DriftIndicator, len(r.Indicators))
	for i, ind := range r.Indicators {
		indicators[i] = &cortexv1.DriftIndicator{
			Metric:        ind.Metric,
			BaselineValue: ind.BaselineValue,
			CurrentValue:  ind.CurrentValue,
			ChangePercent: ind.ChangePercent,
			IsSignificant: ind.IsSignificant,
		}
	}

	return &cortexv1.ModelDriftReport{
		WorkspaceId:     string(r.WorkspaceID),
		PeriodHours:     int64(r.Period.Hours()),
		DriftDetected:   r.DriftDetected,
		DriftSeverity:   r.DriftSeverity,
		Indicators:      indicators,
		Recommendations: r.Recommendations,
		GeneratedAt:     r.GeneratedAt.Unix(),
	}
}
