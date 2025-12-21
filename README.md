# Cortex - Semantic File Organization for VS Code

**A semantic cognition layer on top of your filesystem.**

Cortex lets you organize your workspace files by **projects**, **tags**, and **file types** without moving files or creating folders. Think of it like Windows Media Player organizing MP3s by Artist, Album, and Genre — but for code, documents, and all your work files.

## Core Principle

- **Files stay where they are** - No duplication, no moving
- **Multiple virtual views** - See the same files organized different ways
- **Files belong to multiple projects** - A file can be tagged "urgent" and belong to "book-draft" and "client-acme" simultaneously

## Features

### 1. Virtual Semantic Views

Three tree views in the Activity Bar:

- **By Project** - Group files by projects, clients, cases, or any semantic container
- **By Tag** - Filter files by tags like "urgent", "review", "bug-fix"
- **By Type** - Browse by file type (typescript, pdf, markdown, etc.)

### 2. Quick Commands

- `Cortex: Add tag to current file` - Tag the active file
- `Cortex: Assign project to current file` - Add file to a project
- `Cortex: Open Cortex View` - Focus the Cortex sidebar
- `Cortex: Rebuild Index` - Rescan the workspace

### 3. Local-First Architecture

- All metadata stored in `.cortex/index.sqlite` (falls back to `.cortex/index.json` if SQLite is unavailable)
- No cloud, no AI, no required external dependencies
- Fast startup and indexing
- Deterministic behavior

## Installation

1. Clone this repository
2. Install dependencies:
   ```bash
   npm install
   ```

3. Compile TypeScript:
   ```bash
   npm run compile
   ```

4. Press `F5` in VS Code to launch Extension Development Host

## Troubleshooting

If you see a message about missing `better-sqlite3` bindings, rebuild the native module:

```bash
npm rebuild better-sqlite3
```

## Usage

### Basic Workflow

1. **Open your workspace** - Cortex will automatically index all files
2. **Add projects** - Right-click a file → "Cortex: Assign project to current file"
3. **Add tags** - Right-click a file → "Cortex: Add tag to current file"
4. **Browse views** - Click the Cortex icon in the Activity Bar

### Example Use Cases

#### Organizing a Client Project
```
Project: "client-acme"
Files:
  - contracts/acme-contract.pdf
  - src/acme-integration.ts
  - emails/acme-kickoff.eml
  - designs/acme-mockups.fig
```

#### Tracking Code Reviews
```
Tag: "needs-review"
Files:
  - src/auth/login.ts
  - src/api/endpoints.ts
  - tests/auth.test.ts
```

#### Finding All TypeScript Files
```
Type: "typescript"
Files:
  - (all .ts and .tsx files)
```

## Architecture

### Directory Structure

```
cortex/
├── src/
│   ├── extension.ts              # Entry point, orchestrates everything
│   ├── core/
│   │   ├── FileScanner.ts        # Scans workspace, builds file list
│   │   ├── IndexStore.ts         # In-memory index (fast lookups)
│   │   └── MetadataStore.ts      # SQLite persistence (tags, projects)
│   ├── views/
│   │   ├── ContextTreeProvider.ts # "By Project" view
│   │   ├── TagTreeProvider.ts     # "By Tag" view
│   │   └── TypeTreeProvider.ts    # "By Type" view
│   ├── commands/
│   │   ├── addTag.ts
│   │   ├── assignContext.ts      # Assign project (stored as context)
│   │   ├── openView.ts
│   │   └── rebuildIndex.ts
│   ├── models/
│   │   └── types.ts               # TypeScript interfaces
│   └── utils/
│       └── fileHash.ts            # File ID generation
├── package.json
├── tsconfig.json
└── README.md
```

### Data Model

#### FileIndexEntry (In-Memory)
```typescript
{
  absolutePath: string;
  relativePath: string;
  filename: string;
  extension: string;
  lastModified: number;
  fileSize: number;
}
```

#### FileMetadata (Persisted in SQLite)
```typescript
{
  file_id: string;           // SHA-256 hash of relative path
  relativePath: string;
  tags: string[];
  contexts: string[];     // Projects
  type: string;              // Inferred from extension
  notes?: string;
  created_at: number;
  updated_at: number;
}
```

### Database Schema

**file_metadata**
- `file_id` (PRIMARY KEY) - Stable hash of relative path
- `relative_path` (UNIQUE) - Workspace-relative path
- `type` - Inferred file type
- `notes` - Optional user notes
- `created_at`, `updated_at` - Timestamps

**file_tags** (Many-to-Many)
- `file_id`, `tag` (COMPOSITE PRIMARY KEY)

**file_contexts** (Many-to-Many Projects)
- `file_id`, `context` (COMPOSITE PRIMARY KEY)

## Key Architectural Decisions

| Decision | Rationale |
|----------|-----------|
| **SQLite over JSON** | Better query performance, atomic updates, handles 10,000+ files |
| **Stable file_id (hash)** | Survives renames if we track them later; deterministic |
| **Separate IndexStore and MetadataStore** | Index = workspace state (fast), Metadata = user semantics (persistent) |
| **TreeDataProvider pattern** | Native VS Code API, efficient, familiar UX |
| **Ignored directories** | `.git`, `node_modules`, `.vscode`, `.cortex` - standard exclusions |
| **Incremental updates** | File watcher updates index on create/delete without full rescan |

## Extension Points (Future)

While MVP is local-first and manual, the architecture supports:

- **Smart indexing** - Index file contents for search
- **Relationship graphs** - Track which files import/reference each other
- **Time-based views** - Group by "last week", "this month"
- **Saved searches** - Persist complex queries
- **Sync (optional)** - Share `.cortex/` via Git or cloud

## Development

### Build
```bash
npm run compile
```

### Watch Mode
```bash
npm run watch
```

### Lint
```bash
npm run lint
```

### Package Extension
```bash
vsce package
```

## Dependencies

- **better-sqlite3** - Fast, synchronous SQLite library
- **VS Code Extension API** - ^1.80.0
- **LibreOffice (soffice)** - Optional, needed to extract legacy Office metadata (.doc/.xls/.ppt)

## License

MIT

---

**Cortex** - Your files, semantically organized.
