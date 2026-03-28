# Cortex

A semantic cognition layer for your workspace. Cortex is a VS Code extension backed by a Go daemon that lets you organize files by projects, tags, and categories — without moving a single file.

## What It Does

Cortex adds a metadata layer on top of your file system. Instead of reorganizing folders, you assign semantic attributes (projects, tags, categories) to files. Multiple virtual views then present the same files grouped in different ways.

**Key principles:**

- Files stay where they are — no copies, no moves
- One file can belong to multiple projects and have multiple tags
- All processing happens locally — your data never leaves your machine
- AI features are optional and powered by local LLMs (Ollama)

## Architecture

Cortex has two components:

| Component | Language | Role |
|-----------|----------|------|
| **VS Code Extension** | TypeScript | UI layer — tree views, commands, webview panels |
| **Backend Daemon (`cortexd`)** | Go | All processing — indexing, AI, storage, search |

They communicate over gRPC on `127.0.0.1:50051`. The extension is a thin client; all heavy work happens in the daemon.

```
VS Code Extension (TypeScript)
  │
  │  gRPC (localhost:50051)
  │
Backend Daemon (Go)
  ├── Pipeline: basic → mime → mirror → code → document → relationship → state → AI
  ├── SQLite database (metadata, embeddings, relationships)
  ├── LLM integration (Ollama, LM Studio, OpenAI-compatible)
  └── File system watcher (real-time updates)
```

## Quick Start

### Prerequisites

- [Go](https://go.dev/dl/) 1.22+
- [Node.js](https://nodejs.org/) 18+
- [VS Code](https://code.visualstudio.com/) 1.80+
- [Ollama](https://ollama.com/) (optional, for AI features)

### 1. Start the backend

```bash
cd backend
cp configs/cortexd.yaml.example cortexd.local.yaml
# Edit cortexd.local.yaml — set watch_paths to your workspace directory
make build
./cortexd --config cortexd.local.yaml
```

The daemon starts a gRPC server on `localhost:50051` and begins indexing files in the configured watch paths.

### 2. Run the extension

```bash
npm install
npm run compile
```

Open the project in VS Code and press `F5` to launch the Extension Development Host. Open any workspace — the extension connects to the running daemon automatically.

### 3. (Optional) Enable AI features

```bash
ollama serve
ollama pull llama3.2           # Language model
ollama pull nomic-embed-text   # Embedding model for RAG
```

AI features are enabled by default in the daemon config. If Ollama is not running, the daemon skips AI stages gracefully.

## Features

### Virtual Views

Cortex provides faceted views in the VS Code sidebar:

- **By Project** — files grouped by assigned projects
- **By Tag** — files grouped by tags
- **By Type** — files grouped by extension / MIME type
- **By Date** — files grouped by modification date ranges
- **By Size** — files grouped by size ranges
- **By Folder** — standard folder hierarchy
- **By Content Type** — files grouped by detected MIME type
- **Code Metrics** — lines of code, complexity, functions
- **Document Metrics** — page count, word count, author
- **Taxonomy** — AI-generated hierarchical categories

### AI-Powered Organization

When Ollama is running, Cortex automatically:

- **Suggests tags** based on file content
- **Suggests projects** based on context and related files
- **Generates summaries** for documents
- **Classifies files** into a dynamically generated taxonomy
- **Finds related files** using semantic similarity (RAG)

You can also trigger these manually via the Command Palette.

### Semantic Search (RAG)

Ask natural language questions about your workspace:

```
Command Palette → "Cortex: Ask AI"
> "Which files discuss authentication?"
> "Summarize the API contracts in this project"
```

The RAG system chunks documents, generates embeddings with `nomic-embed-text`, and retrieves relevant context before answering with the LLM.

### Semantic File System (SFS)

Natural language commands for file organization:

```
Command Palette → "Cortex: Execute Semantic Command"
> "tag all PDFs as documentation"
> "assign files in src/auth to project authentication"
> "find all files modified today"
```

### Document Clustering

Cortex groups related files into clusters using:

- Semantic similarity (embedding distance)
- Temporal co-occurrence (files edited together)
- Structural proximity (shared folder paths)
- Entity overlap (shared named entities)

### Knowledge Engine

Beyond simple tags, Cortex tracks:

- **Document states** — draft, active, replaced, archived
- **Relationships** — replaces, depends_on, references, parent_of
- **Project hierarchy** — projects with sub-projects
- **Usage analytics** — open frequency, co-occurrence patterns

### Pipeline

The backend processes each file through these stages:

| Stage | What It Does |
|-------|-------------|
| `basic` | File size, timestamps, hashes (MD5, SHA-256) |
| `mime` | MIME type detection via magic bytes |
| `mirror` | Text extraction from PDFs, DOCX, XLSX, PPTX |
| `code` | Lines of code, complexity, imports, exports |
| `document` | Markdown parsing, chunking for RAG |
| `relationship` | Cross-file reference detection |
| `state` | Document lifecycle state inference |
| `ai` | Tag/project/summary/category generation via LLM |

Additional stages run for enrichment, clustering, taxonomy induction, and project inference.

## Commands

All commands are available via `Cmd+Shift+P` / `Ctrl+Shift+P`:

| Command | Description |
|---------|-------------|
| `Cortex: Add tag to current file` | Assign a tag to the active file |
| `Cortex: Assign project to current file` | Assign a project to the active file |
| `Cortex: Create New Project` | Create a new project |
| `Cortex: Suggest Tags (AI)` | Get AI-generated tag suggestions |
| `Cortex: Suggest Project (AI)` | Get AI-generated project suggestions |
| `Cortex: Generate File Summary (AI)` | Generate an AI summary of the file |
| `Cortex: Ask AI` | Ask a question about your workspace (RAG) |
| `Cortex: Execute Semantic Command` | Run a natural language file operation |
| `Cortex: Open Cortex View` | Focus the Cortex sidebar |
| `Re-index Everything` | Trigger a full workspace rescan |
| `Backend Admin` | Open the backend admin dashboard |
| `Pipeline Progress` | View real-time indexing progress |

## Configuration

### Extension Settings (VS Code)

Open VS Code Settings (`Cmd+,`) and search for "cortex":

```jsonc
// gRPC connection
"cortex.grpc.address": "127.0.0.1:50051",
"cortex.autoStartBackend": true,

// LLM settings
"cortex.llm.endpoint": "http://localhost:11434",
"cortex.llm.model": "llama3.2",
"cortex.llm.maxContextTokens": 2000,

// Auto-indexing
"cortex.llm.autoSummary.enabled": true,
"cortex.llm.autoIndex.enabled": true,
"cortex.llm.autoIndex.applyTags": true,
"cortex.llm.autoIndex.applyProjects": true,

// RAG
"cortex.rag.similarityThreshold": 0.5,
"cortex.rag.maxSuggestions": 10
```

### Daemon Configuration (YAML)

The daemon reads from a YAML config file. See [`backend/configs/cortexd.yaml.example`](backend/configs/cortexd.yaml.example) for the full reference.

Key sections:

```yaml
grpc_address: "localhost:50051"
data_dir: "./cortex-data"
worker_count: 4
log_level: "info"

llm:
  enabled: true
  default_provider: "ollama"
  default_model: "llama3.2"
  embeddings:
    enabled: true
    model: "nomic-embed-text"

tika:
  enabled: true          # Apache Tika for document extraction
  auto_download: true    # Downloads Tika JAR automatically
```

## gRPC API

The backend exposes these gRPC services (defined in [`backend/api/proto/cortex/v1/`](backend/api/proto/)):

| Service | Methods | Purpose |
|---------|---------|---------|
| `AdminService` | 16 | Daemon control, workspace management, health checks, pipeline streaming |
| `FileService` | 11 | Workspace scanning, file queries, grouping operations |
| `MetadataService` | 16 | Tags, projects, notes, AI summaries, suggestions |
| `LLMService` | 10 | Provider management, AI operations (tags, projects, summaries) |
| `RAGService` | 3 | Semantic search, RAG queries, index statistics |
| `KnowledgeService` | 34 | Projects, relationships, states, usage, visualization |
| `TaxonomyService` | 15 | Hierarchical categories, AI-driven taxonomy induction |
| `ClusteringService` | 6 | Document clustering, graph analysis |

See the [backend README](backend/README.md) for detailed API documentation.

## Project Structure

```
cortex/
├── src/                           # VS Code extension (TypeScript)
│   ├── extension.ts               # Entry point
│   ├── core/                      # gRPC clients (Admin, Metadata, RAG, LLM, ...)
│   ├── views/                     # Facet-based tree providers
│   ├── frontend/                  # WebView panels (dashboard, metrics, editor)
│   ├── commands/                  # VS Code command handlers
│   ├── services/                  # Extension services (progress, realtime, AI quality)
│   ├── models/                    # TypeScript type definitions
│   └── test/                      # Tests
├── backend/                       # Go daemon
│   ├── cmd/cortexd/               # Main entry point
│   ├── api/proto/                 # Protocol Buffer definitions
│   ├── internal/
│   │   ├── application/           # Business logic, pipeline
│   │   ├── domain/                # Domain models, repository interfaces
│   │   ├── infrastructure/        # SQLite, LLM providers, file system
│   │   └── interfaces/grpc/       # gRPC handlers and adapters
│   └── Makefile
├── docs/                          # Documentation
├── docker-compose.yml             # Apache Tika service
├── docker-compose.onlyoffice.yml  # OnlyOffice document editor
├── package.json                   # Extension manifest
└── LICENSE                        # MIT
```

## Development

### Extension

```bash
npm install          # Install dependencies
npm run compile      # Build once
npm run watch        # Build on file changes
npm test             # Run tests
npm run lint         # Lint
```

Press `F5` in VS Code to launch the Extension Development Host with the debugger attached.

### Backend

```bash
cd backend
make build           # Build the cortexd binary
make run             # Build and run
make test            # Run all tests
make proto           # Regenerate gRPC code from .proto files
```

See [`backend/README.md`](backend/README.md) for full backend development docs.

### Docker Services

```bash
# Apache Tika (document metadata extraction)
docker compose up -d

# OnlyOffice (optional, for document editing)
docker compose -f docker-compose.onlyoffice.yml up -d
```

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines on how to contribute.

## License

[MIT](LICENSE)
