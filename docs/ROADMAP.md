# Cortex Roadmap

Future enhancements for Cortex, organized by theme. All additions maintain the core principles: local-first, deterministic, no AI/cloud.

---

## Phase 1: MVP (Current)

✅ **Complete**

- [x] Workspace indexing
- [x] SQLite metadata storage
- [x] Three virtual views (Project, Tag, Type)
- [x] Four commands (add tag, assign project, open view, rebuild index)
- [x] File watcher for incremental updates
- [x] Activity Bar integration

---

## Phase 2: Enhanced UX

### 2.1 Multi-Select Operations

**Goal**: Tag/project-assign multiple files at once

**Features**:
- Select multiple files in File Explorer → Right-click → "Add Cortex Tag"
- Bulk assign projects to all files in a folder
- Remove tags/projects from multiple files

**Implementation**:
- Extend commands to accept URI arrays
- Add context menu entries for `explorer/context`

**Effort**: Small (1-2 days)

---

### 2.2 Quick Pick Suggestions

**Goal**: Autocomplete for tags and projects

**Features**:
- Show existing tags when adding a new tag (quick pick)
- Recently used tags appear first
- Same for projects

**Implementation**:
- Replace `showInputBox` with `showQuickPick`
- Sort by frequency of use

**Effort**: Small (1 day)

---

### 2.3 Inline Tag/Project Badges

**Goal**: Show tags/projects directly in File Explorer

**Features**:
- File Explorer shows badges next to files (e.g., "📁 project-alpha" or "🏷️ urgent")
- Configurable in settings (on/off)

**Implementation**:
- Use `FileDecorationProvider` API
- Query metadata for visible files
- Show first tag/project as suffix

**Effort**: Medium (2-3 days)

---

### 2.4 Search/Filter in Views

**Goal**: Filter Cortex views by keyword

**Features**:
- Search box at top of Project/Tag/Type views
- Filter tree items by name
- Fuzzy matching

**Implementation**:
- Add `TreeViewOptions` with `canSelectMany`
- Implement search logic in providers

**Effort**: Medium (2-3 days)

---

## Phase 3: Advanced Metadata

### 3.1 File Notes

**Goal**: Attach freeform notes to files

**Features**:
- Command: "Cortex: Edit notes for current file"
- Hover tooltip shows notes in tree views
- Full text search in notes (SQLite FTS)

**Implementation**:
- Already in schema (`notes` column)
- Add command + UI (input box or editor)

**Effort**: Small (1-2 days)

---

### 3.2 Custom Metadata Fields

**Goal**: User-defined fields (priority, owner, status, etc.)

**Features**:
- Settings: Define custom fields (name, type, options)
- Command: "Cortex: Set custom field"
- Views: Group by custom field

**Implementation**:
- New table: `custom_fields` (file_id, field_name, field_value)
- Dynamic TreeProvider based on schema

**Effort**: Large (1-2 weeks)

---

### 3.3 Relationships

**Goal**: Link files to each other

**Features**:
- "This design relates to these components"
- "This test covers these source files"
- Graph view (requires visualization)

**Implementation**:
- New table: `file_relationships` (from_id, to_id, relation_type)
- Commands to add/remove links

**Effort**: Large (2-3 weeks)

---

## Phase 4: Smart Indexing

### 4.1 Content-Based Search

**Goal**: Search inside files, not just metadata

**Features**:
- Index file contents (SQLite FTS5)
- Command: "Cortex: Search all files"
- Results grouped by project/tag

**Implementation**:
- Read file contents during indexing
- Store in FTS5 table
- Incremental updates on file save

**Effort**: Medium (1 week)

---

### 4.2 Import/Export Tracking

**Goal**: Know which files import each other

**Features**:
- View: "By Dependencies"
- "Find all files that import X"
- "Find all files imported by X"

**Implementation**:
- Parse imports (TypeScript, Python, etc.)
- Store in `file_dependencies` table
- Use AST parsers (ts-morph, recast, etc.)

**Effort**: Large (2-3 weeks, per language)

---

### 4.3 Git Integration

**Goal**: Leverage Git history

**Features**:
- Auto-tag files modified in last commit: `recent-change`
- Group by "Changed this week"
- Show who last edited (from Git blame)

**Implementation**:
- Use `simple-git` library
- Query Git log during indexing
- Store `last_committer`, `last_commit_date` in metadata

**Effort**: Medium (1 week)

---

## Phase 5: Team Collaboration

### 5.1 Shared Projects (via Git)

**Goal**: Team shares `.cortex/` in version control

**Features**:
- Commit `.cortex/index.sqlite` to Git
- Merge conflicts resolved via rebuild
- Documentation on team workflows

**Implementation**:
- Add `.cortex/` to `.gitignore` by default
- Provide opt-in setting to track it
- Handle SQLite merge conflicts gracefully

**Effort**: Small (1-2 days, mostly docs)

---

### 5.2 Project Templates

**Goal**: Predefined project structures for teams

**Features**:
- Template: "New client project" → creates projects `client-x`, `client-x-dev`, `client-x-docs`
- Share templates via JSON files

**Implementation**:
- Settings: `cortex.projectTemplates`
- Command: "Cortex: Apply template"

**Effort**: Small (1-2 days)

---

### 5.3 Export/Import Metadata

**Goal**: Share metadata without Git

**Features**:
- Export metadata to JSON
- Import from JSON (merge or replace)
- Use case: Onboarding new team members

**Implementation**:
- Command: "Cortex: Export metadata"
- Command: "Cortex: Import metadata"
- Handle conflicts (last-write-wins or prompt)

**Effort**: Small (1-2 days)

---

## Phase 6: Visualization

### 6.1 Graph View

**Goal**: Visualize relationships between files

**Features**:
- WebView panel showing graph
- Nodes = files, edges = relationships (imports, "relates-to", same project)
- Interactive (click to open file)

**Implementation**:
- Use D3.js or Cytoscape.js
- Query relationships from SQLite
- Render in VS Code WebView

**Effort**: Large (2-3 weeks)

---

### 6.2 Tag Cloud

**Goal**: Visual overview of tags

**Features**:
- WebView panel showing tag cloud
- Size = number of files with tag
- Click tag to filter

**Implementation**:
- Query tag counts
- Render with D3.js
- Integrate with tree views

**Effort**: Medium (1 week)

---

### 6.3 Timeline View

**Goal**: See file activity over time

**Features**:
- Group files by "Last modified"
- Heatmap of activity
- Filter by project/tag

**Implementation**:
- Use `lastModified` from index
- Group into buckets (today, this week, this month, older)
- Render in tree view or WebView

**Effort**: Medium (1 week)

---

## Phase 7: Performance & Scale

### 7.1 Virtual Scrolling for Large Views

**Goal**: Handle 10,000+ items in tree view

**Features**:
- Only render visible items
- Lazy load children

**Implementation**:
- VS Code TreeView already handles this
- Optimize queries to return only needed data

**Effort**: Small (optimize queries only)

---

### 7.2 Background Indexing

**Goal**: Index large workspaces without blocking UI

**Features**:
- Index in chunks (1000 files at a time)
- Progress indicator
- Cancelable

**Implementation**:
- Use Web Workers or Node worker threads
- Stream results to IndexStore

**Effort**: Medium (1 week)

---

### 7.3 Indexed Queries

**Goal**: Sub-millisecond queries on 100k+ files

**Features**:
- Benchmark query performance
- Add SQLite indexes where needed
- Cache frequently used queries

**Implementation**:
- Profile slow queries with SQLite `EXPLAIN QUERY PLAN`
- Add composite indexes
- Use prepared statements (already doing this)

**Effort**: Small (1-2 days)

---

## Phase 8: Integrations

### 8.1 Task Tracker Integration

**Goal**: Link files to Jira/GitHub Issues/Linear

**Features**:
- Tag files with `issue:JIRA-123`
- Fetch issue details via API
- Show issue title/status in tooltip

**Implementation**:
- Settings: API keys for issue trackers
- Fetch issue metadata on demand
- Cache in SQLite

**Effort**: Medium (1 week per integration)

---

### 8.2 Email Integration

**Goal**: Link email threads to files

**Features**:
- Tag files with `email:thread-id`
- Open related emails from Cortex
- Use case: Client communication + deliverables

**Implementation**:
- Integrate with mail clients (Outlook, Gmail APIs)
- Store email metadata

**Effort**: Large (2-3 weeks, highly dependent on mail client)

---

### 8.3 Calendar Integration

**Goal**: Tag files by meeting/event

**Features**:
- Tag files with `meeting:2024-01-15-kickoff`
- Link to calendar event
- Show upcoming deadlines

**Implementation**:
- Integrate with Google Calendar / Outlook Calendar APIs
- Store event metadata

**Effort**: Medium (1-2 weeks)

---

## Phase 9: Research-Based Organization Systems

**Note**: These features are based on academic research in information organization, classification, and personal information management. See [FUTURE_RESEARCH_PROPOSALS.md](FUTURE_RESEARCH_PROPOSALS.md) for detailed references and implementation proposals.

### 9.1 Faceted Classification

**Goal**: Organize documents using multiple independent dimensions (facets)

**Features**:
- Multiple classification axes (topic, type, status, priority, date, etc.)
- Combine facets for complex queries
- Faceted search interface
- Custom facet definitions

**Research Basis**: Ranganathan (1933), Svenonius (2000), Broughton (2004)

**Effort**: Large (3-4 weeks)

---

### 9.2 Polyhierarchies

**Goal**: Allow documents to belong to multiple hierarchies simultaneously

**Features**:
- Multiple hierarchy views (project, technology, team, lifecycle)
- Cross-hierarchy navigation
- Hierarchy-aware search

**Research Basis**: Svenonius (2000), Hjørland (2013)

**Effort**: Medium (2-3 weeks)

---

### 9.3 Knowledge Graph Visualization

**Goal**: Visualize document relationships as an interactive knowledge graph

**Features**:
- Graph view of document relationships
- Interactive navigation
- Community detection (clusters)
- Path finding between documents

**Research Basis**: Ehrlinger & Wöß (2016), Paulheim (2017)

**Effort**: Large (3-4 weeks)

---

### 9.4 Activity-Based Organization

**Goal**: Organize files by activity/project context, not just type

**Features**:
- Activity entities (projects, tasks, contexts)
- Temporal activity tracking
- Activity-based document retrieval
- Related activities discovery

**Research Basis**: Jones (2007), "Keeping Found Things Found"

**Effort**: Large (3-4 weeks)

---

### 9.5 Information Scraps Management

**Goal**: Handle fragments of information that don't fit formal structures

**Features**:
- Quick notes, URLs, ideas, todos
- Link scraps to documents
- Scrap search and organization
- Scrap-to-document conversion

**Research Basis**: Jones (2007), Whittaker & Sidner (1996)

**Effort**: Medium (2 weeks)

---

### 9.6 Collaborative Tagging (Folksonomies)

**Goal**: Enable team-based tagging with emergent taxonomies

**Features**:
- Multi-user tagging
- Tag popularity and co-occurrence
- Tag suggestions based on team usage
- Tag synonym detection

**Research Basis**: Vander Wal (2007), Marlow et al. (2006)

**Effort**: Medium (2-3 weeks)

---

### 9.7 Document Recommendation System

**Goal**: Recommend relevant documents based on usage patterns and content

**Features**:
- Collaborative filtering (users with similar interests)
- Content-based recommendations
- Context-aware suggestions
- Recommendation explanations

**Research Basis**: Ricci et al. (2011), Aggarwal (2016)

**Effort**: Large (4-5 weeks)

---

### 9.8 Temporal and Contextual Organization

**Goal**: Organize based on when and in what context documents are used

**Features**:
- Access pattern analysis
- Contextual retrieval ("documents I used when working on X")
- Temporal clustering
- Context-aware search

**Research Basis**: Dumais et al. (2003), Teevan et al. (2004)

**Effort**: Medium (2-3 weeks)

---

### 9.9 Advanced Visualizations

**Goal**: Visual exploration of document spaces

**Features**:
- 2D/3D document maps (semantic similarity)
- Timeline visualization
- Network graph views
- Cluster visualization

**Research Basis**: Card et al. (1999), Shneiderman (1996)

**Effort**: Large (4-5 weeks)

---

### 9.10 Rich Metadata Extraction

**Goal**: Extract and use comprehensive metadata from documents

**Features**:
- Dublin Core metadata support
- Named entity extraction
- Citation network analysis
- Automatic concept extraction

**Research Basis**: Dublin Core Metadata Initiative, ISO 15836

**Effort**: Medium (2-3 weeks)

---

## Phase 10: AI Enhancements (Optional)

**Note**: These violate the "no AI" MVP constraint but may be valuable for power users.

### 9.1 Auto-Tagging Suggestions

**Goal**: Suggest tags based on file content

**Features**:
- After opening a file, suggest tags
- "This looks like a test file, tag it 'test'?"
- User confirms (never automatic)

**Implementation**:
- Local ML model (TensorFlow.js) or keyword heuristics
- Always requires user approval

**Effort**: Large (3-4 weeks)

---

### 9.2 Smart Project Detection

**Goal**: Suggest projects based on imports/paths

**Features**:
- "Files in `src/auth/` often belong to project 'authentication'"
- Learn from user behavior

**Implementation**:
- Track patterns (path → project mappings)
- Suggest when user adds files to new folders

**Effort**: Large (3-4 weeks)

---

## Non-Goals (Out of Scope)

These are explicitly **NOT** planned:

- ❌ **Cloud sync** - Local-first is core principle
- ❌ **Real-time collaboration** - Use Git for team sharing
- ❌ **File hosting** - Cortex organizes, doesn't store
- ❌ **Web UI** - VS Code extension only
- ❌ **Mobile app** - Desktop-focused
- ❌ **Auto-organization** - User control is key

---

## Community Requests

Ideas from users (if open-sourced):

1. **Saved Searches** - Persist complex queries
2. **Keyboard Shortcuts** - Quick access to common commands
3. **Color Coding** - Different colors for different projects/tags
4. **Emoji Support** - Use emojis in tag/project names
5. **Multi-Workspace** - Manage multiple workspaces from one index

---

## Versioning Plan

### v0.1.0 (MVP)
- Current implementation

### v0.2.0 (UX Improvements)
- Phase 2 features (multi-select, suggestions, badges)

### v0.3.0 (Advanced Metadata)
- Phase 3 features (notes, custom fields)

### v0.4.0 (Smart Indexing)
- Phase 4 features (content search, dependencies)

### v1.0.0 (Production Ready)
- All critical bugs fixed
- Performance tested at scale
- Comprehensive documentation
- Team collaboration features

---

## How to Contribute

1. **Pick a feature** from this roadmap
2. **Open an issue** to discuss approach
3. **Submit a PR** with tests
4. **Update docs** (README, ARCHITECTURE, etc.)

---

**Cortex Roadmap** - Building the future of semantic file organization, one feature at a time.
