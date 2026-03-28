# Cortex

A contextual understanding layer for your file system. Cortex indexes, extracts, and embeds every file in your workspace — code, documents, images, spreadsheets, PDFs — into a semantic vector space. AI agents can then query this space through MCP to understand your files, not just search them.

## The Problem

Your file system is a flat list of bytes. AI agents that need to work with your files can only do text search — they have no understanding of what a Word document is about, how a PDF relates to a spreadsheet, or what a folder structure represents.

## What Cortex Does

Cortex sits between your files and AI agents. It:

1. **Extracts** content from every file type — PDFs, DOCX, XLSX, images, code, audio, video
2. **Indexes** deep metadata — not just filenames, but content structure, code complexity, document authors, EXIF data, MIME types
3. **Embeds** everything into a vector space using local embedding models
4. **Locates** each file precisely in semantic space based on its full context — content, metadata, relationships, usage patterns
5. **Serves** this understanding to AI agents via gRPC (MCP-ready)

The result: an agent can ask "find the contracts related to the Q3 financial report" and get meaningful answers, even if those files share no keywords, live in different folders, and are in different formats.

```
Your Files (any type)
  │
  ▼
Cortex Pipeline
  ├── Extract: text from PDFs, Office docs, images (OCR), code analysis
  ├── Analyze: metadata, structure, relationships, entities
  ├── Embed: vector representations via local models
  ├── Classify: AI-generated tags, projects, categories, summaries
  └── Link: cross-document relationships, temporal co-occurrence
  │
  ▼
Semantic Vector Space (SQLite + embeddings)
  │
  ▼
gRPC API / MCP Server
  ├── Semantic search: "files about authentication"
  ├── RAG queries: "summarize the API contracts"
  ├── Contextual retrieval: related files, clusters, knowledge graph
  └── Structured metadata: tags, projects, states, relationships
```

## Architecture

| Component | Language | Role |
|-----------|----------|------|
| **Backend Daemon (`cortexd`)** | Go | Core engine — extraction, indexing, embedding, AI, storage |
| **VS Code Extension** | TypeScript | UI layer — browse the semantic space visually |

The daemon is the brain. It processes files through a multi-stage pipeline, stores everything in SQLite with vector embeddings, and exposes a gRPC API with 100+ methods across 8 services. The VS Code extension is one client; any MCP-compatible agent can be another.

## Quick Start

### Prerequisites

- [Go](https://go.dev/dl/) 1.22+
- [Node.js](https://nodejs.org/) 18+ (for the VS Code extension)
- [Ollama](https://ollama.com/) with `llama3.2` and `nomic-embed-text` models

### 1. Start the backend

```bash
cd backend
cp configs/cortexd.yaml.example cortexd.local.yaml
# Edit cortexd.local.yaml — set watch_paths to your workspace directory
make build
./cortexd --config cortexd.local.yaml
```

### 2. Pull the models

```bash
ollama pull llama3.2           # Language model for AI features
ollama pull nomic-embed-text   # Embedding model for vector space
```

### 3. (Optional) Run the VS Code extension

```bash
npm install
npm run compile
# Press F5 in VS Code to launch the Extension Development Host
```

The daemon works standalone — the VS Code extension is just one way to interact with it.

## How Indexing Works

Every file passes through a pipeline that extracts progressively deeper understanding:

| Stage | What It Extracts |
|-------|-----------------|
| `basic` | Size, timestamps, hashes (MD5, SHA-256), path structure |
| `mime` | True MIME type via magic bytes (not just extension) |
| `mirror` | Full text content from PDFs, DOCX, XLSX, PPTX, legacy Office formats |
| `code` | Lines of code, complexity, imports, exports, functions, classes |
| `document` | Markdown parsing, chunking, heading structure, word/page counts |
| `metadata` | Author, title, creation date, EXIF, IPTC, XMP, OS-level attributes |
| `relationship` | Cross-file references, imports, dependencies |
| `state` | Document lifecycle: draft, active, replaced, archived |
| `enrichment` | Named entities, sentiment, citations, tables, formulas |
| `embedding` | Vector representation via `nomic-embed-text` for semantic search |
| `ai` | LLM-generated tags, project assignments, summaries, categories |
| `clustering` | Semantic clusters via embedding similarity, temporal co-occurrence |
| `taxonomy` | AI-induced hierarchical category tree (Chain-of-Layer) |

After indexing, every file has a rich vector representation that captures its full context — not just its text content, but its metadata, relationships, and semantic meaning.

## What You Can Query

### Semantic Search (RAG)

Ask natural language questions and get answers with source citations:

```
"Which files discuss the payment integration?"
"Summarize the architecture decisions in this project"
"What contracts are related to the Q3 report?"
```

The RAG system retrieves relevant document chunks by embedding similarity, then generates an answer using the LLM with the retrieved context.

### Contextual Retrieval

- **Related files** — find files semantically similar to a given file
- **Document clusters** — auto-detected groups of related files
- **Knowledge graph** — relationships between documents (depends_on, replaces, references)
- **Usage patterns** — files frequently accessed or edited together
- **Faceted browsing** — filter by tag, project, type, date, size, category, metrics

### Structured Metadata

Every file has extractable structured data:

- Tags (manual + AI-generated)
- Project assignments (manual + AI-inferred)
- AI summaries
- Document state (draft/active/replaced/archived)
- Code metrics (LOC, complexity, functions)
- Document metrics (pages, words, author)
- Hierarchical categories (AI-generated taxonomy)

## gRPC API

The backend exposes 8 gRPC services (defined in [`backend/api/proto/cortex/v1/`](backend/api/proto/)):

| Service | Methods | Purpose |
|---------|---------|---------|
| `AdminService` | 16 | Daemon control, workspace management, pipeline streaming |
| `FileService` | 11 | Workspace scanning, file queries, grouped listings |
| `MetadataService` | 16 | Tags, projects, notes, AI summaries, suggestions |
| `LLMService` | 10 | AI operations — tag/project/summary/category generation |
| `RAGService` | 3 | Semantic search, RAG queries with citations, index stats |
| `KnowledgeService` | 34 | Projects, relationships, states, usage analytics, visualization |
| `TaxonomyService` | 15 | Hierarchical categories, AI-driven taxonomy induction |
| `ClusteringService` | 6 | Document clustering, similarity graph analysis |

See the [backend README](backend/README.md) for the full API reference with every method documented.

## MCP Server (AI Agent Interface)

Cortex can run as an [MCP](https://modelcontextprotocol.io/) server, giving AI coding agents (Claude Code, Copilot, Cursor) structured access to the knowledge graph. Instead of reading entire files, agents query documents, projects, and relationships with token-efficient tool calls.

```bash
cortexd --mcp --config cortexd.yaml
```

### Tools

| Tool | Purpose | Example |
|------|---------|---------|
| `cortex_find` | Search documents, projects, or files | `{"kind": "document", "state": "active", "tag": "review"}` |
| `cortex_show` | Inspect metadata, outline, or content | `{"target": "architecture.md", "view": "outline"}` |
| `cortex_relations` | Navigate the knowledge graph | `{"target": "api-spec.md", "type": "depends_on"}` |

### `cortex_find` — Search the knowledge graph

```json
// Find active documents tagged "review"
{"kind": "document", "state": "active", "tag": "review"}

// Find software projects
{"kind": "project", "nature": "development.software"}

// Semantic search (RAG-powered)
{"kind": "document", "query": "network VPN configuration"}

// Find PDF files
{"kind": "file", "extension": ".pdf", "limit": 10}
```

### `cortex_show` — Inspect a document or project

```json
// Document metadata (title, state, tags, projects, AI summary)
{"target": "meeting-notes.md", "view": "signature"}

// Document outline (heading structure)
{"target": "architecture.pdf", "view": "outline"}

// Project members (assigned documents)
{"target": "Nexus Platform", "view": "members"}

// Full file metadata (tags, contexts, language, AI category)
{"target": "invoice.xlsx", "view": "metadata"}
```

### `cortex_relations` — Navigate relationships

```json
// What does this document depend on?
{"target": "api-spec.md", "direction": "outgoing", "type": "depends_on"}

// What references this template?
{"target": "template.md", "direction": "incoming", "type": "references"}

// Find shortest path between two documents
{"target": "doc-a.md", "path_to": "doc-b.md"}
```

### Integration with Claude Code

Add to your Claude Code settings (`~/.claude/settings.json`):

```json
{
  "mcpServers": {
    "cortex": {
      "command": "/path/to/cortexd",
      "args": ["--mcp", "--config", "/path/to/cortexd.yaml"]
    }
  }
}
```

Then Claude Code can query your document knowledge graph directly:

> "What documents do I have about network configuration?" -> `cortex_find` with semantic search
>
> "Show me the outline of the architecture document" -> `cortex_show` with outline view
>
> "What depends on the API spec?" -> `cortex_relations` with depends_on traversal

## VS Code Extension

The extension provides a visual interface to browse the semantic space:

- **Faceted views** — by project, tag, type, date, size, folder, content type, metrics
- **Taxonomy tree** — AI-generated hierarchical categories
- **Admin dashboard** — backend status, pipeline progress, configuration
- **Cluster graph** — visual representation of document relationships
- **Semantic commands** — natural language file operations
- **RAG queries** — ask questions about your workspace from the editor

### Commands

| Command | Description |
|---------|-------------|
| `Cortex: Ask AI` | RAG query about your workspace |
| `Cortex: Execute Semantic Command` | Natural language file operation |
| `Cortex: Suggest Tags (AI)` | AI tag suggestions for current file |
| `Cortex: Suggest Project (AI)` | AI project suggestions for current file |
| `Cortex: Generate File Summary (AI)` | Generate AI summary |
| `Cortex: Add tag to current file` | Manual tag assignment |
| `Cortex: Assign project to current file` | Manual project assignment |
| `Re-index Everything` | Trigger full re-indexing |
| `Backend Admin` | Open admin dashboard |
| `Pipeline Progress` | Real-time indexing progress |

## Configuration

### Daemon (YAML)

See [`backend/configs/cortexd.yaml.example`](backend/configs/cortexd.yaml.example) for the full reference.

```yaml
grpc_address: "localhost:50051"
data_dir: "./cortex-data"
worker_count: 4

watch_paths:
  - "/path/to/your/workspace"

llm:
  enabled: true
  default_provider: "ollama"
  default_model: "llama3.2"
  embeddings:
    enabled: true
    model: "nomic-embed-text"

tika:
  enabled: true          # Apache Tika for deep document extraction
  auto_download: true    # Downloads Tika JAR automatically
```

### Extension (VS Code Settings)

```jsonc
"cortex.grpc.address": "127.0.0.1:50051",
"cortex.llm.endpoint": "http://localhost:11434",
"cortex.llm.model": "llama3.2"
```

## Project Structure

```
cortex/
├── backend/                       # Go daemon (the core engine)
│   ├── cmd/cortexd/               # Entry point
│   ├── api/proto/                 # gRPC service definitions (8 services)
│   ├── internal/
│   │   ├── application/           # Pipeline, services, business logic
│   │   ├── domain/                # Entities, repository interfaces
│   │   ├── infrastructure/        # SQLite, LLM providers, embeddings, file system
│   │   └── interfaces/grpc/       # gRPC handlers and adapters
│   └── Makefile
├── src/                           # VS Code extension (TypeScript)
│   ├── extension.ts               # Entry point
│   ├── core/                      # gRPC clients
│   ├── views/                     # Facet-based tree providers
│   ├── frontend/                  # WebView panels
│   ├── commands/                  # Command handlers
│   └── services/                  # Extension services
├── docker-compose.yml             # Apache Tika (document extraction)
├── docker-compose.onlyoffice.yml  # OnlyOffice (document editing)
└── LICENSE                        # MIT
```

## Development

```bash
# Backend
cd backend
make build           # Build cortexd
make run             # Build and run
make test            # Run tests
make proto           # Regenerate gRPC code

# Extension
npm install
npm run compile
npm run watch        # Watch mode
npm test
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full development guide.

## Roadmap

- **MCP Server** — expose Cortex as a Model Context Protocol server for any AI agent
- **Image understanding** — extract visual content, not just EXIF metadata
- **Multi-workspace** — index across multiple directories and projects
- **Graph visualization** — interactive knowledge graph in the browser
- **Plugin system** — custom extractors and pipeline stages

## License

[MIT](LICENSE)
