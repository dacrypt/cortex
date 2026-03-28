// Package config provides configuration management for the Cortex daemon.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Config holds the daemon configuration.
type Config struct {
	// Server settings
	GRPCAddress string `mapstructure:"grpc_address"`
	HTTPAddress string `mapstructure:"http_address"`

	// Storage settings
	DataDir   string `mapstructure:"data_dir"`
	PluginDir string `mapstructure:"plugin_dir"`

	// Worker settings
	WorkerCount        int `mapstructure:"worker_count"`
	MaxConcurrentTasks int `mapstructure:"max_concurrent_tasks"`

	// LLM settings
	LLM LLMConfig `mapstructure:"llm"`

	// Logging settings
	LogLevel string `mapstructure:"log_level"`
	LogFile  string `mapstructure:"log_file"`

	// Watch settings
	WatchPaths []string `mapstructure:"watch_paths"`

	// Mirror settings
	Mirror MirrorConfig `mapstructure:"mirror"`

	// Enrichment settings
	Enrichment EnrichmentConfig `mapstructure:"enrichment"`

	// OS Metadata settings
	OSMetadata OSMetadataConfig `mapstructure:"os_metadata"`

	// Folder indexing settings
	FolderIndex FolderIndexConfig `mapstructure:"folder_index"`

	// Project auto-assignment settings
	ProjectAutoAssign ProjectAutoAssignConfig `mapstructure:"project_auto_assign"`

	// Clustering settings
	Clustering ClusteringConfig `mapstructure:"clustering"`

	// Tika settings
	Tika TikaConfig `mapstructure:"tika"`

	// Vision settings (image understanding via LLM vision models)
	Vision VisionConfig `mapstructure:"vision"`
}

// VisionConfig holds image understanding settings.
type VisionConfig struct {
	Enabled       bool   `mapstructure:"enabled"`
	Model         string `mapstructure:"model"`           // e.g., "llama3.2-vision"
	MaxFileSizeMB int64  `mapstructure:"max_file_size_mb"` // default: 10
	Prompt        string `mapstructure:"prompt"`
}

// LLMConfig holds LLM-specific configuration.
type LLMConfig struct {
	Enabled          bool              `mapstructure:"enabled"`
	DefaultProvider  string            `mapstructure:"default_provider"`
	DefaultModel     string            `mapstructure:"default_model"`
	MaxContextTokens int               `mapstructure:"max_context_tokens"`
	RequestTimeoutMs int               `mapstructure:"request_timeout_ms"`
	MustSucceed      bool              `mapstructure:"must_succeed"`
	Providers        []ProviderConfig  `mapstructure:"providers"`
	AutoSummary      AutoSummaryConfig `mapstructure:"auto_summary"`
	AutoIndex        AutoIndexConfig   `mapstructure:"auto_index"`
	Embeddings       EmbeddingsConfig  `mapstructure:"embeddings"`
	Prompts          PromptsConfig     `mapstructure:"prompts"`
}

// EmbeddingsConfig holds embedding-specific configuration.
type EmbeddingsConfig struct {
	Enabled  bool   `mapstructure:"enabled"`
	Endpoint string `mapstructure:"endpoint"`
	Model    string `mapstructure:"model"`
}

// AutoSummaryConfig holds AI summary settings.
type AutoSummaryConfig struct {
	Enabled     bool  `mapstructure:"enabled"`
	MaxFileSize int64 `mapstructure:"max_file_size"`
}

// AutoIndexConfig holds AI tag/project settings.
type AutoIndexConfig struct {
	Enabled              bool  `mapstructure:"enabled"`
	ApplyTags            bool  `mapstructure:"apply_tags"`
	ApplyProjects        bool  `mapstructure:"apply_projects"`
	UseSuggestedContexts bool  `mapstructure:"use_suggested_contexts"`
	MaxFileSize          int64 `mapstructure:"max_file_size"`
	MaxTags              int   `mapstructure:"max_tags"`
	EnableCategories     bool  `mapstructure:"enable_categories"`
	EnableRelated        bool  `mapstructure:"enable_related"`
	MaxRelatedResults    int   `mapstructure:"max_related_results"`
	RelatedCandidates    int   `mapstructure:"related_candidates"`
	// RAG options
	UseRAGForCategories    bool    `mapstructure:"use_rag_for_categories"`
	UseRAGForTags          bool    `mapstructure:"use_rag_for_tags"`
	UseRAGForProjects      bool    `mapstructure:"use_rag_for_projects"`
	UseRAGForRelated       bool    `mapstructure:"use_rag_for_related"`
	UseRAGForSummary       bool    `mapstructure:"use_rag_for_summary"`
	RAGSimilarityThreshold float32 `mapstructure:"rag_similarity_threshold"`
}

// ProviderConfig holds per-provider configuration.
type ProviderConfig struct {
	ID       string            `mapstructure:"id"`
	Type     string            `mapstructure:"type"` // ollama, lmstudio, openai, anthropic
	Endpoint string            `mapstructure:"endpoint"`
	APIKey   string            `mapstructure:"api_key"`
	Options  map[string]string `mapstructure:"options"`
}

// MirrorConfig holds mirror extraction settings.
type MirrorConfig struct {
	MaxFileSizeMB int64 `mapstructure:"max_file_size_mb"`
}

// EnrichmentConfig holds enrichment stage settings.
type EnrichmentConfig struct {
	Enabled              bool `mapstructure:"enabled"`
	NEREnabled           bool `mapstructure:"ner_enabled"`
	CitationsEnabled     bool `mapstructure:"citations_enabled"`
	SentimentEnabled     bool `mapstructure:"sentiment_enabled"`
	OCREnabled           bool `mapstructure:"ocr_enabled"`
	TablesEnabled        bool `mapstructure:"tables_enabled"`
	FormulasEnabled      bool `mapstructure:"formulas_enabled"`
	DependenciesEnabled  bool `mapstructure:"dependencies_enabled"`
	TranscriptionEnabled bool `mapstructure:"transcription_enabled"`
	DuplicateDetectionEnabled bool `mapstructure:"duplicate_detection_enabled"`
	ISBNEnrichmentEnabled bool `mapstructure:"isbn_enrichment_enabled"`
}

// OSMetadataConfig holds OS metadata extraction settings.
type OSMetadataConfig struct {
	Enabled bool `mapstructure:"enabled"`
}

// FolderIndexConfig holds folder indexing settings.
type FolderIndexConfig struct {
	Enabled                  bool `mapstructure:"enabled"`
	InferProjectsFromFolders bool `mapstructure:"infer_projects_from_folders"`
	MinFilesForProject       int  `mapstructure:"min_files_for_project"`
}

// ProjectAutoAssignConfig holds automatic project assignment settings.
type ProjectAutoAssignConfig struct {
	Enabled             bool    `mapstructure:"enabled"`
	WindowHours         int     `mapstructure:"window_hours"`
	MinClusterSize      int     `mapstructure:"min_cluster_size"`
	DominanceThreshold  float64 `mapstructure:"dominance_threshold"`
	SuggestionThreshold float64 `mapstructure:"suggestion_threshold"`
}

// ClusteringConfig holds document clustering settings.
type ClusteringConfig struct {
	Enabled            bool `mapstructure:"enabled"`
	MinDocumentsToRun  int  `mapstructure:"min_documents_to_run"`  // Minimum documents before running clustering
	ClusteringInterval int  `mapstructure:"clustering_interval"`  // Minutes between clustering runs
	RunOnFinalize      bool `mapstructure:"run_on_finalize"`       // Run clustering at pipeline finalization
}

// TikaConfig holds Apache Tika Server configuration.
type TikaConfig struct {
	Enabled        bool          `mapstructure:"enabled"`
	ManageProcess  bool          `mapstructure:"manage_process"`  // Auto-start/stop Tika Server
	AutoDownload   bool          `mapstructure:"auto_download"`   // Automatically download JAR if not found
	JarPath        string        `mapstructure:"jar_path"`        // Path to tika-server-standard.jar (empty = auto-detect or download)
	Endpoint       string        `mapstructure:"endpoint"`        // http://localhost:9998
	Port           int           `mapstructure:"port"`           // Port for Tika Server
	Timeout        time.Duration `mapstructure:"timeout"`        // 30s
	MaxFileSize    int64         `mapstructure:"max_file_size"`  // 100MB
	StartupTimeout time.Duration `mapstructure:"startup_timeout"` // Timeout for Tika to start
	HealthInterval time.Duration `mapstructure:"health_interval"` // Interval for health checks
	MaxRestarts    int           `mapstructure:"max_restarts"`    // Max restart attempts
	RestartDelay   time.Duration `mapstructure:"restart_delay"`   // Delay before restart
}

// PromptsConfig holds all AI prompt templates.
type PromptsConfig struct {
	SuggestTags              string `mapstructure:"suggest_tags"`
	SuggestProject           string `mapstructure:"suggest_project"`
	GenerateSummary          string `mapstructure:"generate_summary"`
	ExtractKeyTerms          string `mapstructure:"extract_key_terms"`
	RAGAnswer                string `mapstructure:"rag_answer"`
	ClassifyCategory         string `mapstructure:"classify_category"`
	SuggestProjectNature     string `mapstructure:"suggest_project_nature"`
	GenerateProjectDescription string `mapstructure:"generate_project_description"`
	ValidateProjectName      string `mapstructure:"validate_project_name"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	homeDir, _ := os.UserHomeDir()
	dataDir := filepath.Join(homeDir, ".cortex")

	return &Config{
		GRPCAddress:        "localhost:50051",
		HTTPAddress:        "localhost:8080",
		DataDir:            dataDir,
		PluginDir:          filepath.Join(dataDir, "plugins"),
		WorkerCount:        4,
		MaxConcurrentTasks: 10,
		WatchPaths:         []string{},
		LLM: LLMConfig{
			Enabled:          true,
			DefaultProvider:  "ollama",
			DefaultModel:     "llama3.2",
			MaxContextTokens: 2000,
			RequestTimeoutMs: 1800000, // 30 minutes - effectively unlimited for analysis
			MustSucceed:      true,
			Providers: []ProviderConfig{
				{
					ID:       "ollama",
					Type:     "ollama",
					Endpoint: "http://localhost:11434",
				},
				{
					ID:       "lmstudio",
					Type:     "openai",
					Endpoint: "http://localhost:1234/v1",
				},
			},
			AutoSummary: AutoSummaryConfig{
				Enabled:     true,
				MaxFileSize: 250000,
			},
			AutoIndex: AutoIndexConfig{
				Enabled:                true,
				ApplyTags:              true,
				ApplyProjects:          true,
				UseSuggestedContexts:   true,
				MaxFileSize:            250000,
				MaxTags:                5,
				EnableCategories:       true,
				EnableRelated:          true,
				MaxRelatedResults:      10,
				RelatedCandidates:      100,
				UseRAGForCategories:    true,
				UseRAGForTags:          true,
				UseRAGForProjects:      true,
				UseRAGForRelated:       true,
				UseRAGForSummary:       true,
				RAGSimilarityThreshold: 0.5,
			},
			Embeddings: EmbeddingsConfig{
				Enabled:  true,
				Endpoint: "http://localhost:11434",
				Model:    "nomic-embed-text",
			},
			Prompts: DefaultPromptsConfig(),
		},
		LogLevel: "info",
		Mirror: MirrorConfig{
			MaxFileSizeMB: 25,
		},
		Enrichment: EnrichmentConfig{
			Enabled:                   true,
			NEREnabled:                true,
			CitationsEnabled:          true,
			SentimentEnabled:          false, // Disabled by default (expensive)
			OCREnabled:                false, // Disabled by default (requires tesseract)
			TablesEnabled:             true,
			FormulasEnabled:           true,
			DependenciesEnabled:       true,
			TranscriptionEnabled:      false, // Disabled by default (requires whisper)
			DuplicateDetectionEnabled: true,
			ISBNEnrichmentEnabled:     true,
		},
		OSMetadata: OSMetadataConfig{
			Enabled: true,
		},
		FolderIndex: FolderIndexConfig{
			Enabled:                  true,
			InferProjectsFromFolders: true,
			MinFilesForProject:       3,
		},
		ProjectAutoAssign: ProjectAutoAssignConfig{
			Enabled:             true,
			WindowHours:         6,
			MinClusterSize:      2,
			DominanceThreshold:  0.6,
			SuggestionThreshold: 0.3,
		},
		Clustering: ClusteringConfig{
			Enabled:            true,
			MinDocumentsToRun:  10,
			ClusteringInterval: 5, // minutes
			RunOnFinalize:      true,
		},
		Tika: TikaConfig{
			Enabled:        false, // Disabled by default (requires Tika Server)
			ManageProcess:  true,  // Auto-manage Tika Server lifecycle
			AutoDownload:   true,  // Automatically download JAR if not found
			JarPath:        "",    // Auto-detect or download
			Endpoint:       "http://localhost:9998",
			Port:           9998,
			Timeout:        30 * time.Second,
			MaxFileSize:    104857600, // 100MB
			StartupTimeout: 30 * time.Second,
			HealthInterval: 10 * time.Second,
			MaxRestarts:    3,
			RestartDelay:   5 * time.Second,
		},
	}
}

// Load loads configuration from file and environment.
func Load(configPath string) (*Config, error) {
	v := viper.New()

	// Set defaults
	defaults := DefaultConfig()
	v.SetDefault("grpc_address", defaults.GRPCAddress)
	v.SetDefault("http_address", defaults.HTTPAddress)
	v.SetDefault("data_dir", defaults.DataDir)
	v.SetDefault("plugin_dir", defaults.PluginDir)
	v.SetDefault("worker_count", defaults.WorkerCount)
	v.SetDefault("max_concurrent_tasks", defaults.MaxConcurrentTasks)
	v.SetDefault("log_level", defaults.LogLevel)
	v.SetDefault("watch_paths", defaults.WatchPaths)
	v.SetDefault("llm.enabled", defaults.LLM.Enabled)
	v.SetDefault("llm.default_provider", defaults.LLM.DefaultProvider)
	v.SetDefault("llm.default_model", defaults.LLM.DefaultModel)
	v.SetDefault("llm.max_context_tokens", defaults.LLM.MaxContextTokens)
	v.SetDefault("llm.request_timeout_ms", defaults.LLM.RequestTimeoutMs)
	v.SetDefault("llm.must_succeed", defaults.LLM.MustSucceed)
	v.SetDefault("mirror.max_file_size_mb", defaults.Mirror.MaxFileSizeMB)
	v.SetDefault("llm.auto_summary.enabled", defaults.LLM.AutoSummary.Enabled)
	v.SetDefault("llm.auto_summary.max_file_size", defaults.LLM.AutoSummary.MaxFileSize)
	v.SetDefault("llm.auto_index.enabled", defaults.LLM.AutoIndex.Enabled)
	v.SetDefault("llm.auto_index.apply_tags", defaults.LLM.AutoIndex.ApplyTags)
	v.SetDefault("llm.auto_index.apply_projects", defaults.LLM.AutoIndex.ApplyProjects)
	v.SetDefault("llm.auto_index.use_suggested_contexts", defaults.LLM.AutoIndex.UseSuggestedContexts)
	v.SetDefault("llm.auto_index.max_file_size", defaults.LLM.AutoIndex.MaxFileSize)
	v.SetDefault("llm.auto_index.max_tags", defaults.LLM.AutoIndex.MaxTags)
	v.SetDefault("llm.auto_index.enable_categories", defaults.LLM.AutoIndex.EnableCategories)
	v.SetDefault("llm.auto_index.enable_related", defaults.LLM.AutoIndex.EnableRelated)
	v.SetDefault("llm.auto_index.max_related_results", defaults.LLM.AutoIndex.MaxRelatedResults)
	v.SetDefault("llm.auto_index.related_candidates", defaults.LLM.AutoIndex.RelatedCandidates)
	v.SetDefault("llm.auto_index.use_rag_for_categories", defaults.LLM.AutoIndex.UseRAGForCategories)
	v.SetDefault("llm.auto_index.use_rag_for_tags", defaults.LLM.AutoIndex.UseRAGForTags)
	v.SetDefault("llm.auto_index.use_rag_for_projects", defaults.LLM.AutoIndex.UseRAGForProjects)
	v.SetDefault("llm.auto_index.use_rag_for_related", defaults.LLM.AutoIndex.UseRAGForRelated)
	v.SetDefault("llm.auto_index.use_rag_for_summary", defaults.LLM.AutoIndex.UseRAGForSummary)
	v.SetDefault("llm.auto_index.rag_similarity_threshold", defaults.LLM.AutoIndex.RAGSimilarityThreshold)
	v.SetDefault("llm.embeddings.enabled", defaults.LLM.Embeddings.Enabled)
	v.SetDefault("llm.embeddings.endpoint", defaults.LLM.Embeddings.Endpoint)
	v.SetDefault("llm.embeddings.model", defaults.LLM.Embeddings.Model)

	// Tika defaults
	v.SetDefault("tika.enabled", defaults.Tika.Enabled)
	v.SetDefault("tika.manage_process", defaults.Tika.ManageProcess)
	v.SetDefault("tika.auto_download", defaults.Tika.AutoDownload)
	v.SetDefault("tika.jar_path", defaults.Tika.JarPath)
	v.SetDefault("tika.endpoint", defaults.Tika.Endpoint)
	v.SetDefault("tika.port", defaults.Tika.Port)
	v.SetDefault("tika.timeout", defaults.Tika.Timeout)
	v.SetDefault("tika.max_file_size", defaults.Tika.MaxFileSize)
	v.SetDefault("tika.startup_timeout", defaults.Tika.StartupTimeout)
	v.SetDefault("tika.health_interval", defaults.Tika.HealthInterval)
	v.SetDefault("tika.max_restarts", defaults.Tika.MaxRestarts)
	v.SetDefault("tika.restart_delay", defaults.Tika.RestartDelay)

	// Project auto-assignment defaults
	v.SetDefault("project_auto_assign.enabled", defaults.ProjectAutoAssign.Enabled)
	v.SetDefault("project_auto_assign.window_hours", defaults.ProjectAutoAssign.WindowHours)
	v.SetDefault("project_auto_assign.min_cluster_size", defaults.ProjectAutoAssign.MinClusterSize)
	v.SetDefault("project_auto_assign.dominance_threshold", defaults.ProjectAutoAssign.DominanceThreshold)
	v.SetDefault("project_auto_assign.suggestion_threshold", defaults.ProjectAutoAssign.SuggestionThreshold)

	// Config file
	if configPath != "" {
		v.SetConfigFile(configPath)
	} else {
		v.SetConfigName("cortexd")
		v.SetConfigType("yaml")
		v.AddConfigPath(".")
		v.AddConfigPath(defaults.DataDir)
		v.AddConfigPath("/etc/cortex")
	}

	// Environment variables
	v.SetEnvPrefix("CORTEX")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	// Read config file (ignore if not found)
	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal into struct
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Apply default prompts if empty
	defaultPrompts := DefaultPromptsConfig()
	if cfg.LLM.Prompts.SuggestTags == "" {
		cfg.LLM.Prompts.SuggestTags = defaultPrompts.SuggestTags
	}
	if cfg.LLM.Prompts.SuggestProject == "" {
		cfg.LLM.Prompts.SuggestProject = defaultPrompts.SuggestProject
	}
	if cfg.LLM.Prompts.GenerateSummary == "" {
		cfg.LLM.Prompts.GenerateSummary = defaultPrompts.GenerateSummary
	}
	if cfg.LLM.Prompts.ExtractKeyTerms == "" {
		cfg.LLM.Prompts.ExtractKeyTerms = defaultPrompts.ExtractKeyTerms
	}
	if cfg.LLM.Prompts.RAGAnswer == "" {
		cfg.LLM.Prompts.RAGAnswer = defaultPrompts.RAGAnswer
	}
	if cfg.LLM.Prompts.ClassifyCategory == "" {
		cfg.LLM.Prompts.ClassifyCategory = defaultPrompts.ClassifyCategory
	}
	if cfg.LLM.Prompts.SuggestProjectNature == "" {
		cfg.LLM.Prompts.SuggestProjectNature = defaultPrompts.SuggestProjectNature
	}
	if cfg.LLM.Prompts.GenerateProjectDescription == "" {
		cfg.LLM.Prompts.GenerateProjectDescription = defaultPrompts.GenerateProjectDescription
	}
	if cfg.LLM.Prompts.ValidateProjectName == "" {
		cfg.LLM.Prompts.ValidateProjectName = defaultPrompts.ValidateProjectName
	}

	// Ensure directories exist
	if err := os.MkdirAll(cfg.DataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create data directory: %w", err)
	}
	if err := os.MkdirAll(cfg.PluginDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create plugin directory: %w", err)
	}

	return &cfg, nil
}

// DatabasePath returns the path to the main database.
func (c *Config) DatabasePath() string {
	return filepath.Join(c.DataDir, "cortex.sqlite")
}

// Validate validates the configuration and ensures quality requirements.
// This is a mandatory step - configuration errors will prevent startup.
func (c *Config) Validate() error {
	if err := c.validateBasic(); err != nil {
		return err
	}
	if c.LLM.Enabled {
		if err := c.validateLLM(); err != nil {
			return err
		}
	}
	return nil
}

// validateBasic validates basic configuration requirements.
func (c *Config) validateBasic() error {
	if c.GRPCAddress == "" {
		return fmt.Errorf("grpc_address is required")
	}
	if c.DataDir == "" {
		return fmt.Errorf("data_dir is required")
	}
	if c.WorkerCount < 1 {
		return fmt.Errorf("worker_count must be at least 1")
	}
	if c.MaxConcurrentTasks < 1 {
		return fmt.Errorf("max_concurrent_tasks must be at least 1")
	}
	return nil
}

// validateLLM validates LLM configuration and ensures quality requirements.
func (c *Config) validateLLM() error {
	if len(c.LLM.Providers) == 0 {
		return fmt.Errorf("llm.enabled is true but no providers are configured - at least one provider is required")
	}

	if err := c.validateLLMProviders(); err != nil {
		return err
	}

	if err := c.validateLLMDefaultProvider(); err != nil {
		return err
	}

	if c.LLM.DefaultModel == "" {
		return fmt.Errorf("llm.default_model is required when llm.enabled is true")
	}

	if c.LLM.Embeddings.Enabled {
		if err := c.validateEmbeddings(); err != nil {
			return err
		}
	}

	return nil
}

// validateLLMProviders validates all LLM provider configurations.
func (c *Config) validateLLMProviders() error {
	supportedTypes := map[string]bool{
		"ollama": true,
		"openai": true,
	}

	for i, providerCfg := range c.LLM.Providers {
		if !supportedTypes[providerCfg.Type] {
			return fmt.Errorf("unsupported LLM provider type '%s' for provider '%s' (index %d) - supported types: ollama, openai",
				providerCfg.Type, providerCfg.ID, i)
		}
		if providerCfg.ID == "" {
			return fmt.Errorf("llm.providers[%d].id is required", i)
		}
		if providerCfg.Endpoint == "" {
			return fmt.Errorf("llm.providers[%d].endpoint is required for provider '%s'", i, providerCfg.ID)
		}
	}

	return nil
}

// validateLLMDefaultProvider validates that the default provider exists.
func (c *Config) validateLLMDefaultProvider() error {
	if c.LLM.DefaultProvider == "" {
		return fmt.Errorf("llm.default_provider is required when llm.enabled is true")
	}

	for _, providerCfg := range c.LLM.Providers {
		if providerCfg.ID == c.LLM.DefaultProvider {
			return nil
		}
	}

	return fmt.Errorf("llm.default_provider '%s' is not in the configured providers list", c.LLM.DefaultProvider)
}

// validateEmbeddings validates embeddings configuration.
func (c *Config) validateEmbeddings() error {
	if c.LLM.Embeddings.Endpoint == "" {
		return fmt.Errorf("llm.embeddings.endpoint is required when llm.embeddings.enabled is true")
	}
	if c.LLM.Embeddings.Model == "" {
		return fmt.Errorf("llm.embeddings.model is required when llm.embeddings.enabled is true")
	}
	return nil
}

// DefaultPromptsConfig returns default prompt templates.
func DefaultPromptsConfig() PromptsConfig {
	return PromptsConfig{
		SuggestTags: `Analyze the following content and suggest up to %d relevant tags in Spanish.
Use concise noun phrases (1-3 words), avoid duplicates or near-duplicates, and avoid punctuation.
Return only a JSON array of tag strings, nothing else.

Content:
%s

Tags (JSON array):`,

		SuggestProject: `Eres un asistente experto en organización de documentos.

Basándote en la siguiente información, sugiere el proyecto/contexto más apropiado de la lista a continuación.
Si ninguno de los proyectos existentes encaja, sugiere un nuevo nombre de proyecto.

REGLAS CRÍTICAS:
1. El nombre del proyecto DEBE estar en ESPAÑOL, el mismo idioma que el contenido.
2. El nombre debe ser descriptivo y relevante al contenido.
3. Responde SOLO con el nombre del proyecto, sin explicaciones, sin comillas, sin puntos finales.

Proyectos existentes:
%s

%s

Proyecto sugerido:`,

		GenerateSummary: `Resume el siguiente contenido en %d palabras o menos.
Sé conciso y captura los puntos principales.
IMPORTANTE: El resumen DEBE estar en español, el mismo idioma que el contenido.

Contenido:
%s

Resumen:`,

		ExtractKeyTerms: `Extrae los términos clave más importantes del siguiente resumen.
Responde SOLO con una lista de términos separados por comas, sin explicaciones.
Máximo 12 términos. Los términos DEBEN estar en español.
Evita palabras comunes como: que, los, del, por, una, con, las, pero, para, etc.

Resumen:
%s

Términos clave:`,

		RAGAnswer: `Eres un asistente experto de Cortex. Tu tarea es responder la pregunta del usuario utilizando UNICAMENTE el contexto proporcionado.
Si la información no está en el contexto, indica amablemente que no tienes esa información en tus documentos.
Utiliza un tono profesional y directo. Cita siempre las fuentes usando corchetes como [1], [2], etc., al final de la frase o párrafo que utiliza esa información.

Contexto:
%s

Pregunta: %s

Respuesta del asistente:`,

		ClassifyCategory: `Eres un bibliotecario experto que clasifica documentos en categorías temáticas como en una biblioteca.

Basándote en la información proporcionada, clasifica este documento en UNA de las siguientes categorías de biblioteca (en español):

- %s

%s

Responde SOLO con el nombre exacto de la categoría de la lista (sin comillas, sin punto final, sin explicaciones).
Si no puedes determinar la categoría, responde: "Sin Clasificar"`,

		SuggestProjectNature: `Sugiere la naturaleza/tipo de un proyecto basándote en su nombre y descripción.

Nombre del proyecto: "%s"
Descripción: "%s"

Proyectos existentes similares:
%s

Responde en JSON:
{
  "nature": "tipo de proyecto (ej: desarrollo, documentación, investigación, etc.)",
  "confidence": 0.0-1.0
}`,

		GenerateProjectDescription: `Generate a brief, professional description for a project.

Project Name: "%s"
Project Type: "%s"

Requirements:
- 1-2 sentences maximum
- Professional and clear
- Describes the project's purpose
- In the same language as the project name

Respond in JSON format:
{
  "description": "the description text",
  "confidence": 0.0-1.0
}

Only respond with valid JSON, no other text.`,

		ValidateProjectName: `Valida y sugiere mejoras para el nombre de un proyecto.

Nombre propuesto: "%s"
Proyectos existentes:
%s

Responde en JSON:
{
  "valid": true/false,
  "suggested_name": "nombre sugerido si es necesario",
  "reason": "razón de la validación o sugerencia",
  "confidence": 0.0-1.0
}`,
	}
}
