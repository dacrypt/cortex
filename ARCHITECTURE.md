# Cortex Architecture Deep Dive

## Design Philosophy

Cortex is built on the principle of **separation without modification**. Files remain in their physical locations while Cortex provides semantic organization layers on top.

### Mental Model: iTunes for Work Files

Just as iTunes organized music files by:
- Artist
- Album
- Genre
- Year

...without moving the MP3 files from their original locations, Cortex organizes work files by:
- **Project** (book, client, case)
- **Tag** (status, category, priority)
- **Type** (code, document, image)

## Three-Layer Architecture

```
┌─────────────────────────────────────────────┐
│           VIEW LAYER                        │
│  (TreeDataProviders - Virtual Hierarchies)  │
│                                             │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐   │
│  │ Project  │ │   Tag    │ │   Type   │   │
│  │   View   │ │   View   │ │   View   │   │
│  └──────────┘ └──────────┘ └──────────┘   │
└─────────────────────────────────────────────┘
                    ▲
                    │ queries
                    │
┌─────────────────────────────────────────────┐
│         METADATA LAYER                      │
│    (MetadataStore - User Semantics)         │
│                                             │
│  SQLite Database (.cortex/index.sqlite)    │
│  - Tags (many-to-many)                     │
│  - Projects (stored as contexts, many-to-many) │
│  - Notes                                   │
│  - Timestamps                              │
└─────────────────────────────────────────────┘
                    ▲
                    │ references
                    │
┌─────────────────────────────────────────────┐
│         INDEXING LAYER                      │
│   (FileScanner + IndexStore - Fast Index)   │
│                                             │
│  In-Memory Map                             │
│  - All workspace files                     │
│  - Paths, sizes, timestamps                │
│  - Quick lookups                           │
└─────────────────────────────────────────────┘
                    ▲
                    │ scans
                    │
┌─────────────────────────────────────────────┐
│         FILESYSTEM                          │
│   (Real files in workspace)                 │
└─────────────────────────────────────────────┘
```

## Core Components

### 1. FileScanner

**Responsibility**: Discover all files in the workspace

**Key Features**:
- Recursive directory traversal
- Ignores standard directories (`.git`, `node_modules`, etc.)
- Extracts file metadata (size, modified time, extension)
- Async/await for non-blocking scans

**Why not use VS Code's workspace API?**
- Full control over ignore patterns
- Consistent behavior across VS Code versions
- Can extend to track file content hashes later

```typescript
// Example output:
{
  absolutePath: "/workspace/src/index.ts",
  relativePath: "src/index.ts",
  filename: "index.ts",
  extension: ".ts",
  lastModified: 1704067200000,
  fileSize: 2048
}
```

### 2. IndexStore

**Responsibility**: Fast in-memory index of all files

**Key Features**:
- Map-based storage (O(1) lookups)
- Keyed by `relativePath` (stable across sessions)
- Provides query methods (by extension, search, stats)
- Rebuilt on activation, updated incrementally via file watcher

**Why in-memory?**
- Speed: 10,000 files indexed in <100ms
- Simplicity: No disk I/O for basic lookups
- Disposable: Can rebuild quickly from filesystem

**Trade-off**: Memory usage scales linearly with file count. Acceptable for typical workspaces (<50k files).

### 3. MetadataStore

**Responsibility**: Persist user-defined semantics

**Key Features**:
- SQLite database for reliability
- Normalized schema (file_metadata, file_tags, file_contexts for projects)
- Stable `file_id` via SHA-256 hash of relative path
- Indexed for fast queries

**Why SQLite?**
1. **Performance**: Indexed queries on 10k+ files
2. **Atomicity**: ACID guarantees prevent corruption
3. **Simplicity**: Single-file database
4. **Portability**: Can be committed to Git for team sharing

**Schema Design**:
- **file_metadata**: Core file info (1:1)
- **file_tags**: Many-to-many relationship
- **file_contexts**: Many-to-many relationship for projects

This allows:
- A file to have multiple tags
- A tag to apply to multiple files
- Efficient queries: "Get all files with tag X"

**Alternative Considered: JSON**
- Pros: No binary dependency, easier to inspect
- Cons: O(n) queries, no transactions, fragile updates
- Decision: SQLite wins for scalability

### 4. TreeDataProviders

**Responsibility**: Render virtual hierarchies in UI

**Key Features**:
- Implements VS Code's `TreeDataProvider` interface
- Lazy loading (only expand when user clicks)
- Refresh on metadata changes
- Clickable file items (open in editor)

**Three Providers**:

#### ContextTreeProvider (Project View)
```
project-alpha (15 files)
├── design-doc.pdf
├── src/main.ts
└── notes/meeting.md

client-acme (8 files)
├── contract.pdf
└── invoice.xlsx
```

#### TagTreeProvider
```
urgent (5 files)
├── bug-report.md
└── fix.ts

review (12 files)
├── pr-123.patch
└── code.ts
```

#### TypeTreeProvider
```
typescript (250 files)
├── src/index.ts
└── tests/unit.test.ts

pdf (12 files)
├── contracts/acme.pdf
└── invoices/jan.pdf
```

**Why separate providers?**
- Each view has different grouping logic
- Independent refresh cycles
- Clear separation of concerns

## File Identity: The `file_id` Problem

**Challenge**: How do we uniquely identify a file that might be renamed or moved?

**Solution**: SHA-256 hash of relative path

```typescript
file_id = sha256("src/components/Button.tsx")
// => "a3c8f9e2..."
```

**Trade-offs**:

| Approach | Pros | Cons |
|----------|------|------|
| Absolute path | Simple | Breaks on workspace move |
| Relative path | Stable in workspace | Long string, not unique |
| Hash of path | Short, deterministic | Breaks on rename (acceptable for MVP) |
| UUID | Truly stable | Requires tracking renames |

**Decision**: Use hash for MVP. Future enhancement: track renames via file watcher.

## Data Flow

### Activation Sequence
```
1. Extension activates
2. MetadataStore initializes SQLite
3. FileScanner scans workspace
4. IndexStore builds in-memory index
5. MetadataStore ensures all files have entries
6. TreeProviders render initial views
```

### Adding a Tag
```
1. User invokes "Add tag" command
2. Command validates active file
3. Shows input box
4. Calls MetadataStore.addTag()
5. SQLite: INSERT INTO file_tags
6. Command triggers refreshAllViews()
7. TreeProviders re-query and refresh
```

### File Created (Incremental Update)
```
1. File watcher detects creation
2. FileScanner creates FileIndexEntry
3. IndexStore.upsertFile()
4. MetadataStore.getOrCreateMetadata()
5. refreshAllViews()
```

## Performance Characteristics

### Indexing Speed
- **10,000 files**: ~1-2 seconds (initial scan)
- **50,000 files**: ~5-10 seconds
- **Incremental update**: <10ms per file

### Query Speed (SQLite)
- **Get files by tag**: <1ms (indexed)
- **Get files by project**: <1ms (indexed)
- **Get all tags**: <1ms (DISTINCT with index)

### Memory Usage
- **IndexStore**: ~200 bytes/file (10k files = 2MB)
- **TreeProviders**: Lazy (only loaded nodes)
- **Total overhead**: <10MB for typical workspace

## Ignored Directories

```typescript
const IGNORE_DIRS = [
  '.git',           // Version control
  'node_modules',   // Dependencies
  '.vscode',        // Editor config
  '.cortex',        // Our own data
  'dist',           // Build output
  'build',          // Build output
  'out',            // Build output
  '.next',          // Next.js
  'target',         // Rust/Java
  'bin',            // Binaries
  'obj',            // C# intermediate
];
```

**Why hardcoded?**
- Covers 95% of use cases
- Prevents indexing thousands of dependencies
- User can extend via settings in future version

## Extension Lifecycle

### Activation
- Triggered: `onStartupFinished` (after VS Code is ready)
- Work: Index workspace, initialize UI
- Time budget: <5 seconds for good UX

### Deactivation
- Triggered: Extension disabled or VS Code closing
- Work: Close SQLite connection
- No data loss: SQLite auto-commits

### File Watching
- Uses VS Code's `createFileSystemWatcher`
- Monitors `**/*` (all files)
- Updates index incrementally
- Ignores events in ignored directories

## Future-Proofing

### Extensibility Points

1. **Content Indexing**
   - Store file content hashes
   - Enable full-text search
   - Track imports/references

2. **Relationship Graphs**
   - Which files import each other?
   - Dependency visualization

3. **Time-Based Views**
   - "Files modified this week"
   - "Recently tagged"

4. **Saved Queries**
   - Persist complex filters
   - Quick access to common views

5. **Team Sync**
   - Commit `.cortex/` to Git
   - Shared tagging conventions

### Migration Strategy

If schema changes are needed:

```sql
-- Add version table
CREATE TABLE metadata_version (version INTEGER);
INSERT INTO metadata_version VALUES (1);

-- Future migration
ALTER TABLE file_metadata ADD COLUMN new_field TEXT;
UPDATE metadata_version SET version = 2;
```

## Security Considerations

### Local-Only
- No network requests
- No telemetry
- No external dependencies (except better-sqlite3)

### File System Access
- Only reads workspace files
- Only writes to `.cortex/`
- Respects VS Code's sandboxing

### SQLite Injection
- Uses parameterized queries exclusively
- No string concatenation for SQL

```typescript
// SAFE
stmt = db.prepare("SELECT * FROM files WHERE tag = ?");
stmt.run(userInput);

// UNSAFE (not used)
db.exec(`SELECT * FROM files WHERE tag = '${userInput}'`);
```

## Testing Strategy (Future Work)

### Unit Tests
- FileScanner: Mock filesystem
- IndexStore: Test all query methods
- MetadataStore: Test SQLite operations

### Integration Tests
- Full activation sequence
- Command execution
- View rendering

### Performance Tests
- Benchmark indexing on large workspaces
- Query performance with 100k files

## Comparison to Alternatives

### VS Code Workspace Search
- **Pros**: Built-in, fast, content-aware
- **Cons**: No persistent organization, no projects

### File Explorer Folders
- **Pros**: Simple, visual, familiar
- **Cons**: Files can only be in one place, duplication required

### External Tools (Notion, Obsidian)
- **Pros**: Rich features, collaboration
- **Cons**: Not integrated with code editor, project switching

### Cortex Advantages
- **Integrated**: Lives in VS Code
- **Non-destructive**: Files stay put
- **Flexible**: Multiple organizational axes
- **Local**: No cloud dependency

---

**Cortex** is designed to scale from personal projects (100s of files) to large codebases (10,000+ files) while maintaining sub-second responsiveness.
