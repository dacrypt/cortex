// Package main is the entry point for the Cortex daemon.
package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	embeddingApp "github.com/dacrypt/cortex/backend/internal/application/embedding"
	metadataApp "github.com/dacrypt/cortex/backend/internal/application/metadata"
	metricsApp "github.com/dacrypt/cortex/backend/internal/application/metrics"
	"github.com/dacrypt/cortex/backend/internal/application/clustering"
	"github.com/dacrypt/cortex/backend/internal/application/pipeline"
	"github.com/dacrypt/cortex/backend/internal/application/taxonomy"
	"github.com/dacrypt/cortex/backend/internal/application/pipeline/stages"
	"github.com/dacrypt/cortex/backend/internal/application/project"
	"github.com/dacrypt/cortex/backend/internal/application/rag"
	"github.com/dacrypt/cortex/backend/internal/domain/entity"
	"github.com/dacrypt/cortex/backend/internal/domain/event"
	"github.com/dacrypt/cortex/backend/internal/domain/service"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/config"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/embedding"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/llm/providers"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/metadata"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/mirror"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/persistence/sqlite"
	"github.com/dacrypt/cortex/backend/internal/infrastructure/trace"
	interfaceEvent "github.com/dacrypt/cortex/backend/internal/interfaces/event"
	grpcserver "github.com/dacrypt/cortex/backend/internal/interfaces/grpc"
	"github.com/dacrypt/cortex/backend/internal/interfaces/grpc/adapters"
	"github.com/dacrypt/cortex/backend/internal/interfaces/grpc/handlers"
	httpserver "github.com/dacrypt/cortex/backend/internal/interfaces/http"
	mcpserver "github.com/dacrypt/cortex/backend/internal/interfaces/mcp"
)

// Version information (set by build)
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// repositories holds all repository instances
type repositories struct {
	workspaceRepo       *sqlite.WorkspaceRepository
	fileRepo            *sqlite.FileRepository
	taskRepo            *sqlite.TaskRepository
	metadataRepo        *sqlite.MetadataRepository
	suggestedRepo       *sqlite.SuggestedMetadataRepository
	traceRepo           *sqlite.TraceRepository
	docRepo             *sqlite.DocumentRepository
	vectorStore         *sqlite.VectorStore
	projectRepo         *sqlite.ProjectRepository
	projectAssignRepo   *sqlite.ProjectAssignmentRepository
	documentStateRepo   *sqlite.DocumentStateRepository
	relationshipRepo    *sqlite.RelationshipRepository
	usageRepo           *sqlite.UsageRepository
	configVersionRepo   *sqlite.ConfigVersionRepository
	folderRepo          *sqlite.FolderRepository
	inferredProjectRepo *sqlite.InferredProjectRepository
	entityRepo          *sqlite.EntityRepository
	clusterRepo         *sqlite.ClusterRepository
	taxonomyRepo        *sqlite.TaxonomyRepository
}

// appHandlers holds all handler instances
type appHandlers struct {
	adminHandler      *handlers.AdminHandler
	fileHandler       *handlers.FileHandler
	metadataHandler   *handlers.MetadataHandler
	ragGrpcHandler    *handlers.RAGHandler
	llmHandler        *handlers.LLMHandler
	knowledgeHandler  *handlers.KnowledgeHandler
	entityHandler     *handlers.EntityHandler
	clusteringHandler *grpcserver.ClusteringHandler
	taxonomyHandler   *grpcserver.TaxonomyHandler
}

// appAdapters holds all adapter instances
type appAdapters struct {
	adminAdapter     *adapters.AdminServiceAdapter
	fileAdapter      *adapters.FileServiceAdapter
	metadataAdapter  *adapters.MetadataServiceAdapter
	llmAdapter       *adapters.LLMServiceAdapter
	ragAdapter       *adapters.RAGServiceAdapter
	knowledgeAdapter *adapters.KnowledgeServiceAdapter
	entityAdapter    *adapters.EntityServiceAdapter
}

// handlerInitConfig holds configuration for handler initialization
type handlerInitConfig struct {
	cfg              *config.Config
	configPath       string
	repos            *repositories
	orchestrator     *pipeline.Orchestrator
	publisher        *event.BufferedPublisher
	subscriber       *interfaceEvent.Subscriber
	progressTracker  *pipeline.ProgressTracker
	ragService       *rag.Service
	llmRouter        *llm.Router
	dashboardService *metricsApp.Service
	cancel           context.CancelFunc
	logger           zerolog.Logger
}

func main() {
	flags := parseFlagsAndShowVersion()

	cfg, logger := loadConfigAndSetupLogging(flags.configPath)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	publisher := event.NewBufferedPublisher(event.NewInMemoryPublisher(), 1000)
	defer publisher.Close()
	subscriber := interfaceEvent.NewSubscriber(publisher, logger)

	db, repos := initializeDatabaseAndRepositories(ctx, cfg, logger)
	defer db.Close()

	projectService := project.NewService(repos.projectRepo)

	// Initialize pipeline
	orchestrator := pipeline.NewOrchestrator(publisher, logger)

	// Define pipeline stages for progress tracking
	pipelineStages := []string{
		"start",
		"basic",
		"mime",
		"mirror",
		"code",
		"metadata",
		"os_metadata",
		"document",
		"relationship",
		"state",
		"suggestion",
		"enrichment",
		"folder_index",
		"project_inference",
		"ai",
		"complete",
	}

	// Initialize progress tracker
	progressTracker := pipeline.NewProgressTracker(pipelineStages)

	ragEmbedder := initializeEmbedder(cfg, logger)
	llmRouter := initializeLLMRouter(cfg, repos.traceRepo, logger)

	ragService := rag.NewService(repos.docRepo, repos.vectorStore, ragEmbedder, llmRouter, logger)
	ragHandler := httpserver.NewRAGHandler(ragService)

	suggestionService := initializeSuggestionService(cfg, ragService, llmRouter, repos, logger)

	// Initialize clustering service
	clusteringService := initializeClusteringService(repos, llmRouter, db, logger)
	clusteringHandler := grpcserver.NewClusteringHandler(clusteringService, logger)

	// Initialize taxonomy service
	taxonomyService := initializeTaxonomyService(repos, llmRouter, logger)
	taxonomyHandler := grpcserver.NewTaxonomyHandler(taxonomyService, logger)

	// Initialize dashboard metrics service
	dashboardService := metricsApp.NewService(
		nil, // benchmarkRepo - not yet wired up
		repos.metadataRepo,
		logger,
	)

	httpSrv := initializeHTTPServer(cfg, ragHandler, logger)

	handlerCfg := &handlerInitConfig{
		cfg:              cfg,
		configPath:       flags.configPath,
		repos:            repos,
		orchestrator:     orchestrator,
		publisher:        publisher,
		subscriber:       subscriber,
		progressTracker:  progressTracker,
		ragService:       ragService,
		llmRouter:        llmRouter,
		dashboardService: dashboardService,
		cancel:           cancel,
		logger:           logger,
	}
	appHandlers := initializeHandlers(handlerCfg)

	// Initialize Tika Manager (but don't start yet - will start after pipeline setup)
	var tikaManager *metadata.TikaManager
	if cfg.Tika.Enabled && cfg.Tika.ManageProcess {
		tikaManager = initializeTikaManager(cfg, logger)
	}

	setupPipelineStages(cfg, orchestrator, repos, ragEmbedder, llmRouter, suggestionService, clusteringService, tikaManager, logger)

	aiStage := createAIStage(cfg, llmRouter, repos, projectService, ragEmbedder, logger)
	orchestrator.AddStage(aiStage)

	// MCP mode: serve as MCP server on stdio for AI agents
	if flags.mcpMode {
		// Determine default workspace ID from watch_paths config
		var defaultWsID entity.WorkspaceID
		if len(cfg.WatchPaths) > 0 {
			defaultWsID = entity.WorkspaceID(cfg.WatchPaths[0])
		}

		mcpSrv := mcpserver.NewServer(mcpserver.Config{
			KnowledgeHandler: appHandlers.knowledgeHandler,
			FileHandler:      appHandlers.fileHandler,
			MetadataHandler:  appHandlers.metadataHandler,
			RAGHandler:       appHandlers.ragGrpcHandler,
			DefaultWorkspace: defaultWsID,
			Logger:           logger,
		})

		logger.Info().Msg("Starting Cortex in MCP server mode (stdio)")
		if err := mcpSrv.ServeStdio(); err != nil {
			logger.Fatal().Err(err).Msg("MCP server error")
		}
		return
	}

	appAdapters := initializeAdapters(appHandlers, repos.fileRepo, logger)

	server := initializeGRPCServer(ctx, cfg, publisher, appAdapters, clusteringHandler, taxonomyHandler, logger)

	logger.Info().
		Str("grpc_address", server.Address()).
		Msg("Cortex daemon started")

	// Start Tika Manager if it was initialized
	if tikaManager != nil {
		if err := tikaManager.Start(ctx); err != nil {
			logger.Warn().
				Err(err).
				Msg("Failed to start Tika Server - will use fallback extractors")
		} else {
			logger.Info().Msg("Tika Server started and managed by Cortex")
		}
	}

	watchers, err := startWatchers(ctx, cfg, repos.workspaceRepo, repos.fileRepo, orchestrator, logger)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to start watch_paths")
	}

	waitForShutdown(server, httpSrv, db, watchers, tikaManager, logger)
}

type cliFlags struct {
	configPath string
	mcpMode    bool
}

func parseFlagsAndShowVersion() cliFlags {
	configPath := flag.String("config", "", "Path to configuration file")
	showVersion := flag.Bool("version", false, "Show version information")
	mcpMode := flag.Bool("mcp", false, "Run as MCP server on stdio (for AI agents)")
	flag.Parse()

	if *showVersion {
		fmt.Printf("cortexd %s\n", Version)
		fmt.Printf("  Build time: %s\n", BuildTime)
		fmt.Printf("  Git commit: %s\n", GitCommit)
		fmt.Printf("  Go version: %s\n", runtime.Version())
		fmt.Printf("  OS/Arch:    %s/%s\n", runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	return cliFlags{
		configPath: *configPath,
		mcpMode:    *mcpMode,
	}
}

func loadConfigAndSetupLogging(configPath string) (*config.Config, zerolog.Logger) {
	cfg, err := config.Load(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load configuration: %v\n", err)
		os.Exit(1)
	}

	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
		os.Exit(1)
	}

	logger := setupLogging(cfg)
	logger.Info().
		Str("version", Version).
		Str("config_path", configPath).
		Msg("Starting Cortex daemon")

	return cfg, logger
}

func initializeDatabaseAndRepositories(ctx context.Context, cfg *config.Config, logger zerolog.Logger) (*sqlite.Connection, *repositories) {
	db, err := sqlite.NewConnection(cfg.DatabasePath())
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}

	logger.Info().Msg("Running database migrations")
	if err := db.Migrate(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Failed to run database migrations")
	}

	repos := &repositories{
		workspaceRepo:       sqlite.NewWorkspaceRepository(db),
		fileRepo:            sqlite.NewFileRepository(db),
		taskRepo:            sqlite.NewTaskRepository(db),
		metadataRepo:        sqlite.NewMetadataRepository(db),
		suggestedRepo:       sqlite.NewSuggestedMetadataRepository(db),
		traceRepo:           sqlite.NewTraceRepository(db),
		docRepo:             sqlite.NewDocumentRepository(db),
		vectorStore:         sqlite.NewVectorStore(db),
		projectRepo:         sqlite.NewProjectRepository(db),
		projectAssignRepo:   sqlite.NewProjectAssignmentRepository(db),
		documentStateRepo:   sqlite.NewDocumentStateRepository(db),
		relationshipRepo:    sqlite.NewRelationshipRepository(db),
		usageRepo:           sqlite.NewUsageRepository(db),
		configVersionRepo:   sqlite.NewConfigVersionRepository(db),
		folderRepo:          sqlite.NewFolderRepository(db),
		inferredProjectRepo: sqlite.NewInferredProjectRepository(db),
		clusterRepo:         sqlite.NewClusterRepository(db),
		taxonomyRepo:        sqlite.NewTaxonomyRepository(db),
	}

	// Create EntityRepository (requires other repos)
	entityRepo := sqlite.NewEntityRepository(
		db,
		repos.fileRepo,
		repos.folderRepo,
		repos.projectRepo,
		repos.metadataRepo,
	)
	repos.entityRepo = entityRepo

	return db, repos
}

func initializeEmbedder(cfg *config.Config, logger zerolog.Logger) embeddingApp.Embedder {
	if !cfg.LLM.Embeddings.Enabled {
		logger.Fatal().Msg("CRITICAL: llm.embeddings.enabled is false - RAG functionality requires proper embeddings")
	}

	if cfg.LLM.Embeddings.Endpoint == "" {
		logger.Fatal().Msg("CRITICAL: llm.embeddings.endpoint is required when llm.embeddings.enabled is true")
	}
	if cfg.LLM.Embeddings.Model == "" {
		logger.Fatal().Msg("CRITICAL: llm.embeddings.model is required when llm.embeddings.enabled is true")
	}

	ragEmbedder := embedding.NewOllamaEmbedder(
		cfg.LLM.Embeddings.Endpoint,
		cfg.LLM.Embeddings.Model,
	)

	// Check Ollama availability at startup with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if ragEmbedder.IsAvailable(ctx) {
		logger.Info().
			Str("endpoint", cfg.LLM.Embeddings.Endpoint).
			Str("model", cfg.LLM.Embeddings.Model).
			Bool("available", true).
			Msg("Ollama embeddings verified and ready")
	} else {
		logger.Warn().
			Str("endpoint", cfg.LLM.Embeddings.Endpoint).
			Str("model", cfg.LLM.Embeddings.Model).
			Bool("available", false).
			Msg("Ollama embedding service unavailable - RAG will not work until service is restored")
	}

	return ragEmbedder
}

func initializeLLMRouter(cfg *config.Config, traceRepo *sqlite.TraceRepository, logger zerolog.Logger) *llm.Router {
	llmRouter := llm.NewRouter(logger)
	llmRouter.SetTraceWriter(trace.NewWriter(traceRepo, logger))
	llmRouter.SetMustSucceed(cfg.LLM.MustSucceed)

	if !cfg.LLM.Enabled {
		logger.Warn().Msg("LLM features are disabled in configuration - AI capabilities will not be available")
		return llmRouter
	}

	promptsAdapter := llm.NewConfigPromptsAdapter(cfg.LLM.Prompts)
	llmRouter.SetPrompts(promptsAdapter)

	registeredProviders := registerLLMProviders(cfg, llmRouter, logger)
	if registeredProviders == 0 {
		logger.Fatal().
			Int("configured_providers", len(cfg.LLM.Providers)).
			Msg("CRITICAL: No LLM providers were successfully registered - AI features require at least one working provider")
	}

	logger.Info().
		Int("registered_providers", registeredProviders).
		Str("default_provider", cfg.LLM.DefaultProvider).
		Str("default_model", cfg.LLM.DefaultModel).
		Msg("LLM providers initialized")

	if err := llmRouter.SetActiveProvider(cfg.LLM.DefaultProvider, cfg.LLM.DefaultModel); err != nil {
		logger.Fatal().
			Err(err).
			Str("default_provider", cfg.LLM.DefaultProvider).
			Str("default_model", cfg.LLM.DefaultModel).
			Msg("CRITICAL: Failed to set active LLM provider - cannot proceed")
	}

	verifyLLMProviderAvailability(llmRouter, cfg.LLM.DefaultProvider, cfg.LLM.DefaultModel, logger)

	return llmRouter
}

func registerLLMProviders(cfg *config.Config, llmRouter *llm.Router, logger zerolog.Logger) int {
	registeredProviders := 0
	for _, providerCfg := range cfg.LLM.Providers {
		var provider llm.Provider
		switch providerCfg.Type {
		case "ollama":
			provider = providers.NewOllamaProvider(providerCfg.ID, providerCfg.ID, providerCfg.Endpoint)
			llmRouter.RegisterProvider(provider)
			registeredProviders++
			logger.Info().
				Str("provider_id", providerCfg.ID).
				Str("endpoint", providerCfg.Endpoint).
				Msg("Registered Ollama LLM provider")
		case "openai":
			apiKey := providerCfg.APIKey
			if apiKey == "" {
				apiKey = os.Getenv("OPENAI_API_KEY")
			}
			provider = providers.NewOpenAIProvider(providerCfg.ID, providerCfg.ID, providerCfg.Endpoint, apiKey)
			llmRouter.RegisterProvider(provider)
			registeredProviders++
			logger.Info().
				Str("provider_id", providerCfg.ID).
				Str("endpoint", providerCfg.Endpoint).
				Bool("has_api_key", apiKey != "").
				Msg("Registered OpenAI-compatible LLM provider")
		default:
			logger.Error().
				Str("provider_id", providerCfg.ID).
				Str("provider_type", providerCfg.Type).
				Msg("Unsupported LLM provider type - this is a configuration error")
		}
	}
	return registeredProviders
}

func verifyLLMProviderAvailability(llmRouter *llm.Router, providerID, model string, logger zerolog.Logger) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if available := llmRouter.IsAvailable(ctx); !available {
		logger.Warn().
			Str("provider", providerID).
			Msg("Active LLM provider is not available - AI features may fail")
	} else {
		logger.Info().
			Str("provider", providerID).
			Str("model", model).
			Msg("Active LLM provider is available and ready")
	}
}

func initializeSuggestionService(cfg *config.Config, ragService *rag.Service, llmRouter *llm.Router, repos *repositories, logger zerolog.Logger) *metadataApp.SuggestionService {
	if !cfg.LLM.Enabled || !cfg.LLM.Embeddings.Enabled {
		logger.Debug().Msg("Metadata suggestion service disabled (requires LLM and embeddings)")
		return nil
	}

	ragAdapter := metadataApp.NewRAGServiceAdapter(ragService)
	llmAdapter := metadataApp.NewLLMServiceAdapter(llmRouter)
	suggestionService := metadataApp.NewSuggestionService(
		ragAdapter,
		llmAdapter,
		repos.metadataRepo,
		repos.docRepo,
		repos.projectRepo,
		logger,
	)
	logger.Info().Msg("Metadata suggestion service enabled (RAG + LLM)")
	return suggestionService
}

func initializeHTTPServer(cfg *config.Config, ragHandler *httpserver.RAGHandler, logger zerolog.Logger) *httpserver.Server {
	httpSrv := httpserver.NewServer(httpserver.Config{
		Addr:   cfg.HTTPAddress,
		Logger: logger,
		ExtraHandlers: map[string]http.Handler{
			"/query": http.HandlerFunc(ragHandler.HandleQuery),
		},
	})
	if err := httpSrv.Start(); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start HTTP server")
	}
	return httpSrv
}

func initializeHandlers(cfg *handlerInitConfig) *appHandlers {
	adminHandler := handlers.NewAdminHandler(handlers.AdminHandlerConfig{
		WorkspaceRepo:     cfg.repos.workspaceRepo,
		FileRepo:          cfg.repos.fileRepo,
		TaskRepo:          cfg.repos.taskRepo,
		Config:            cfg.cfg,
		ConfigPath:        cfg.configPath,
		ConfigVersionRepo: cfg.repos.configVersionRepo,
		DashboardService:  cfg.dashboardService,
		Version:           Version,
		Logger:            cfg.logger,
		ShutdownFunc:      cfg.cancel,
		Subscriber:        cfg.subscriber,
		ProgressTracker:   cfg.progressTracker,
	})

	fileHandler := handlers.NewFileHandler(handlers.FileHandlerConfig{
		FileRepo:      cfg.repos.fileRepo,
		MetaRepo:      cfg.repos.metadataRepo,
		WorkspaceRepo: cfg.repos.workspaceRepo,
		Pipeline:      cfg.orchestrator,
		Publisher:     cfg.publisher,
		Logger:        cfg.logger,
		WorkerCount:   cfg.cfg.WorkerCount,
	})

	metadataHandler := handlers.NewMetadataHandler(cfg.repos.metadataRepo, cfg.repos.traceRepo, cfg.publisher, cfg.logger)
	metadataHandler.SetSuggestedMetadataRepository(cfg.repos.suggestedRepo)

	ragGrpcHandler := handlers.NewRAGHandler(cfg.ragService, cfg.logger)

	llmHandler := handlers.NewLLMHandler(cfg.llmRouter, cfg.logger)

	knowledgeHandler := handlers.NewKnowledgeHandler(handlers.KnowledgeHandlerConfig{
		ProjectRepo:      cfg.repos.projectRepo,
		DocumentRepo:     cfg.repos.docRepo,
		StateRepo:        cfg.repos.documentStateRepo,
		RelationshipRepo: cfg.repos.relationshipRepo,
		UsageRepo:        cfg.repos.usageRepo,
		FileRepo:         cfg.repos.fileRepo,
		MetaRepo:         cfg.repos.metadataRepo,
		AssignmentRepo:   cfg.repos.projectAssignRepo,
		ClusterRepo:      cfg.repos.clusterRepo,
		Logger:           cfg.logger,
	})

	entityHandler := handlers.NewEntityHandler(handlers.EntityHandlerConfig{
		EntityRepo: cfg.repos.entityRepo,
		Logger:     cfg.logger,
	})

	return &appHandlers{
		adminHandler:     adminHandler,
		fileHandler:      fileHandler,
		metadataHandler:  metadataHandler,
		ragGrpcHandler:   ragGrpcHandler,
		llmHandler:       llmHandler,
		knowledgeHandler: knowledgeHandler,
		entityHandler:    entityHandler,
	}
}

func setupPipelineStages(cfg *config.Config, orchestrator *pipeline.Orchestrator, repos *repositories, ragEmbedder embeddingApp.Embedder, llmRouter *llm.Router, suggestionService *metadataApp.SuggestionService, clusteringService *clustering.Service, tikaManager *metadata.TikaManager, logger zerolog.Logger) {
	mirrorExtractor := &mirror.Extractor{
		Logger:      logger.With().Str("component", "mirror").Logger(),
		MaxFileSize: cfg.Mirror.MaxFileSizeMB * 1024 * 1024,
	}
	var ocrService *metadata.OCRService
	if cfg.Enrichment.OCREnabled {
		ocrService = metadata.NewOCRService(logger)
	}
	mirrorStage := stages.NewMirrorStage(mirrorExtractor, repos.metadataRepo, ocrService, logger)
	if err := orchestrator.InsertStage(2, mirrorStage); err != nil {
		logger.Warn().Err(err).Msg("Failed to insert mirror stage")
	}

	metadataRegistry := metadata.NewRegistry()
	
	// Register Tika extractor first (if enabled) - it has the broadest support
	if cfg.Tika.Enabled {
		var tikaClient *metadata.TikaClient
		var endpoint string
		
		// Use manager's endpoint if managing process, otherwise use configured endpoint
		if tikaManager != nil {
			// Tika Manager will start later, but we can still register the service
			// It will check if Tika is available when needed
			endpoint = tikaManager.GetEndpoint()
			tikaClient = metadata.NewTikaClient(endpoint, cfg.Tika.Timeout, logger)
			logger.Info().
				Str("endpoint", endpoint).
				Msg("Using managed Tika Server")
			fallbackExtractor := metadata.NewUniversalExtractor(logger)
			tikaService := metadata.NewTikaService(tikaClient, true, fallbackExtractor, logger)
			metadataRegistry.Register(tikaService)
		} else {
			// Not managing process - check if external Tika Server is available
			endpoint = cfg.Tika.Endpoint
			tikaClient = metadata.NewTikaClient(endpoint, cfg.Tika.Timeout, logger)
			
			// Health check Tika Server
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			if err := tikaClient.HealthCheck(ctx); err != nil {
				logger.Warn().
					Err(err).
					Str("endpoint", endpoint).
					Msg("Tika Server not available, will use fallback extractors")
				cancel()
				// Don't register Tika if not available
			} else {
				logger.Info().
					Str("endpoint", endpoint).
					Msg("Tika Server is available")
				cancel()
				fallbackExtractor := metadata.NewUniversalExtractor(logger)
				metadataRegistry.Register(metadata.NewTikaService(tikaClient, true, fallbackExtractor, logger))
			}
		}
	}
	
	// Register fallback extractors (used when Tika is disabled or fails)
	metadataRegistry.Register(metadata.NewPDFExtractor(logger))
	metadataRegistry.Register(metadata.NewImageExtractor(logger))
	metadataRegistry.Register(metadata.NewAudioExtractor(logger))
	metadataRegistry.Register(metadata.NewVideoExtractor(logger))
	metadataRegistry.Register(metadata.NewUniversalExtractor(logger))
	
	metadataStage := stages.NewMetadataStage(metadataRegistry, logger)
	orchestrator.AddStage(metadataStage)
	
	if cfg.Tika.Enabled {
		logger.Info().Msg("Metadata extraction stage enabled (Tika + fallback extractors)")
	} else {
		logger.Info().Msg("Metadata extraction stage enabled (PDF, Image, Audio, Video)")
	}

	// OS Metadata Stage - extracts permissions, ownership, ACLs, extended attributes
	if cfg.OSMetadata.Enabled {
		osMetadataStage := stages.NewOSMetadataStage(repos.fileRepo, logger)
		orchestrator.AddStage(osMetadataStage)
		logger.Info().Msg("OS metadata extraction stage enabled (permissions, ownership, ACLs)")
	}

	if cfg.LLM.Embeddings.Enabled {
		docStage := stages.NewDocumentStage(repos.metadataRepo, repos.docRepo, repos.vectorStore, ragEmbedder, logger)
		orchestrator.AddStage(docStage)
		logger.Info().Msg("Document embedding stage enabled (before AI stage)")
	}

	relationshipStage := stages.NewRelationshipStage(repos.docRepo, repos.relationshipRepo, logger)
	orchestrator.AddStage(relationshipStage)
	logger.Info().Msg("Relationship detection stage enabled")

	stateStage := stages.NewStateStage(repos.docRepo, repos.documentStateRepo, repos.relationshipRepo, logger)
	orchestrator.AddStage(stateStage)
	logger.Info().Msg("Document state inference stage enabled")

	if suggestionService != nil {
		suggestionStage := stages.NewSuggestionStage(
			suggestionService,
			repos.metadataRepo,
			repos.suggestedRepo,
			logger,
			cfg.LLM.AutoIndex.Enabled,
		)
		orchestrator.AddStage(suggestionStage)
		logger.Info().Msg("Metadata suggestion stage enabled (RAG + LLM)")
	}

	// Enrichment Stage - NER, citations, sentiment, OCR, tables, formulas, transcription
	if cfg.Enrichment.Enabled {
		enrichmentStage := stages.NewEnrichmentStage(
			llmRouter,
			repos.metadataRepo,
			repos.docRepo,
			repos.vectorStore,
			ragEmbedder,
			logger,
			stages.EnrichmentConfig{
				Enabled:                   cfg.Enrichment.Enabled,
				NEREnabled:                cfg.Enrichment.NEREnabled,
				CitationsEnabled:          cfg.Enrichment.CitationsEnabled,
				SentimentEnabled:          cfg.Enrichment.SentimentEnabled,
				OCREnabled:                cfg.Enrichment.OCREnabled,
				TablesEnabled:             cfg.Enrichment.TablesEnabled,
				FormulasEnabled:           cfg.Enrichment.FormulasEnabled,
				DependenciesEnabled:       cfg.Enrichment.DependenciesEnabled,
				TranscriptionEnabled:      cfg.Enrichment.TranscriptionEnabled,
				DuplicateDetectionEnabled: cfg.Enrichment.DuplicateDetectionEnabled,
				ISBNEnrichmentEnabled:     cfg.Enrichment.ISBNEnrichmentEnabled,
			},
		)
		orchestrator.AddStage(enrichmentStage)
		logger.Info().
			Bool("ner", cfg.Enrichment.NEREnabled).
			Bool("citations", cfg.Enrichment.CitationsEnabled).
			Bool("tables", cfg.Enrichment.TablesEnabled).
			Bool("duplicates", cfg.Enrichment.DuplicateDetectionEnabled).
			Msg("Enrichment stage enabled")
	}

	// Folder Index Stage - indexes folders as first-class entities with aggregated metrics
	if cfg.FolderIndex.Enabled {
		folderIndexStage := stages.NewFolderIndexStage(
			stages.FolderIndexConfig{
				Enabled:                  cfg.FolderIndex.Enabled,
				InferProjectsFromFolders: cfg.FolderIndex.InferProjectsFromFolders,
				MinFilesForProject:       cfg.FolderIndex.MinFilesForProject,
			},
			repos.folderRepo,
			logger,
		)
		orchestrator.AddStage(folderIndexStage)
		logger.Info().
			Bool("infer_projects", cfg.FolderIndex.InferProjectsFromFolders).
			Int("min_files", cfg.FolderIndex.MinFilesForProject).
			Msg("Folder indexing stage enabled")
	}

	// Project Inference Stage - automatically infers projects from folder structure
	if cfg.FolderIndex.Enabled && cfg.FolderIndex.InferProjectsFromFolders {
		projectInferenceStage := stages.NewProjectInferenceStage(
			stages.ProjectInferenceConfig{
				Enabled:              true,
				MinFilesForProject:   cfg.FolderIndex.MinFilesForProject,
				ConfidenceThreshold:  0.3, // Minimum confidence to consider a folder as a project
				UseAIForDescription:  cfg.LLM.Enabled,
				MaxProjectsPerFolder: 1,
			},
			repos.inferredProjectRepo,
			logger,
		)
		orchestrator.AddStage(projectInferenceStage)
		logger.Info().Msg("Project inference stage enabled (auto-create projects from folders)")
	}

	// Temporal Cluster Stage - groups files edited together and propagates project assignments
	if cfg.ProjectAutoAssign.Enabled {
		temporalClusterConfig := metadataApp.TemporalClusterConfig{
			WindowHours:         cfg.ProjectAutoAssign.WindowHours,
			MinClusterSize:      cfg.ProjectAutoAssign.MinClusterSize,
			DominanceThreshold:  cfg.ProjectAutoAssign.DominanceThreshold,
			SuggestionThreshold: cfg.ProjectAutoAssign.SuggestionThreshold,
		}
		temporalClusterStage := stages.NewTemporalClusterStage(
			repos.usageRepo,
			repos.metadataRepo,
			temporalClusterConfig,
			logger,
		)
		orchestrator.AddStage(temporalClusterStage)
		logger.Info().
			Int("window_hours", cfg.ProjectAutoAssign.WindowHours).
			Int("min_cluster_size", cfg.ProjectAutoAssign.MinClusterSize).
			Float64("dominance_threshold", cfg.ProjectAutoAssign.DominanceThreshold).
			Float64("suggestion_threshold", cfg.ProjectAutoAssign.SuggestionThreshold).
			Msg("Temporal clustering stage enabled (auto-assign projects from edit patterns)")
	}

	// Clustering Stage - automatically runs document clustering after indexing
	if cfg.Clustering.Enabled && clusteringService != nil {
		clusteringStage := stages.NewClusteringStage(
			clusteringService,
			stages.ClusteringStageConfig{
				Enabled:            true,
				MinDocumentsToRun:  cfg.Clustering.MinDocumentsToRun,
				ClusteringInterval: time.Duration(cfg.Clustering.ClusteringInterval) * time.Minute,
				RunOnFinalize:      cfg.Clustering.RunOnFinalize,
			},
			logger,
		)
		orchestrator.AddStage(clusteringStage)
		logger.Info().
			Int("min_documents", cfg.Clustering.MinDocumentsToRun).
			Int("interval_minutes", cfg.Clustering.ClusteringInterval).
			Bool("run_on_finalize", cfg.Clustering.RunOnFinalize).
			Msg("Document clustering stage enabled (automatic cluster detection)")
	}
}

func createAIStage(cfg *config.Config, llmRouter *llm.Router, repos *repositories, projectService *project.Service, ragEmbedder embeddingApp.Embedder, logger zerolog.Logger) *stages.AIStage {
	maxFileSize := cfg.LLM.AutoSummary.MaxFileSize
	if cfg.LLM.AutoIndex.MaxFileSize > maxFileSize {
		maxFileSize = cfg.LLM.AutoIndex.MaxFileSize
	}

	if cfg.LLM.Embeddings.Enabled {
		return stages.NewAIStageWithRAG(
			llmRouter,
			repos.metadataRepo,
			repos.suggestedRepo,
			repos.fileRepo,
			repos.docRepo,
			repos.vectorStore,
			ragEmbedder,
			repos.projectRepo,
			repos.projectAssignRepo,
			projectService,
			logger,
			stages.AIStageConfig{
				Enabled:                cfg.LLM.Enabled,
				AutoSummaryEnabled:     cfg.LLM.AutoSummary.Enabled,
				AutoIndexEnabled:       cfg.LLM.AutoIndex.Enabled,
				ApplyTags:              cfg.LLM.AutoIndex.ApplyTags,
				ApplyProjects:          cfg.LLM.AutoIndex.ApplyProjects,
				UseSuggestedContexts:   cfg.LLM.AutoIndex.UseSuggestedContexts,
				MaxFileSize:            maxFileSize,
				MaxTags:                cfg.LLM.AutoIndex.MaxTags,
				RequestTimeout:         time.Duration(cfg.LLM.RequestTimeoutMs) * time.Millisecond,
				MustSucceed:            cfg.LLM.MustSucceed,
				CategoryEnabled:        cfg.LLM.AutoIndex.EnableCategories,
				RelatedEnabled:         cfg.LLM.AutoIndex.EnableRelated,
				RelatedMaxResults:      cfg.LLM.AutoIndex.MaxRelatedResults,
				RelatedCandidates:      cfg.LLM.AutoIndex.RelatedCandidates,
				UseRAGForCategories:    cfg.LLM.AutoIndex.UseRAGForCategories,
				UseRAGForTags:          cfg.LLM.AutoIndex.UseRAGForTags,
				UseRAGForProjects:      cfg.LLM.AutoIndex.UseRAGForProjects,
				UseRAGForRelated:       cfg.LLM.AutoIndex.UseRAGForRelated,
				UseRAGForSummary:       cfg.LLM.AutoIndex.UseRAGForSummary,
				RAGSimilarityThreshold: cfg.LLM.AutoIndex.RAGSimilarityThreshold,
			},
		)
	}

	return stages.NewAIStage(
		llmRouter,
		repos.metadataRepo,
		repos.suggestedRepo,
		repos.fileRepo,
		repos.projectRepo,
		repos.projectAssignRepo,
		projectService,
		logger,
		stages.AIStageConfig{
			Enabled:              cfg.LLM.Enabled,
			AutoSummaryEnabled:   cfg.LLM.AutoSummary.Enabled,
			AutoIndexEnabled:     cfg.LLM.AutoIndex.Enabled,
			ApplyTags:            cfg.LLM.AutoIndex.ApplyTags,
			ApplyProjects:        cfg.LLM.AutoIndex.ApplyProjects,
			UseSuggestedContexts: cfg.LLM.AutoIndex.UseSuggestedContexts,
			MaxFileSize:          maxFileSize,
			MaxTags:              cfg.LLM.AutoIndex.MaxTags,
			RequestTimeout:       time.Duration(cfg.LLM.RequestTimeoutMs) * time.Millisecond,
			MustSucceed:          cfg.LLM.MustSucceed,
			CategoryEnabled:      cfg.LLM.AutoIndex.EnableCategories,
			RelatedEnabled:       cfg.LLM.AutoIndex.EnableRelated,
			RelatedMaxResults:    cfg.LLM.AutoIndex.MaxRelatedResults,
			RelatedCandidates:    cfg.LLM.AutoIndex.RelatedCandidates,
		},
	)
}

func initializeAdapters(appHandlers *appHandlers, fileRepo *sqlite.FileRepository, logger zerolog.Logger) *appAdapters {
	return &appAdapters{
		adminAdapter:     adapters.NewAdminServiceAdapter(appHandlers.adminHandler),
		fileAdapter:      adapters.NewFileServiceAdapter(appHandlers.fileHandler),
		metadataAdapter:  adapters.NewMetadataServiceAdapter(appHandlers.metadataHandler, fileRepo),
		llmAdapter:       adapters.NewLLMServiceAdapter(appHandlers.llmHandler),
		ragAdapter:       adapters.NewRAGServiceAdapter(appHandlers.ragGrpcHandler),
		knowledgeAdapter: adapters.NewKnowledgeServiceAdapter(appHandlers.knowledgeHandler, logger),
		entityAdapter:    adapters.NewEntityServiceAdapter(appHandlers.entityHandler),
	}
}

func initializeGRPCServer(ctx context.Context, cfg *config.Config, publisher *event.BufferedPublisher, adapters *appAdapters, clusteringHandler *grpcserver.ClusteringHandler, taxonomyHandler *grpcserver.TaxonomyHandler, logger zerolog.Logger) *grpcserver.Server {
	server := grpcserver.NewServer(cfg, logger, publisher)
	server.RegisterAdminService(adapters.adminAdapter)
	server.RegisterFileService(adapters.fileAdapter)
	server.RegisterMetadataService(adapters.metadataAdapter)
	server.RegisterLLMService(adapters.llmAdapter)
	server.RegisterRAGService(adapters.ragAdapter)
	server.RegisterKnowledgeService(adapters.knowledgeAdapter)
	server.RegisterEntityService(adapters.entityAdapter)
	if clusteringHandler != nil {
		server.RegisterClusteringService(clusteringHandler)
	}
	if taxonomyHandler != nil {
		server.RegisterTaxonomyService(taxonomyHandler)
	}
	if err := server.Start(ctx); err != nil {
		logger.Fatal().Err(err).Msg("Failed to start gRPC server")
	}
	return server
}

func waitForShutdown(server *grpcserver.Server, httpSrv *httpserver.Server, db *sqlite.Connection, watchers []service.FileWatcher, tikaManager *metadata.TikaManager, logger zerolog.Logger) {
	shutdown := make(chan os.Signal, 1)
	signal.Notify(shutdown, syscall.SIGINT, syscall.SIGTERM)

	sig := <-shutdown
	logger.Info().
		Str("signal", sig.String()).
		Msg("Received shutdown signal")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	// Stop Tika Manager first
	if tikaManager != nil {
		if err := tikaManager.Stop(); err != nil {
			logger.Warn().Err(err).Msg("Error stopping Tika Server")
		}
	}

	for _, watcher := range watchers {
		if err := watcher.Stop(); err != nil {
			logger.Warn().Err(err).Msg("Failed to stop watcher")
		}
	}

	if err := server.Stop(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("Error stopping gRPC server")
	}

	if err := httpSrv.Stop(shutdownCtx); err != nil {
		logger.Error().Err(err).Msg("Error stopping HTTP server")
	}

	if err := db.Close(); err != nil {
		logger.Error().Err(err).Msg("Error closing database")
	}

	logger.Info().Msg("Cortex daemon stopped")
}

func setupLogging(cfg *config.Config) zerolog.Logger {
	// Set log level
	level, err := zerolog.ParseLevel(cfg.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)

	// Configure output
	var output = zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}

	logger := zerolog.New(output).
		With().
		Timestamp().
		Str("service", "cortexd").
		Logger()

	// If log file specified, also log to file
	if cfg.LogFile != "" {
		file, err := os.OpenFile(cfg.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to open log file")
		} else {
			multi := zerolog.MultiLevelWriter(output, file)
			logger = zerolog.New(multi).
				With().
				Timestamp().
				Str("service", "cortexd").
				Logger()
		}
	}

	return logger
}

func initializeClusteringService(repos *repositories, llmRouter *llm.Router, db *sqlite.Connection, logger zerolog.Logger) *clustering.Service {
	// Create embedding connection using VectorStore
	embeddingConn := clustering.NewVectorStoreEmbeddingConnection(
		repos.vectorStore,
		repos.docRepo,
		db, // SQLConnection interface
		logger,
	)

	// Create graph builder
	graphBuilder := clustering.NewGraphBuilder(
		repos.docRepo,
		repos.fileRepo,
		nil, // entityRepo - entity-based edges are optional
		repos.usageRepo,
		repos.clusterRepo,
		embeddingConn, // embedding connection for semantic edges
		clustering.DefaultGraphBuilderConfig(),
		logger,
	)

	// Create community detector
	communityDetector := clustering.NewCommunityDetector(
		clustering.DefaultCommunityDetectorConfig(),
		logger,
	)

	// Create LLM validator (optional - can be nil if LLM is disabled)
	var llmValidator *clustering.LLMValidator
	if llmRouter != nil {
		// Create document info provider to give LLM context about documents
		docInfoProvider := clustering.NewDocumentInfoProvider(
			repos.docRepo,
			repos.metadataRepo,
		)
		llmValidator = clustering.NewLLMValidator(
			llmRouter,
			docInfoProvider,
			clustering.DefaultLLMValidatorConfig(),
			logger,
		)
	}

	// Create clustering service
	clusteringService := clustering.NewService(
		graphBuilder,
		communityDetector,
		llmValidator,
		repos.clusterRepo,
		repos.projectRepo,
		clustering.DefaultServiceConfig(),
		logger,
	)

	logger.Info().Msg("Clustering service initialized")
	return clusteringService
}

func initializeTaxonomyService(repos *repositories, llmRouter *llm.Router, logger zerolog.Logger) *taxonomy.Service {
	// Create LLM adapter for taxonomy service (can be nil if LLM is disabled)
	var taxonomyLLM taxonomy.LLMRouter
	if llmRouter != nil {
		taxonomyLLM = &taxonomyLLMAdapter{router: llmRouter}
	}

	taxonomyService := taxonomy.NewService(
		taxonomy.DefaultServiceConfig(),
		repos.taxonomyRepo,
		repos.fileRepo,
		repos.docRepo,
		taxonomyLLM,
		logger,
	)

	logger.Info().Msg("Taxonomy service initialized")
	return taxonomyService
}

// taxonomyLLMAdapter adapts llm.Router to taxonomy.LLMRouter interface
type taxonomyLLMAdapter struct {
	router *llm.Router
}

func (a *taxonomyLLMAdapter) Complete(ctx context.Context, prompt string, maxTokens int) (string, error) {
	resp, err := a.router.Generate(ctx, llm.GenerateRequest{
		Prompt:    prompt,
		MaxTokens: maxTokens,
	})
	if err != nil {
		return "", err
	}
	return resp.Text, nil
}
