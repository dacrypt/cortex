# Cortex Knowledge Engine - Target Architecture

## Design Principles

1. **Markdown as Source of Truth**: Documents are Markdown files on disk. Cortex indexes and enriches, never modifies source.
2. **Local-First**: All data stored locally in SQLite. No cloud dependencies.
3. **Deterministic**: Same input produces same output. LLMs used only for classification/heuristics, not core logic.
4. **Graph-Based**: Projects and documents form graphs, not just hierarchies.
5. **State-Aware**: Documents have explicit lifecycle states.
6. **Temporal**: All events are logged for analytics and memory.

## Core Components

### 1. Domain Layer

#### 1.1 Document Model
```go
type DocumentState string

const (
    DocumentStateDraft     DocumentState = "draft"
    DocumentStateActive    DocumentState = "active"
    DocumentStateReplaced  DocumentState = "replaced"
    DocumentStateArchived  DocumentState = "archived"
)

type Document struct {
    ID           DocumentID
    FileID       FileID
    RelativePath string
    Title        string
    State        DocumentState
    Frontmatter  map[string]interface{}
    Checksum     string
    CreatedAt    time.Time
    UpdatedAt    time.Time
    StateHistory []DocumentStateTransition
}

type DocumentStateTransition struct {
    FromState   DocumentState
    ToState     DocumentState
    Reason      string
    ChangedAt   time.Time
    ChangedBy   string // Optional: user/system
}
```

#### 1.2 Project Model (Graph-Based)
```go
type ProjectID string

type Project struct {
    ID          ProjectID
    Name        string
    Description string
    ParentID    *ProjectID // Nil for root projects
    Path        string     // Hierarchical path: "parent/child/grandchild"
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

// ProjectGraph provides graph operations
type ProjectGraph interface {
    GetProject(id ProjectID) (*Project, error)
    GetChildren(id ProjectID) ([]*Project, error)
    GetAncestors(id ProjectID) ([]*Project, error)
    GetDescendants(id ProjectID) ([]*Project, error)
    GetRootProjects() ([]*Project, error)
}
```

#### 1.3 Relationship Model
```go
type RelationshipType string

const (
    RelationshipReplaces  RelationshipType = "replaces"
    RelationshipDependsOn RelationshipType = "depends_on"
    RelationshipBelongsTo  RelationshipType = "belongs_to"
    RelationshipReferences RelationshipType = "references"
)

type DocumentRelationship struct {
    ID           RelationshipID
    FromDocument DocumentID
    ToDocument   DocumentID
    Type         RelationshipType
    Strength     float64 // 0.0-1.0, optional confidence
    CreatedAt    time.Time
    Metadata     map[string]interface{} // Optional context
}

type ProjectDocumentRelationship struct {
    ID        RelationshipID
    ProjectID ProjectID
    DocumentID DocumentID
    Role      string // "primary", "reference", "archive"
    AddedAt   time.Time
}
```

#### 1.4 Temporal Memory Model
```go
type UsageEventType string

const (
    UsageEventOpened     UsageEventType = "opened"
    UsageEventEdited     UsageEventType = "edited"
    UsageEventSearched   UsageEventType = "searched"
    UsageEventReferenced UsageEventType = "referenced"
)

type DocumentUsageEvent struct {
    ID          EventID
    DocumentID  DocumentID
    EventType   UsageEventType
    Context     string // e.g., "project:auth-refactor"
    Timestamp   time.Time
    Metadata    map[string]interface{}
}

type DocumentUsageStats struct {
    DocumentID      DocumentID
    AccessCount     int
    LastAccessed    time.Time
    FirstAccessed   time.Time
    CoOccurrences   map[DocumentID]int // Documents used together
    Frequency       float64 // Accesses per day
}
```

### 2. Storage Layer

#### 2.1 Database Schema

**New Tables:**

```sql
-- Projects table (hierarchical)
CREATE TABLE projects (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    name TEXT NOT NULL,
    description TEXT,
    parent_id TEXT, -- NULL for root projects
    path TEXT NOT NULL, -- Hierarchical path
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL,
    FOREIGN KEY (workspace_id) REFERENCES workspaces(id) ON DELETE CASCADE,
    FOREIGN KEY (parent_id) REFERENCES projects(id) ON DELETE SET NULL
);

CREATE INDEX idx_projects_workspace ON projects(workspace_id);
CREATE INDEX idx_projects_parent ON projects(workspace_id, parent_id);
CREATE INDEX idx_projects_path ON projects(workspace_id, path);

-- Document states
ALTER TABLE documents ADD COLUMN state TEXT NOT NULL DEFAULT 'draft';
ALTER TABLE documents ADD COLUMN state_changed_at INTEGER;

CREATE TABLE document_state_history (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    from_state TEXT,
    to_state TEXT NOT NULL,
    reason TEXT,
    changed_by TEXT,
    changed_at INTEGER NOT NULL,
    FOREIGN KEY (workspace_id, document_id) REFERENCES documents(workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_state_history_document ON document_state_history(workspace_id, document_id);
CREATE INDEX idx_state_history_changed_at ON document_state_history(workspace_id, changed_at);

-- Document relationships
CREATE TABLE document_relationships (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    from_document_id TEXT NOT NULL,
    to_document_id TEXT NOT NULL,
    type TEXT NOT NULL,
    strength REAL,
    metadata TEXT, -- JSON
    created_at INTEGER NOT NULL,
    FOREIGN KEY (workspace_id, from_document_id) REFERENCES documents(workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, to_document_id) REFERENCES documents(workspace_id, id) ON DELETE CASCADE,
    UNIQUE(workspace_id, from_document_id, to_document_id, type)
);

CREATE INDEX idx_relationships_from ON document_relationships(workspace_id, from_document_id);
CREATE INDEX idx_relationships_to ON document_relationships(workspace_id, to_document_id);
CREATE INDEX idx_relationships_type ON document_relationships(workspace_id, type);

-- Project-document relationships (replaces file_contexts for projects)
CREATE TABLE project_documents (
    workspace_id TEXT NOT NULL,
    project_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    role TEXT NOT NULL DEFAULT 'primary',
    added_at INTEGER NOT NULL,
    PRIMARY KEY (workspace_id, project_id, document_id),
    FOREIGN KEY (workspace_id, project_id) REFERENCES projects(workspace_id, id) ON DELETE CASCADE,
    FOREIGN KEY (workspace_id, document_id) REFERENCES documents(workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_project_documents_project ON project_documents(workspace_id, project_id);
CREATE INDEX idx_project_documents_document ON project_documents(workspace_id, document_id);

-- Usage events (temporal memory)
CREATE TABLE document_usage_events (
    id TEXT PRIMARY KEY,
    workspace_id TEXT NOT NULL,
    document_id TEXT NOT NULL,
    event_type TEXT NOT NULL,
    context TEXT,
    metadata TEXT, -- JSON
    timestamp INTEGER NOT NULL,
    FOREIGN KEY (workspace_id, document_id) REFERENCES documents(workspace_id, id) ON DELETE CASCADE
);

CREATE INDEX idx_usage_events_document ON document_usage_events(workspace_id, document_id);
CREATE INDEX idx_usage_events_timestamp ON document_usage_events(workspace_id, timestamp);
CREATE INDEX idx_usage_events_type ON document_usage_events(workspace_id, event_type);
```

#### 2.2 Repository Interfaces

```go
// ProjectRepository
type ProjectRepository interface {
    Create(ctx context.Context, workspaceID WorkspaceID, project *Project) error
    Get(ctx context.Context, workspaceID WorkspaceID, id ProjectID) (*Project, error)
    GetByPath(ctx context.Context, workspaceID WorkspaceID, path string) (*Project, error)
    Update(ctx context.Context, workspaceID WorkspaceID, project *Project) error
    Delete(ctx context.Context, workspaceID WorkspaceID, id ProjectID) error
    
    // Graph operations
    GetChildren(ctx context.Context, workspaceID WorkspaceID, parentID ProjectID) ([]*Project, error)
    GetAncestors(ctx context.Context, workspaceID WorkspaceID, id ProjectID) ([]*Project, error)
    GetDescendants(ctx context.Context, workspaceID WorkspaceID, id ProjectID) ([]*Project, error)
    GetRootProjects(ctx context.Context, workspaceID WorkspaceID) ([]*Project, error)
}

// DocumentStateRepository
type DocumentStateRepository interface {
    SetState(ctx context.Context, workspaceID WorkspaceID, docID DocumentID, state DocumentState, reason string) error
    GetState(ctx context.Context, workspaceID WorkspaceID, docID DocumentID) (DocumentState, error)
    GetStateHistory(ctx context.Context, workspaceID WorkspaceID, docID DocumentID) ([]DocumentStateTransition, error)
    GetDocumentsByState(ctx context.Context, workspaceID WorkspaceID, state DocumentState) ([]DocumentID, error)
}

// RelationshipRepository
type RelationshipRepository interface {
    Create(ctx context.Context, workspaceID WorkspaceID, rel *DocumentRelationship) error
    Get(ctx context.Context, workspaceID WorkspaceID, id RelationshipID) (*DocumentRelationship, error)
    Delete(ctx context.Context, workspaceID WorkspaceID, id RelationshipID) error
    
    // Query operations
    GetOutgoing(ctx context.Context, workspaceID WorkspaceID, docID DocumentID, relType RelationshipType) ([]*DocumentRelationship, error)
    GetIncoming(ctx context.Context, workspaceID WorkspaceID, docID DocumentID, relType RelationshipType) ([]*DocumentRelationship, error)
    GetRelated(ctx context.Context, workspaceID WorkspaceID, docID DocumentID, relType RelationshipType) ([]DocumentID, error)
    Traverse(ctx context.Context, workspaceID WorkspaceID, startDocID DocumentID, relType RelationshipType, maxDepth int) ([]DocumentID, error)
}

// UsageRepository
type UsageRepository interface {
    RecordEvent(ctx context.Context, workspaceID WorkspaceID, event *DocumentUsageEvent) error
    GetUsageStats(ctx context.Context, workspaceID WorkspaceID, docID DocumentID, since time.Time) (*DocumentUsageStats, error)
    GetCoOccurrences(ctx context.Context, workspaceID WorkspaceID, docID DocumentID, limit int) (map[DocumentID]int, error)
    GetFrequentlyUsed(ctx context.Context, workspaceID WorkspaceID, since time.Time, limit int) ([]DocumentID, error)
    GetRecentlyUsed(ctx context.Context, workspaceID WorkspaceID, limit int) ([]DocumentID, error)
}
```

### 3. Application Layer

#### 3.1 Query Service

```go
type QueryService struct {
    docRepo      DocumentRepository
    projectRepo  ProjectRepository
    relRepo      RelationshipRepository
    usageRepo    UsageRepository
    ragService   *rag.Service
}

// QueryBuilder provides declarative query interface
type QueryBuilder struct {
    workspaceID WorkspaceID
    filters     []QueryFilter
    projections []QueryProjection
    ordering    QueryOrdering
    limit       int
}

type QueryFilter interface {
    Apply(ctx context.Context, qb *QueryBuilder) ([]DocumentID, error)
}

// Example filters
type ProjectFilter struct {
    ProjectID ProjectID
    IncludeSubprojects bool
}

type StateFilter struct {
    States []DocumentState
}

type RelationshipFilter struct {
    FromDocument DocumentID
    RelationshipType RelationshipType
    MaxDepth int
}

type TemporalFilter struct {
    Since time.Time
    EventType UsageEventType
}

// Query execution
func (qs *QueryService) Execute(ctx context.Context, qb *QueryBuilder) (*QueryResult, error) {
    // Apply filters sequentially
    // Combine results
    // Apply projections
    // Sort and limit
    // Return structured result
}
```

#### 3.2 Indexing Orchestrator (Enhanced)

```go
type IndexingOrchestrator struct {
    fileScanner    FileScanner
    parser         DocumentParser
    classifier     DocumentClassifier
    linker         RelationshipLinker
    stateManager   DocumentStateManager
    usageTracker   UsageTracker
}

// Process flow:
// 1. FileScanner → FileEntry
// 2. Parser → Document (parse Markdown, extract structure)
// 3. Classifier → Project assignment, state inference
// 4. Linker → Relationship detection (explicit + inferred)
// 5. StateManager → State transitions
// 6. UsageTracker → Record indexing event
```

### 4. Boundaries

#### 4.1 Cortex MUST Do
- Index Markdown documents from filesystem
- Maintain document state and relationships
- Track project hierarchy
- Log usage events
- Provide query interface (semantic + structural)
- Generate visualization data structures

#### 4.2 Cortex MUST NOT Do
- Modify source Markdown files
- Implement WYSIWYG editor
- Reimplement Cursor functionality
- Hide logic in prompts (all reasoning explicit)
- Treat documents as flat chunks only
- Rely solely on embeddings for reasoning

### 5. Data Flow

```
Filesystem (Markdown files)
    ↓
FileScanner → FileEntry
    ↓
DocumentParser → Document (with chunks)
    ↓
DocumentClassifier → Project assignment, State inference
    ↓
RelationshipLinker → Explicit + inferred relationships
    ↓
StateManager → State transitions
    ↓
Storage (SQLite)
    ↓
QueryService → Structured queries
    ↓
API (gRPC/HTTP) → Cursor/MCP
```

### 6. Query Patterns

#### 6.1 Structural Queries
```go
// "Documents in project X and subprojects"
Query().
    Filter(ProjectFilter{ProjectID: "x", IncludeSubprojects: true}).
    Execute()

// "Current valid document for topic Y"
Query().
    Filter(ProjectFilter{ProjectID: "y"}).
    Filter(StateFilter{States: []DocumentState{DocumentStateActive}}).
    OrderBy(Ordering{Field: "updated_at", Desc: true}).
    Limit(1).
    Execute()

// "Documents that replace document Z"
Query().
    Filter(RelationshipFilter{
        FromDocument: "z",
        RelationshipType: RelationshipReplaces,
    }).
    Execute()
```

#### 6.2 Temporal Queries
```go
// "Documents used this week"
Query().
    Filter(TemporalFilter{
        Since: time.Now().AddDate(0, 0, -7),
        EventType: UsageEventOpened,
    }).
    Execute()

// "Documents usually used together with document A"
usageRepo.GetCoOccurrences(ctx, workspaceID, docA, 10)
```

#### 6.3 Graph Traversal
```go
// "All documents in project hierarchy"
projectRepo.GetDescendants(ctx, workspaceID, projectID)
projectRepo.GetDocuments(ctx, workspaceID, descendantIDs)

// "Document dependency chain"
relRepo.Traverse(ctx, workspaceID, startDoc, RelationshipDependsOn, 10)
```

## Implementation Priorities

1. **Domain Models** (Phase 4.1)
   - Document state enum + transitions
   - Project graph model
   - Relationship types

2. **Storage Schema** (Phase 4.1)
   - Database migrations
   - Repository implementations

3. **Relationship Modeling** (Phase 4.2)
   - Relationship detection
   - Graph traversal

4. **Temporal Memory** (Phase 4.3)
   - Usage event logging
   - Analytics queries

5. **Query Layer** (Phase 4.5)
   - Query builder
   - Graph traversal queries

6. **API Surface** (Phase 4.6)
   - gRPC service extensions
   - MCP compatibility

