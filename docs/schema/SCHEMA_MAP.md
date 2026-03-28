# Schema Map: Migration to Table Mappings

This document maps each database migration to the tables and columns it creates or modifies.

## Migration Overview

| Version | Name | Category | Tables Created/Modified |
|---------|------|----------|------------------------|
| v1 | initial_schema | Core | workspaces, files, file_metadata, file_tags, file_contexts, file_context_suggestions, tasks, scheduled_tasks |
| v2 | documents_and_vectors | RAG | documents, chunks, chunk_embeddings |
| v3 | ai_metadata_fields | AI | file_metadata (columns) |
| v4 | file_processing_traces | Observability | file_traces |
| v5 | knowledge_engine_features | Knowledge | projects, document_state_history, document_relationships, project_documents, document_usage_events |
| v6 | detected_language_field | AI | file_metadata (column) |
| v7 | project_nature_and_attributes | Knowledge | projects (columns) |
| v8 | config_versions | Config | config_versions |
| v9 | suggested_metadata | AI Suggestions | suggested_metadata, suggested_tags, suggested_projects, suggested_taxonomy, suggested_taxonomy_topics, suggested_fields |
| v10 | ai_context_field | AI | file_metadata (column) |
| v11 | enrichment_data_field | Enrichment | file_metadata (column) |
| v12 | os_metadata_and_users | OS/Users | persons, system_users, file_ownership, file_access, project_memberships, files (columns) |
| v13 | additional_timestamps | Files | files (columns) |
| v14 | path_components | Files | files (columns) |
| v15 | denormalize_ai_context | AI Denorm | file_authors, file_locations, file_people, file_organizations, file_events, file_references, file_publication_info |
| v16 | denormalize_enrichment | Enrichment Denorm | file_named_entities, file_citations, file_dependencies, file_duplicates, file_sentiment |
| v17 | index_frequently_queried_metadata | Indexes | (indexes only) |
| v18 | file_relationships | Code Analysis | file_relationships |
| v19 | enhance_document_relationships | Knowledge | document_relationships (columns) |
| v20 | temporal_analysis | Temporal | file_access_events, file_modification_history, temporal_clusters |
| v21 | file_hashes | Deduplication | files (columns) |
| v23 | mirror_extraction_metadata | Mirroring | (placeholder) |

---

## Detailed Migration Breakdown

### v1: initial_schema (Core Infrastructure)

**Tables Created:**

| Table | Purpose | Primary Key |
|-------|---------|-------------|
| workspaces | Registered workspaces | id |
| files | File index entries | (workspace_id, id) |
| file_metadata | Semantic metadata | (workspace_id, file_id) |
| file_tags | File-to-tag associations | (workspace_id, file_id, tag) |
| file_contexts | File-to-project associations | (workspace_id, file_id, context) |
| file_context_suggestions | Suggested project associations | (workspace_id, file_id, context) |
| tasks | Async task queue | id |
| scheduled_tasks | Cron-scheduled tasks | id |

---

### v2: documents_and_vectors (RAG Infrastructure)

**Tables Created:**

| Table | Purpose | Primary Key |
|-------|---------|-------------|
| documents | Document registry for RAG | (workspace_id, id) |
| chunks | Document chunks for embedding | (workspace_id, id) |
| chunk_embeddings | Vector embeddings for similarity search | (workspace_id, chunk_id) |

---

### v3: ai_metadata_fields (AI Metadata)

**Columns Added to file_metadata:**

| Column | Type | Description |
|--------|------|-------------|
| ai_category | TEXT | AI-assigned category |
| ai_category_confidence | REAL | Confidence score (0-1) |
| ai_category_updated_at | INTEGER | Timestamp of last update |
| ai_related | TEXT | JSON array of related file IDs |

---

### v4: file_processing_traces (Observability)

**Tables Created:**

| Table | Purpose | Columns |
|-------|---------|---------|
| file_traces | Processing audit log | workspace_id, file_id, relative_path, stage, operation, prompt_path, output_path, prompt_preview, output_preview, model, tokens_used, duration_ms, error, created_at |

---

### v5: knowledge_engine_features (Knowledge Graph)

**Tables Created:**

| Table | Purpose | Primary Key |
|-------|---------|-------------|
| projects | Hierarchical projects | (workspace_id, id) |
| document_state_history | State change audit | id |
| document_relationships | Document-to-document links | id |
| project_documents | Project-document associations | (workspace_id, project_id, document_id) |
| document_usage_events | Temporal memory | id |

**Columns Added to documents:**

| Column | Type | Description |
|--------|------|-------------|
| state | TEXT | Document state (draft, published, etc.) |
| state_changed_at | INTEGER | State change timestamp |

---

### v6: detected_language_field

**Columns Added to file_metadata:**

| Column | Type | Description |
|--------|------|-------------|
| detected_language | TEXT | Detected language code (en, es, etc.) |

---

### v7: project_nature_and_attributes

**Columns Added to projects:**

| Column | Type | Description |
|--------|------|-------------|
| nature | TEXT | Project nature (generic, writing, development, etc.) |
| attributes | TEXT | JSON attributes (status, priority, complexity, etc.) |

---

### v8: config_versions

**Tables Created:**

| Table | Purpose | Primary Key |
|-------|---------|-------------|
| config_versions | Versioned configuration snapshots | version_id |

---

### v9: suggested_metadata (AI Suggestions)

**Tables Created:**

| Table | Purpose | Primary Key |
|-------|---------|-------------|
| suggested_metadata | Master suggestion record | (workspace_id, file_id) |
| suggested_tags | AI-suggested tags | (workspace_id, file_id, tag) |
| suggested_projects | AI-suggested projects | (workspace_id, file_id, project_name) |
| suggested_taxonomy | AI-suggested taxonomy | (workspace_id, file_id) |
| suggested_taxonomy_topics | Taxonomy topic associations | (workspace_id, file_id, topic) |
| suggested_fields | AI-suggested custom fields | (workspace_id, file_id, field_name) |

---

### v10: ai_context_field

**Columns Added to file_metadata:**

| Column | Type | Description |
|--------|------|-------------|
| ai_context | TEXT | JSON blob of AIContext entity |

---

### v11: enrichment_data_field

**Columns Added to file_metadata:**

| Column | Type | Description |
|--------|------|-------------|
| enrichment_data | TEXT | JSON blob of EnrichmentData entity |

---

### v12: os_metadata_and_users

**Tables Created:**

| Table | Purpose | Primary Key |
|-------|---------|-------------|
| persons | Human identities | id |
| system_users | OS user accounts | id |
| file_ownership | File-to-user ownership | (workspace_id, file_id, user_id) |
| file_access | File ACLs | id |
| project_memberships | Person-to-project roles | (workspace_id, project_id, person_id) |

**Columns Added to files:**

| Column | Type | Description |
|--------|------|-------------|
| os_metadata | TEXT | JSON of OSMetadata |
| os_taxonomy | TEXT | JSON of OSContextTaxonomy |

---

### v13: additional_timestamps

**Columns Added to files:**

| Column | Type | Description |
|--------|------|-------------|
| accessed_at | INTEGER | Last access timestamp |
| changed_at | INTEGER | Last change timestamp |
| backup_at | INTEGER | Last backup timestamp |

---

### v14: path_components

**Columns Added to files:**

| Column | Type | Description |
|--------|------|-------------|
| path_components | TEXT | JSON array of path components |
| path_pattern | TEXT | Normalized path pattern |

---

### v15: denormalize_ai_context

**Tables Created:**

| Table | Purpose | Primary Key | Source |
|-------|---------|-------------|--------|
| file_authors | Extracted authors | id | ai_context.Authors |
| file_locations | Geographic locations | id | ai_context.Locations |
| file_people | Mentioned people | id | ai_context.PeopleMentioned |
| file_organizations | Mentioned organizations | id | ai_context.Organizations |
| file_events | Historical events | id | ai_context.HistoricalEvents |
| file_references | Bibliographic references | id | ai_context.References |
| file_publication_info | Publication metadata | id | ai_context.Publication* |

---

### v16: denormalize_enrichment

**Tables Created:**

| Table | Purpose | Primary Key | Source |
|-------|---------|-------------|--------|
| file_named_entities | NER results | id | enrichment_data.NamedEntities |
| file_citations | Extracted citations | id | enrichment_data.Citations |
| file_dependencies | Code dependencies | id | enrichment_data.Dependencies |
| file_duplicates | Duplicate detection | id | enrichment_data.Duplicates |
| file_sentiment | Sentiment analysis | id | enrichment_data.Sentiment |

---

### v17: index_frequently_queried_metadata

**Indexes Created:**

| Index | Table | Columns |
|-------|-------|---------|
| idx_files_extension | files | workspace_id, extension |
| idx_files_path_pattern | files | workspace_id, path_pattern |

---

### v18: file_relationships

**Tables Created:**

| Table | Purpose | Primary Key |
|-------|---------|-------------|
| file_relationships | Code import/export tracking | id |

**Columns:**
- from_file_id, to_file_id, type (import/export/include/require/reference), language, confidence

---

### v19: enhance_document_relationships

**Columns Added to document_relationships:**

| Column | Type | Description |
|--------|------|-------------|
| confidence | REAL | Relationship confidence score |
| discovery_method | TEXT | How relationship was discovered |

---

### v20: temporal_analysis

**Tables Created:**

| Table | Purpose | Primary Key |
|-------|---------|-------------|
| file_access_events | Access event tracking | id |
| file_modification_history | Modification history | id |
| temporal_clusters | Edit session clusters | id |

---

### v21: file_hashes

**Columns Added to files:**

| Column | Type | Description |
|--------|------|-------------|
| file_hash_md5 | TEXT | MD5 hash for quick comparison |
| file_hash_sha256 | TEXT | SHA-256 hash for duplicate detection |
| file_hash_sha512 | TEXT | SHA-512 hash for high security |

---

### v23: mirror_extraction_metadata

**Purpose:** Placeholder for future denormalization of mirror extraction data.

---

## Table Relationships

```
workspaces (1) ──────┬───── (*) files
                     ├───── (*) projects
                     └───── (*) documents

files (1) ───────────┬───── (1) file_metadata
                     ├───── (*) file_tags
                     ├───── (*) file_contexts
                     ├───── (*) file_authors
                     ├───── (*) file_locations
                     ├───── (*) file_people
                     ├───── (*) file_organizations
                     ├───── (*) file_events
                     ├───── (*) file_references
                     ├───── (1) file_publication_info
                     ├───── (*) file_named_entities
                     ├───── (*) file_citations
                     ├───── (*) file_dependencies
                     ├───── (*) file_duplicates
                     ├───── (1) file_sentiment
                     ├───── (*) file_relationships (from)
                     ├───── (*) file_relationships (to)
                     └───── (*) file_access_events

documents (1) ───────┬───── (*) chunks
                     ├───── (*) document_relationships
                     └───── (*) project_documents

projects (1) ────────┬───── (*) project_documents
                     ├───── (*) project_memberships
                     └───── (*) projects (parent-child)

chunks (1) ──────────└───── (1) chunk_embeddings
```

---

## Index Summary

| Category | Count | Purpose |
|----------|-------|---------|
| Primary Keys | 25+ | Unique identification |
| Foreign Key Indexes | 30+ | Join performance |
| Query Indexes | 20+ | Search/filter performance |
| Composite Indexes | 15+ | Multi-column queries |
