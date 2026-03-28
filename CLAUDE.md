# Cortex - Claude Code Development Guide

## Project Overview

**Cortex** is a VS Code extension that provides a semantic cognition layer for workspace file organization. It allows users to organize files by projects, tags, types, and other attributes without moving files or creating folders.

**Architecture**: Client-server model. The VS Code extension is a **frontend-only** client that communicates with a **Go backend daemon (`cortexd`)** via gRPC. No local indexing, classification, or detection - all data comes from the backend.

## Project Structure

```
cortex/
├── src/
│   ├── extension.ts              # Main entry point (frontend-only, gRPC clients)
│   ├── core/                     # gRPC clients and backend integration
│   │   ├── GrpcAdminClient.ts    # Status, config, workspace management
│   │   ├── GrpcMetadataClient.ts # File metadata (tags, projects, notes)
│   │   ├── GrpcRAGClient.ts      # Retrieval-augmented generation queries
│   │   ├── GrpcKnowledgeClient.ts # Projects, entities, relationships
│   │   ├── GrpcLLMClient.ts      # LLM provider management
│   │   ├── GrpcPreferencesClient.ts # User preferences
│   │   ├── GrpcTaxonomyClient.ts # Taxonomy/category management
│   │   ├── GrpcClusteringClient.ts # Semantic clustering
│   │   ├── GrpcEntityClient.ts   # Unified entity model
│   │   ├── GrpcSFSClient.ts      # Semantic file system commands
│   │   ├── BackendManager.ts     # Backend daemon lifecycle
│   │   ├── BackendMetadataStore.ts # IMetadataStore via gRPC
│   │   ├── FileCacheService.ts   # Client-side cache (30s TTL)
│   │   └── IMetadataStore.ts     # Storage interface abstraction
│   ├── views/                    # Facet-based tree providers
│   │   ├── CortexTreeProvider.ts       # Hierarchical section-based tree
│   │   ├── UnifiedFacetTreeProvider.ts # Main unified facet provider
│   │   ├── TaxonomyTreeProvider.ts     # Category/taxonomy tree
│   │   ├── TermsFacetTreeProvider.ts   # Tags, projects (string facets)
│   │   ├── DateRangeFacetTreeProvider.ts # Date range faceting
│   │   ├── NumericRangeFacetTreeProvider.ts # Size/metrics ranges
│   │   ├── CategoryFacetTreeProvider.ts # Category grouping
│   │   ├── ClusterFacetTreeProvider.ts  # Semantic clusters
│   │   ├── MetricsFacetTreeProvider.ts  # Code/doc metrics
│   │   ├── FolderTreeProvider.ts        # Folder hierarchy
│   │   ├── FileInfoTreeProvider.ts      # File details panel
│   │   ├── base/                        # Base classes for facet providers
│   │   ├── contracts/                   # Interfaces (IFacetProvider)
│   │   └── i18n.ts                      # Internationalization
│   ├── frontend/                 # WebView-based UI panels
│   │   ├── BackendFrontend.ts    # Backend admin dashboard
│   │   ├── MetricsDashboard.ts   # Metrics visualization
│   │   ├── PipelineProgressView.ts # Real-time indexing progress
│   │   ├── FileInfoWebview.ts    # Rich file info panel
│   │   ├── OnlyOfficeWordEditor.ts # Document editor integration
│   │   ├── ClusterGraphWebview.ts # Cluster visualization
│   │   └── SFSCommandInputWebview.ts # Semantic command interface
│   ├── commands/                 # VS Code command implementations
│   │   ├── addTag.ts            # Add tags to files
│   │   ├── assignContext.ts     # Assign projects to files
│   │   ├── assignProject.ts    # Project assignment
│   │   ├── createProject.ts    # Create new project
│   │   ├── askAI.ts            # RAG queries about workspace
│   │   ├── acceptSuggestion.ts # Accept AI suggestions
│   │   ├── suggestTagsAI.ts    # AI tag suggestions
│   │   ├── suggestProjectAI.ts # AI project suggestions
│   │   ├── generateSummaryAI.ts # AI file summarization
│   │   ├── executeSemanticCommand.ts # Natural language file ops
│   │   ├── openView.ts         # Focus Cortex sidebar
│   │   ├── rebuildIndex.ts     # Trigger backend re-index
│   │   ├── openBackendFrontend.ts # Show backend dashboard
│   │   ├── openMetricsDashboard.ts # Show metrics
│   │   ├── openPipelineProgress.ts # Show pipeline progress
│   │   ├── openWordEditor.ts   # Open OnlyOffice editor
│   │   ├── startOnlyOffice.ts  # Start OnlyOffice service
│   │   └── copyTreeItemText.ts # Copy tree item to clipboard
│   ├── services/
│   │   ├── AIQualityService.ts         # LLM validation for operations
│   │   ├── IndexingProgressService.ts  # Real-time progress tracking
│   │   ├── ProjectInferenceService.ts  # AI project inference
│   │   ├── RealtimeUpdateService.ts    # Backend event streaming
│   │   └── OnlyOfficeBridge.ts         # OnlyOffice integration
│   ├── utils/
│   │   ├── osTags.ts            # OS-level tag integration (macOS Finder)
│   │   ├── saveAISummary.ts     # Persist AI summaries
│   │   ├── dateUtils.ts         # Date formatting
│   │   ├── fileActivity.ts      # File activity tracking
│   │   ├── llmParsers.ts        # LLM response parsing
│   │   ├── projectNature.ts     # Project type classification
│   │   ├── projectVisualization.ts # Project visual attributes
│   │   └── sizeUtils.ts         # File size formatting
│   ├── models/
│   │   ├── types.ts             # TypeScript type definitions
│   │   └── entity.ts            # Unified entity model
│   ├── types/
│   │   └── sqlite-vec.d.ts      # Type definitions
│   └── test/                    # Test suite
│       ├── suite/               # Unit and integration tests
│       └── helpers/             # Test utilities
├── backend/                     # Go backend daemon
│   ├── cmd/
│   │   ├── cortexd/             # Main daemon entry point
│   │   ├── migrate-contexts/    # Data migration tool
│   │   └── diagnose-consistency/ # Database diagnostic tool
│   ├── api/
│   │   ├── proto/               # Protocol buffer definitions
│   │   └── gen/                 # Generated gRPC code
│   ├── internal/
│   │   ├── application/         # Business logic & pipeline
│   │   ├── domain/              # Domain models
│   │   ├── infrastructure/      # Database, storage
│   │   └── interfaces/          # gRPC service implementations
│   ├── pkg/                     # Shared packages
│   ├── plugins/                 # Plugin system
│   ├── go.mod / go.sum          # Dependencies
│   └── Makefile                 # Build targets
├── docs/                        # Documentation
├── scripts/                     # Utility scripts
├── docker-compose.yml           # Apache Tika service
├── docker-compose.onlyoffice.yml # OnlyOffice DocumentServer
├── package.json                 # Extension manifest
├── tsconfig.json                # TypeScript configuration
└── README.md                    # User documentation
```

## Key Architectural Components

### 1. Backend Daemon (cortexd)

The Go backend handles all heavy processing:
- **Multi-stage pipeline**: basic → mime → mirror → code → document → relationship → state → AI
- **gRPC API**: 10+ services exposed
- **SQLite storage**: File metadata, relationships, embeddings
- **LLM integration**: Ollama, LM Studio, OpenAI-compatible
- **RAG**: Vector embeddings for semantic search

Build: `cd backend && make build`
Run: `cd backend && make run`

### 2. gRPC Client Layer

The VS Code extension communicates with the backend via typed gRPC clients:
- **GrpcAdminClient**: Health, status, config, workspace management
- **GrpcMetadataClient**: Tags, projects, notes, summaries
- **GrpcLLMClient**: AI suggestions, summaries, completions
- **GrpcRAGClient**: Semantic search and Q&A
- **GrpcKnowledgeClient**: Projects, entities, relationships
- **GrpcTaxonomyClient**: Categories and taxonomy
- **GrpcClusteringClient**: Semantic file clustering
- **GrpcEntityClient**: Unified file/folder/project entities

### 3. Facet-Based View System

Replaced individual tree providers with a unified facet architecture:
- **UnifiedFacetTreeProvider**: Renders all facets from FacetRegistry
- **BaseFacetTreeProvider**: Abstract base with caching and refresh
- **Facet types**: Terms, NumericRange, DateRange, Structure, Category, Metrics
- **CortexTreeProvider**: Hierarchical section-based container

### 4. Frontend WebViews

Rich HTML panels for advanced features:
- **BackendFrontend**: Admin dashboard (status, config, logs)
- **MetricsDashboard**: File metrics visualization
- **PipelineProgressView**: Real-time indexing progress
- **ClusterGraphWebview**: Relationship visualization
- **OnlyOfficeWordEditor**: Document editing

### 5. Services

- **RealtimeUpdateService**: Streams pipeline events from backend, triggers UI refresh
- **IndexingProgressService**: Progress bar and status tracking
- **AIQualityService**: Validates LLM outputs before applying
- **ProjectInferenceService**: AI-driven project attribute inference
- **OnlyOfficeBridge**: OnlyOffice DocumentServer integration

## Configuration

### gRPC Settings
- `cortex.grpc.address`: Backend gRPC address (default: `127.0.0.1:50051`)
- `cortex.autoStartBackend`: Auto-start backend daemon (default: true)

### AI/LLM Settings
- `cortex.llm.enabled`: Enable AI features
- `cortex.llm.endpoint`: LLM API endpoint (default: `http://localhost:11434`)
- `cortex.llm.model`: Model name (default: `llama3.2`)

### OnlyOffice Settings
- `cortex.onlyoffice.enabled`: Enable document editing
- `cortex.onlyoffice.documentServerUrl`: OnlyOffice server URL

## Development Workflow

### Build & Run
```bash
npm install           # Install TS dependencies
npm run compile       # Compile TypeScript
npm run watch         # Watch mode
F5                    # Launch Extension Development Host

# Backend
cd backend
make build            # Build cortexd
make run              # Build and run
make proto            # Regenerate protobuf code
make test             # Run Go tests
```

### Testing
```bash
npm run test          # Run VS Code extension tests
npm run lint          # Run ESLint
cd backend && make test  # Run Go backend tests
```

## Dependencies

### Runtime (TypeScript)
- **@grpc/grpc-js** - gRPC client
- **@grpc/proto-loader** - Protocol buffer loading
- **better-sqlite3** - SQLite (client-side cache)
- **sqlite-vec** - Vector embeddings
- **fs-extra** - File system utilities

### Runtime (Go Backend)
- **google.golang.org/grpc** - gRPC server
- **google.golang.org/protobuf** - Protocol buffers
- **modernc.org/sqlite** - SQLite driver
- **github.com/tmc/langchaingo** - LangChain Go
- **github.com/fsnotify/fsnotify** - File watching

### External Services
- **Ollama** - Local LLM runtime (for AI features)
- **Apache Tika** - Document extraction (via Docker)
- **OnlyOffice** - Document editing (via Docker)

## Key Design Decisions

| Decision | Rationale |
|----------|-----------|
| **Backend-first** | Heavy processing offloaded to Go daemon for performance |
| **gRPC over REST** | Typed contracts, streaming, efficient serialization |
| **Facet-based views** | Unified, extensible view system vs. individual providers |
| **Unified entity model** | Files, folders, projects share same facet interface |
| **Local-first AI** | Privacy-preserving, works offline, user controls model |
| **No file moving** | Non-invasive, respects existing project structure |
| **Pipeline stages** | Modular processing (13 stages) for incremental indexing |

## Common Tasks for Claude

### Adding a new gRPC client
1. Define service in `backend/api/proto/`
2. Run `make proto` to generate Go code
3. Create client in `src/core/GrpcNewClient.ts`
4. Initialize in `extension.ts` activation
5. Add to ExtensionState interface

### Adding a new facet view
1. Create provider extending `BaseFacetTreeProvider`
2. Implement `IFacetProvider` from `src/views/contracts/`
3. Register in facet registry
4. Add view contribution to `package.json`

### Adding a new command
1. Create command file in `src/commands/`
2. Register in `extension.ts`
3. Add command contribution to `package.json`

### Adding a new pipeline stage (backend)
1. Implement stage in `backend/internal/application/pipeline/`
2. Register in pipeline configuration
3. Add streaming events for frontend progress tracking
