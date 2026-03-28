# Cortex Backend (cortexd)

The Go daemon that powers Cortex. Handles all file indexing, AI processing, storage, and search. Exposes a gRPC API consumed by the VS Code extension.

## Building

```bash
make build    # Produces ./cortexd binary
make run      # Build and run with default config
make test     # Run all tests
make proto    # Regenerate gRPC code from .proto files
make clean    # Remove binaries
```

**Requirements:** Go 1.22+

## Running

```bash
# First time: create a local config from the template
cp configs/cortexd.yaml.example cortexd.local.yaml

# Edit cortexd.local.yaml:
#   - Set watch_paths to directories you want indexed
#   - Adjust LLM settings if needed

# Run the daemon
./cortexd --config cortexd.local.yaml
```

The daemon starts a gRPC server (default `localhost:50051`) and immediately begins indexing files in the configured `watch_paths`.

### With auto-reload (development)

The `bin/start.sh` script uses [Air](https://github.com/air-verse/air) for live reloading:

```bash
# From the repository root
./bin/start.sh
```

## Configuration Reference

All options are set in the YAML config file. See [`configs/cortexd.yaml.example`](configs/cortexd.yaml.example) for a complete annotated example.

### Server

| Key | Default | Description |
|-----|---------|-------------|
| `grpc_address` | `localhost:50051` | gRPC server bind address |
| `http_address` | `localhost:8081` | HTTP server address |
| `data_dir` | `./cortex-data` | Database and state directory |
| `worker_count` | `4` | Concurrent pipeline workers |
| `max_concurrent_tasks` | `10` | Task queue depth |
| `log_level` | `info` | `debug`, `info`, `warn`, `error` |
| `watch_paths` | `[]` | Directories to index and watch for changes |

### LLM

| Key | Default | Description |
|-----|---------|-------------|
| `llm.enabled` | `true` | Enable AI features |
| `llm.default_provider` | `ollama` | `ollama`, `lmstudio`, `openai`, `anthropic` |
| `llm.default_model` | `llama3.2` | Model name |
| `llm.max_context_tokens` | `2000` | Max tokens sent to the LLM |
| `llm.request_timeout_ms` | `30000` | Request timeout in milliseconds |

### Auto Summary

| Key | Default | Description |
|-----|---------|-------------|
| `llm.auto_summary.enabled` | `true` | Auto-generate file summaries |
| `llm.auto_summary.max_file_size` | `250000` | Max file size in bytes (0 = no limit) |

### Auto Index

| Key | Default | Description |
|-----|---------|-------------|
| `llm.auto_index.enabled` | `true` | Auto-generate tags and projects |
| `llm.auto_index.apply_tags` | `true` | Apply AI tags automatically |
| `llm.auto_index.apply_projects` | `true` | Apply AI projects automatically |
| `llm.auto_index.max_tags` | `5` | Max tag suggestions per file |
| `llm.auto_index.enable_categories` | `true` | Enable category classification |
| `llm.auto_index.enable_related` | `true` | Find related files |
| `llm.auto_index.use_rag_for_*` | `true` | Use RAG to improve suggestions |
| `llm.auto_index.rag_similarity_threshold` | `0.5` | Similarity threshold (0.0–1.0) |

### Embeddings

| Key | Default | Description |
|-----|---------|-------------|
| `llm.embeddings.enabled` | `true` | Enable vector embeddings for RAG |
| `llm.embeddings.endpoint` | `http://localhost:11434` | Ollama endpoint |
| `llm.embeddings.model` | `nomic-embed-text` | Embedding model |

### Tika (Document Extraction)

| Key | Default | Description |
|-----|---------|-------------|
| `tika.enabled` | `true` | Enable Apache Tika |
| `tika.manage_process` | `true` | Auto-start/stop Tika Server |
| `tika.auto_download` | `true` | Auto-download Tika JAR |
| `tika.endpoint` | `http://localhost:9998` | Tika server URL |
| `tika.timeout` | `30s` | Request timeout |
| `tika.max_file_size` | `104857600` | Max file size (100 MB) |

### LLM Providers

Configure multiple providers in the `llm.providers` array:

```yaml
llm:
  providers:
    - id: "ollama"
      type: "ollama"
      endpoint: "http://localhost:11434"

    - id: "openai"
      type: "openai"
      endpoint: "https://api.openai.com/v1"
      api_key: "${OPENAI_API_KEY}"     # Use environment variables
```

Supported types: `ollama`, `openai` (also works with LM Studio), `anthropic`.

## Architecture

The backend follows clean architecture with four layers:

```
interfaces/grpc/     ← gRPC handlers (API surface)
    ↓
application/         ← Business logic, pipeline orchestrator
    ↓
domain/              ← Entities, repository interfaces, events
    ↓
infrastructure/      ← SQLite repos, LLM providers, file system
```

### Processing Pipeline

Each file passes through a sequence of stages:

```
basic → mime → mirror → code → document → metadata → os_metadata
  → folder_index → relationship → state → suggestion → enrichment
  → clustering → temporal_cluster → project_inference → ai → complete
```

Each stage is independent and can skip files that don't apply (e.g., `code` skips non-code files). Stages publish events that the VS Code extension consumes via `StreamPipeline` for real-time progress updates.

### Database

SQLite database (via `modernc.org/sqlite`, pure Go — no CGO required).

**Core tables:**

| Table | Purpose |
|-------|---------|
| `workspaces` | Registered workspace paths and configuration |
| `files` | File index with per-stage indexing flags |
| `file_metadata` | Tags, projects, notes, AI summaries |
| `file_tags` | Tag assignments (many-to-many) |
| `file_contexts` | Project assignments (many-to-many) |
| `documents` | Extracted text content (markdown) |
| `chunks` | Document chunks for RAG |
| `chunk_embeddings` | Vector embeddings |
| `projects` | Project hierarchy with parent relationships |
| `project_assignments` | Scored file-to-project mappings |
| `document_states` | Document lifecycle states |
| `document_relationships` | Cross-document relationships |
| `usage_events` | Access and interaction tracking |

Migrations run automatically on startup.

## gRPC API

Proto definitions are in [`api/proto/cortex/v1/`](api/proto/cortex/v1/).

### AdminService (`admin.proto`)

Daemon control and workspace management.

| Method | Description |
|--------|-------------|
| `GetStatus` | Daemon version, uptime, file count, resource usage |
| `Shutdown` | Graceful shutdown with configurable timeout |
| `Reload` | Reload configuration and/or plugins |
| `GetConfig` / `UpdateConfig` | Read and update daemon configuration |
| `RegisterWorkspace` | Register a directory for indexing |
| `UnregisterWorkspace` | Remove a workspace |
| `ListWorkspaces` | List all registered workspaces |
| `HealthCheck` | Health status including plugin checks |
| `GetDashboardMetrics` | Aggregated metrics for the admin dashboard |
| `StreamPipeline` | Server-streaming — real-time pipeline events |

### FileService (`file.proto`)

File queries and workspace scanning.

| Method | Description |
|--------|-------------|
| `ScanWorkspace` | Trigger full or incremental scan (streaming progress) |
| `GetFile` | Get a single file by ID or path |
| `ListFiles` | List files with pagination and sorting |
| `SearchFiles` | Search files by query string |
| `WatchFiles` | Stream file change events in real time |
| `ProcessFile` | Reprocess a single file through the pipeline |
| `ListByType` / `ListByFolder` / `ListByDate` / `ListBySize` | Grouped queries |

### MetadataService (`metadata.proto`)

Semantic metadata operations.

| Method | Description |
|--------|-------------|
| `AddTag` / `RemoveTag` | Manage file tags |
| `AddContext` / `RemoveContext` | Manage project assignments |
| `GetMetadata` | Get all metadata for a file |
| `GetAllTags` / `GetTagCounts` | Tag inventory |
| `UpdateNotes` | Set notes on a file |
| `UpdateAISummary` | Store AI-generated summary |
| `AddSuggestedContext` / `AcceptSuggestion` / `DismissSuggestion` | AI suggestion workflow |

### LLMService (`llm.proto`)

AI operations and provider management.

| Method | Description |
|--------|-------------|
| `ListProviders` / `GetProviderStatus` | Query available LLM providers |
| `SetActiveProvider` | Switch the active provider and model |
| `SuggestTags` | Generate tag suggestions for a file |
| `SuggestProject` | Generate project suggestions for a file |
| `GenerateSummary` | Generate a file summary |
| `ClassifyCategory` | Classify a file into categories |
| `FindRelatedFiles` | Find semantically related files |
| `GenerateCompletion` / `StreamCompletion` | Raw LLM completion |

### RAGService (`rag.proto`)

Retrieval-augmented generation.

| Method | Description |
|--------|-------------|
| `Query` | Ask a question — retrieves context, generates answer with citations |
| `SemanticSearch` | Vector similarity search without LLM generation |
| `GetIndexStats` | Embedding index statistics (document count, chunk count) |

### KnowledgeService (`knowledge.proto`)

Projects, relationships, states, and usage analytics.

| Method | Description |
|--------|-------------|
| `CreateProject` / `GetProject` / `UpdateProject` / `DeleteProject` | Project CRUD |
| `ListProjects` / `GetProjectChildren` / `GetProjectParents` | Project hierarchy |
| `AddDocumentToProject` / `RemoveDocumentFromProject` | File-project linking |
| `AddDocumentRelationship` / `GetDocumentRelationships` | Cross-document relationships |
| `SetDocumentState` / `GetDocumentState` / `GetDocumentStateHistory` | Document lifecycle |
| `RecordUsage` / `GetUsageStats` | Usage tracking |
| `GetFacets` | Faceted metadata for UI rendering |
| `GenerateGraph` / `GenerateHeatmap` | Visualization data |

### TaxonomyService (`taxonomy.proto`)

Hierarchical category management.

| Method | Description |
|--------|-------------|
| `GetRootNodes` / `GetChildren` / `GetAncestors` | Tree navigation |
| `CreateNode` / `UpdateNode` / `DeleteNode` | Category CRUD |
| `AddFileToNode` / `RemoveFileFromNode` | File-category assignments |
| `InduceTaxonomy` | AI-driven taxonomy generation |
| `SuggestTaxonomy` | Get category suggestions for a file |

### ClusteringService (`clustering.proto`)

Document clustering and graph analysis.

| Method | Description |
|--------|-------------|
| `RunClustering` | Execute clustering algorithm |
| `GetClusters` / `GetCluster` | Query clusters |
| `GetDocumentGraph` | Get the full document similarity graph |
| `MergeClusters` | Manually merge two clusters |

## Tools

### diagnose-consistency

Database integrity checker:

```bash
make diagnose-build
./diagnose-consistency -db path/to/cortex.sqlite [-workspace workspace-id]
```

### check_database.sh

Quick database overview:

```bash
../scripts/check_database.sh path/to/cortex.sqlite
```

Shows table counts, top tags, AI coverage, and embedding statistics.

## External Dependencies

| Service | Required | Purpose |
|---------|----------|---------|
| Ollama | Optional | Local LLM for AI features and embeddings |
| Apache Tika | Optional | Document metadata extraction (auto-downloaded) |
| Java | Optional | Required if Tika `manage_process` is enabled |
