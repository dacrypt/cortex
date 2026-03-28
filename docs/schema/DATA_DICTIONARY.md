# Data Dictionary: AI Metadata Tables

This document provides detailed field-level documentation for all AI metadata tables created in migrations v13-v23.

---

## Table of Contents

1. [file_authors](#file_authors)
2. [file_locations](#file_locations)
3. [file_people](#file_people)
4. [file_organizations](#file_organizations)
5. [file_events](#file_events)
6. [file_references](#file_references)
7. [file_publication_info](#file_publication_info)
8. [file_named_entities](#file_named_entities)
9. [file_citations](#file_citations)
10. [file_dependencies](#file_dependencies)
11. [file_duplicates](#file_duplicates)
12. [file_sentiment](#file_sentiment)
13. [file_relationships](#file_relationships)
14. [file_access_events](#file_access_events)
15. [file_modification_history](#file_modification_history)
16. [temporal_clusters](#temporal_clusters)

---

## file_authors

**Migration:** v15 (denormalize_ai_context)
**Source:** AIContext.Authors
**Purpose:** Stores extracted author information from documents.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | TEXT | NO | - | UUID primary key |
| workspace_id | TEXT | NO | - | FK to workspaces.id |
| file_id | TEXT | NO | - | FK to files.id |
| name | TEXT | NO | - | Author name |
| role | TEXT | YES | NULL | author, co-author, contributor, editor |
| affiliation | TEXT | YES | NULL | Institution or organization |
| confidence | REAL | YES | NULL | Extraction confidence (0.0-1.0) |
| created_at | INTEGER | NO | - | Unix milliseconds |

**Indexes:**
- `idx_file_authors_file (workspace_id, file_id)` - Find authors for a file
- `idx_file_authors_name (workspace_id, name)` - Find files by author name

**Unique Constraint:** `(workspace_id, file_id, name)`

---

## file_locations

**Migration:** v15 (denormalize_ai_context)
**Source:** AIContext.Locations
**Purpose:** Stores geographic locations mentioned in documents.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | TEXT | NO | - | UUID primary key |
| workspace_id | TEXT | NO | - | FK to workspaces.id |
| file_id | TEXT | NO | - | FK to files.id |
| name | TEXT | NO | - | Location name |
| type | TEXT | YES | NULL | city, country, region, address, landmark |
| coordinates | TEXT | YES | NULL | JSON with lat/lng |
| context | TEXT | YES | NULL | How location is referenced |
| created_at | INTEGER | NO | - | Unix milliseconds |

**Indexes:**
- `idx_file_locations_file (workspace_id, file_id)` - Find locations for a file

**Unique Constraint:** `(workspace_id, file_id, name)`

---

## file_people

**Migration:** v15 (denormalize_ai_context)
**Source:** AIContext.PeopleMentioned
**Purpose:** Stores people mentioned in documents (distinct from authors).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | TEXT | NO | - | UUID primary key |
| workspace_id | TEXT | NO | - | FK to workspaces.id |
| file_id | TEXT | NO | - | FK to files.id |
| name | TEXT | NO | - | Person name |
| role | TEXT | YES | NULL | Role or relationship to content |
| context | TEXT | YES | NULL | Context of mention |
| confidence | REAL | YES | NULL | Extraction confidence (0.0-1.0) |
| created_at | INTEGER | NO | - | Unix milliseconds |

**Indexes:**
- `idx_file_people_file (workspace_id, file_id)` - Find people for a file
- `idx_file_people_name (workspace_id, name)` - Find files mentioning a person

**Unique Constraint:** `(workspace_id, file_id, name)`

---

## file_organizations

**Migration:** v15 (denormalize_ai_context)
**Source:** AIContext.Organizations
**Purpose:** Stores organizations mentioned in documents.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | TEXT | NO | - | UUID primary key |
| workspace_id | TEXT | NO | - | FK to workspaces.id |
| file_id | TEXT | NO | - | FK to files.id |
| name | TEXT | NO | - | Organization name |
| type | TEXT | YES | NULL | company, university, government, nonprofit, etc. |
| context | TEXT | YES | NULL | Context of mention |
| confidence | REAL | YES | NULL | Extraction confidence (0.0-1.0) |
| created_at | INTEGER | NO | - | Unix milliseconds |

**Indexes:**
- `idx_file_organizations_file (workspace_id, file_id)` - Find organizations for a file

**Unique Constraint:** `(workspace_id, file_id, name)`

---

## file_events

**Migration:** v15 (denormalize_ai_context)
**Source:** AIContext.HistoricalEvents
**Purpose:** Stores historical or significant events mentioned in documents.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | TEXT | NO | - | UUID primary key |
| workspace_id | TEXT | NO | - | FK to workspaces.id |
| file_id | TEXT | NO | - | FK to files.id |
| name | TEXT | NO | - | Event name or description |
| date | TEXT | YES | NULL | Date or date range (free text) |
| location | TEXT | YES | NULL | Where event occurred |
| context | TEXT | YES | NULL | How event is referenced |
| confidence | REAL | YES | NULL | Extraction confidence (0.0-1.0) |
| created_at | INTEGER | NO | - | Unix milliseconds |

**Indexes:**
- `idx_file_events_file (workspace_id, file_id)` - Find events for a file

**Note:** No unique constraint - same event may be mentioned multiple times.

---

## file_references

**Migration:** v15 (denormalize_ai_context)
**Source:** AIContext.References
**Purpose:** Stores bibliographic references found in documents.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | TEXT | NO | - | UUID primary key |
| workspace_id | TEXT | NO | - | FK to workspaces.id |
| file_id | TEXT | NO | - | FK to files.id |
| title | TEXT | YES | NULL | Reference title |
| author | TEXT | YES | NULL | Reference author(s) |
| year | TEXT | YES | NULL | Publication year |
| type | TEXT | YES | NULL | book, article, website, etc. |
| doi | TEXT | YES | NULL | Digital Object Identifier |
| url | TEXT | YES | NULL | URL if available |
| created_at | INTEGER | NO | - | Unix milliseconds |

**Indexes:**
- `idx_file_references_file (workspace_id, file_id)` - Find references for a file

---

## file_publication_info

**Migration:** v15 (denormalize_ai_context)
**Source:** AIContext.Publication*
**Purpose:** Stores publication metadata for documents (books, articles, etc.).

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | TEXT | NO | - | UUID primary key |
| workspace_id | TEXT | NO | - | FK to workspaces.id |
| file_id | TEXT | NO | - | FK to files.id |
| publisher | TEXT | YES | NULL | Publisher name |
| publication_year | TEXT | YES | NULL | Year of publication |
| publication_place | TEXT | YES | NULL | City/location of publication |
| isbn | TEXT | YES | NULL | ISBN for books |
| issn | TEXT | YES | NULL | ISSN for periodicals |
| doi | TEXT | YES | NULL | Digital Object Identifier |
| created_at | INTEGER | NO | - | Unix milliseconds |

**Unique Constraint:** `(workspace_id, file_id)` - One publication info per file.

---

## file_named_entities

**Migration:** v16 (denormalize_enrichment)
**Source:** EnrichmentData.NamedEntities
**Purpose:** Stores Named Entity Recognition (NER) results.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | TEXT | NO | - | UUID primary key |
| workspace_id | TEXT | NO | - | FK to workspaces.id |
| file_id | TEXT | NO | - | FK to files.id |
| text | TEXT | NO | - | Entity text |
| type | TEXT | NO | - | PERSON, LOCATION, ORG, DATE, MONEY, PRODUCT, etc. |
| start_pos | INTEGER | YES | NULL | Start position in text |
| end_pos | INTEGER | YES | NULL | End position in text |
| confidence | REAL | YES | NULL | NER confidence (0.0-1.0) |
| context | TEXT | YES | NULL | Surrounding context |
| created_at | INTEGER | NO | - | Unix milliseconds |

**Indexes:**
- `idx_file_named_entities_file (workspace_id, file_id)` - Find entities for a file
- `idx_file_named_entities_type (workspace_id, type)` - Find files by entity type

**Entity Types:**
| Type | Description |
|------|-------------|
| PERSON | People names |
| LOCATION | Places, addresses |
| ORG | Organizations, companies |
| DATE | Dates and times |
| MONEY | Currency amounts |
| PRODUCT | Products, services |
| EVENT | Events |
| WORK_OF_ART | Books, songs, paintings |
| LAW | Laws, regulations |
| LANGUAGE | Languages |

---

## file_citations

**Migration:** v16 (denormalize_enrichment)
**Source:** EnrichmentData.Citations
**Purpose:** Stores extracted citations from documents.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | TEXT | NO | - | UUID primary key |
| workspace_id | TEXT | NO | - | FK to workspaces.id |
| file_id | TEXT | NO | - | FK to files.id |
| text | TEXT | NO | - | Full citation text |
| authors | TEXT | YES | NULL | Author(s) |
| title | TEXT | YES | NULL | Cited work title |
| year | TEXT | YES | NULL | Publication year |
| doi | TEXT | YES | NULL | DOI if available |
| url | TEXT | YES | NULL | URL if available |
| type | TEXT | YES | NULL | book, article, website, etc. |
| confidence | REAL | YES | NULL | Extraction confidence (0.0-1.0) |
| page | INTEGER | YES | NULL | Page number in source |
| created_at | INTEGER | NO | - | Unix milliseconds |

**Indexes:**
- `idx_file_citations_file (workspace_id, file_id)` - Find citations for a file

---

## file_dependencies

**Migration:** v16 (denormalize_enrichment)
**Source:** EnrichmentData.Dependencies
**Purpose:** Stores code dependencies extracted from source files.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | TEXT | NO | - | UUID primary key |
| workspace_id | TEXT | NO | - | FK to workspaces.id |
| file_id | TEXT | NO | - | FK to files.id |
| name | TEXT | NO | - | Dependency name (e.g., "react") |
| version | TEXT | YES | NULL | Version constraint (e.g., "^18.0.0") |
| type | TEXT | YES | NULL | import, require, include, link |
| language | TEXT | YES | NULL | Programming language |
| path | TEXT | YES | NULL | Import path if applicable |
| created_at | INTEGER | NO | - | Unix milliseconds |

**Indexes:**
- `idx_file_dependencies_file (workspace_id, file_id)` - Find dependencies for a file
- `idx_file_dependencies_name (workspace_id, name)` - Find files using a dependency

**Unique Constraint:** `(workspace_id, file_id, name, type)`

---

## file_duplicates

**Migration:** v16 (denormalize_enrichment)
**Source:** EnrichmentData.Duplicates
**Purpose:** Stores duplicate file detection results.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | TEXT | NO | - | UUID primary key |
| workspace_id | TEXT | NO | - | FK to workspaces.id |
| file_id | TEXT | NO | - | FK to files.id (source) |
| duplicate_file_id | TEXT | NO | - | FK to files.id (duplicate) |
| similarity | REAL | NO | - | Similarity score (0.0-1.0) |
| type | TEXT | YES | NULL | exact, near, version, template |
| reason | TEXT | YES | NULL | Why considered duplicate |
| created_at | INTEGER | NO | - | Unix milliseconds |

**Indexes:**
- `idx_file_duplicates_file (workspace_id, file_id)` - Find duplicates for a file

**Unique Constraint:** `(workspace_id, file_id, duplicate_file_id)`

**Duplicate Types:**
| Type | Description |
|------|-------------|
| exact | Byte-for-byte identical |
| near | High similarity (>95%) |
| version | Different versions of same file |
| template | Based on same template |

---

## file_sentiment

**Migration:** v16 (denormalize_enrichment)
**Source:** EnrichmentData.Sentiment
**Purpose:** Stores sentiment analysis results.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | TEXT | NO | - | UUID primary key |
| workspace_id | TEXT | NO | - | FK to workspaces.id |
| file_id | TEXT | NO | - | FK to files.id |
| overall_sentiment | TEXT | NO | - | positive, negative, neutral, mixed |
| score | REAL | NO | - | Sentiment score (-1.0 to 1.0) |
| confidence | REAL | YES | NULL | Analysis confidence (0.0-1.0) |
| emotions_json | TEXT | YES | NULL | JSON map of emotion scores |
| created_at | INTEGER | NO | - | Unix milliseconds |

**Indexes:**
- `idx_file_sentiment_file (workspace_id, file_id)` - Find sentiment for a file

**Unique Constraint:** `(workspace_id, file_id)` - One sentiment per file.

**Emotion Map Example:**
```json
{
  "joy": 0.7,
  "anger": 0.1,
  "sadness": 0.05,
  "fear": 0.02,
  "surprise": 0.13
}
```

---

## file_relationships

**Migration:** v18 (file_relationships)
**Source:** Code analysis
**Purpose:** Stores code import/export relationships between files.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | TEXT | NO | - | UUID primary key |
| workspace_id | TEXT | NO | - | FK to workspaces.id |
| from_file_id | TEXT | NO | - | FK to files.id (importer) |
| to_file_id | TEXT | NO | - | FK to files.id (imported) |
| type | TEXT | NO | - | import, export, include, require, reference |
| language | TEXT | YES | NULL | Programming language |
| confidence | REAL | YES | NULL | Detection confidence (0.0-1.0) |
| created_at | INTEGER | NO | - | Unix milliseconds |

**Indexes:**
- `idx_file_relationships_from (workspace_id, from_file_id)` - Find imports for a file
- `idx_file_relationships_to (workspace_id, to_file_id)` - Find importers of a file
- `idx_file_relationships_type (workspace_id, type)` - Filter by relationship type
- `idx_file_relationships_language (workspace_id, language)` - Filter by language

**Unique Constraint:** `(workspace_id, from_file_id, to_file_id, type)`

---

## file_access_events

**Migration:** v20 (temporal_analysis)
**Source:** OS file system events
**Purpose:** Tracks file access patterns for temporal analysis.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | TEXT | NO | - | UUID primary key |
| workspace_id | TEXT | NO | - | FK to workspaces.id |
| file_id | TEXT | NO | - | FK to files.id |
| event_type | TEXT | NO | - | read, write, open, close |
| timestamp | INTEGER | NO | - | Event timestamp (Unix ms) |
| metadata | TEXT | YES | NULL | JSON event metadata |
| created_at | INTEGER | NO | - | Unix milliseconds |

**Indexes:**
- `idx_access_events_file (workspace_id, file_id, timestamp)` - Find events for a file
- `idx_access_events_type (workspace_id, event_type, timestamp)` - Filter by event type

---

## file_modification_history

**Migration:** v20 (temporal_analysis)
**Source:** File system monitoring
**Purpose:** Tracks file size changes over time.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | TEXT | NO | - | UUID primary key |
| workspace_id | TEXT | NO | - | FK to workspaces.id |
| file_id | TEXT | NO | - | FK to files.id |
| timestamp | INTEGER | NO | - | Modification timestamp (Unix ms) |
| size_before | INTEGER | YES | NULL | Size before change |
| size_after | INTEGER | YES | NULL | Size after change |
| metadata | TEXT | YES | NULL | JSON modification metadata |
| created_at | INTEGER | NO | - | Unix milliseconds |

**Indexes:**
- `idx_modification_history_file (workspace_id, file_id, timestamp)` - Find history for a file

---

## temporal_clusters

**Migration:** v20 (temporal_analysis)
**Source:** Temporal analysis service
**Purpose:** Groups files by editing sessions or work patterns.

| Column | Type | Nullable | Default | Description |
|--------|------|----------|---------|-------------|
| id | TEXT | NO | - | UUID primary key |
| workspace_id | TEXT | NO | - | FK to workspaces.id |
| cluster_id | TEXT | NO | - | Cluster identifier |
| file_ids | TEXT | NO | - | JSON array of file IDs |
| time_window_start | INTEGER | NO | - | Cluster start (Unix ms) |
| time_window_end | INTEGER | NO | - | Cluster end (Unix ms) |
| pattern_type | TEXT | YES | NULL | edit_session, project_work, backup, etc. |
| created_at | INTEGER | NO | - | Unix milliseconds |

**Indexes:**
- `idx_temporal_clusters_workspace (workspace_id, time_window_start, time_window_end)` - Find clusters in time range

**Unique Constraint:** `(workspace_id, cluster_id)`

**Pattern Types:**
| Type | Description |
|------|-------------|
| edit_session | Files edited in same session |
| project_work | Files related to same project work |
| backup | Backup-related modifications |
| batch_import | Bulk file imports |

---

## Confidence Score Guidelines

Confidence scores throughout these tables follow these conventions:

| Range | Meaning |
|-------|---------|
| 0.9-1.0 | High confidence, likely correct |
| 0.7-0.9 | Good confidence, probably correct |
| 0.5-0.7 | Medium confidence, may need review |
| 0.3-0.5 | Low confidence, likely needs review |
| 0.0-0.3 | Very low confidence, may be incorrect |

---

## JSON Column Conventions

JSON columns (ai_context, enrichment_data, emotions_json, metadata, etc.) follow these conventions:

1. **Empty state:** `NULL` or `{}` for no data
2. **Arrays:** Use `[]` for empty arrays
3. **Encoding:** UTF-8
4. **Size limit:** No hard limit, but keep under 1MB for performance
5. **Validation:** Application-level validation before storage
