# Query Catalog: Top 10 Queries Enabled by AI Metadata Schema

This document provides optimized SQL queries for the most common use cases enabled by the AI metadata schema (v13-v23).

---

## 1. Find Files by Author Name

**Use Case:** Locate all documents written by or mentioning a specific author.

```sql
SELECT
    f.relative_path,
    f.filename,
    fa.role,
    fa.affiliation,
    fa.confidence
FROM file_authors fa
JOIN files f ON f.id = fa.file_id AND f.workspace_id = fa.workspace_id
WHERE fa.workspace_id = ?
  AND fa.name LIKE ?
ORDER BY fa.confidence DESC, f.last_modified DESC;
```

**Parameters:**
- `?1` - workspace_id
- `?2` - author name pattern (e.g., '%Smith%')

**Index Used:** `idx_file_authors_name`

**Performance Notes:**
- For exact matches, use `= ?` instead of `LIKE`
- Add `LIMIT` for pagination

---

## 2. Find Duplicate Files by Hash

**Use Case:** Identify exact or near-duplicate files for deduplication.

```sql
-- Find exact duplicates (same SHA-256 hash)
SELECT
    f1.relative_path AS original,
    f2.relative_path AS duplicate,
    f1.file_hash_sha256
FROM files f1
JOIN files f2 ON f1.file_hash_sha256 = f2.file_hash_sha256
             AND f1.id < f2.id  -- Avoid self-join and duplicates
WHERE f1.workspace_id = ?
  AND f1.file_hash_sha256 IS NOT NULL
ORDER BY f1.relative_path;
```

**Alternative: Using file_duplicates table (includes near-duplicates)**

```sql
SELECT
    f1.relative_path AS source_file,
    f2.relative_path AS duplicate_file,
    fd.similarity,
    fd.type,
    fd.reason
FROM file_duplicates fd
JOIN files f1 ON f1.id = fd.file_id AND f1.workspace_id = fd.workspace_id
JOIN files f2 ON f2.id = fd.duplicate_file_id AND f2.workspace_id = fd.workspace_id
WHERE fd.workspace_id = ?
  AND fd.similarity >= ?  -- e.g., 0.95 for high similarity
ORDER BY fd.similarity DESC;
```

**Index Used:** `idx_files_hash_sha256`, `idx_file_duplicates_file`

---

## 3. Find Files Mentioning Specific Organizations

**Use Case:** Find documents related to a company, university, or institution.

```sql
SELECT
    f.relative_path,
    f.filename,
    fo.name AS organization,
    fo.type AS org_type,
    fo.context,
    fo.confidence
FROM file_organizations fo
JOIN files f ON f.id = fo.file_id AND f.workspace_id = fo.workspace_id
WHERE fo.workspace_id = ?
  AND (fo.name LIKE ? OR fo.name = ?)
ORDER BY fo.confidence DESC, f.last_modified DESC
LIMIT 100;
```

**Parameters:**
- `?1` - workspace_id
- `?2` - organization pattern (e.g., '%Google%')
- `?3` - exact organization name

**Index Used:** `idx_file_organizations_file`

---

## 4. Get Citation Network for a File

**Use Case:** Build a citation graph showing references from and to a document.

```sql
-- Outgoing citations (this file cites these works)
SELECT
    'outgoing' AS direction,
    fc.title,
    fc.authors,
    fc.year,
    fc.doi,
    fc.url,
    fc.confidence
FROM file_citations fc
WHERE fc.workspace_id = ?
  AND fc.file_id = ?
ORDER BY fc.year DESC;
```

```sql
-- Incoming citations (find files that cite this document's title)
SELECT
    'incoming' AS direction,
    f.relative_path AS citing_file,
    fc.confidence
FROM file_citations fc
JOIN files f ON f.id = fc.file_id AND f.workspace_id = fc.workspace_id
JOIN file_metadata fm ON fm.file_id = ? AND fm.workspace_id = ?
WHERE fc.workspace_id = ?
  AND (
    fc.title LIKE '%' || fm.ai_summary || '%'
    OR fc.doi = (SELECT doi FROM file_publication_info WHERE file_id = ? AND workspace_id = ?)
  );
```

**Index Used:** `idx_file_citations_file`

---

## 5. Semantic Search with RAG

**Use Case:** Find semantically similar documents using vector embeddings.

```sql
-- First, get the embedding for the query (done in application code)
-- Then find similar chunks

SELECT
    d.relative_path,
    c.heading_path,
    c.text AS snippet,
    -- Vector similarity calculated in application layer
    1.0 AS score  -- Placeholder; actual score from vector search
FROM chunks c
JOIN documents d ON d.id = c.document_id AND d.workspace_id = c.workspace_id
WHERE c.workspace_id = ?
  AND c.id IN (
    -- IDs returned from vector similarity search
    SELECT chunk_id FROM chunk_embeddings
    WHERE workspace_id = ?
    -- Vector similarity done in application layer
  )
ORDER BY score DESC
LIMIT 10;
```

**Note:** Vector similarity search is typically done in the application layer using the embedding vectors, then joined with SQL for metadata.

---

## 6. Track File Access Patterns

**Use Case:** Analyze file access patterns for a time period.

```sql
-- Most accessed files in last 7 days
SELECT
    f.relative_path,
    COUNT(*) AS access_count,
    MAX(fae.timestamp) AS last_access,
    MIN(fae.timestamp) AS first_access
FROM file_access_events fae
JOIN files f ON f.id = fae.file_id AND f.workspace_id = fae.workspace_id
WHERE fae.workspace_id = ?
  AND fae.timestamp >= ?  -- Start timestamp (e.g., 7 days ago)
  AND fae.timestamp <= ?  -- End timestamp (now)
GROUP BY f.relative_path
ORDER BY access_count DESC
LIMIT 20;
```

**Index Used:** `idx_access_events_file`

---

## 7. Identify Temporal Editing Clusters

**Use Case:** Find files edited together in the same work session.

```sql
-- Get clusters for a time window
SELECT
    tc.cluster_id,
    tc.pattern_type,
    tc.time_window_start,
    tc.time_window_end,
    tc.file_ids
FROM temporal_clusters tc
WHERE tc.workspace_id = ?
  AND tc.time_window_start >= ?
  AND tc.time_window_end <= ?
ORDER BY tc.time_window_start DESC;
```

```sql
-- Expand cluster to show file details
SELECT
    f.relative_path,
    f.filename,
    f.last_modified
FROM files f
WHERE f.workspace_id = ?
  AND f.id IN (
    SELECT value FROM json_each(
      (SELECT file_ids FROM temporal_clusters WHERE cluster_id = ? AND workspace_id = ?)
    )
  )
ORDER BY f.last_modified;
```

**Index Used:** `idx_temporal_clusters_workspace`

---

## 8. Get Sentiment Distribution Across Projects

**Use Case:** Analyze emotional tone across project documents.

```sql
SELECT
    fc.context AS project,
    fs.overall_sentiment,
    COUNT(*) AS file_count,
    AVG(fs.score) AS avg_sentiment_score,
    AVG(fs.confidence) AS avg_confidence
FROM file_sentiment fs
JOIN file_contexts fc ON fc.file_id = fs.file_id AND fc.workspace_id = fs.workspace_id
WHERE fs.workspace_id = ?
GROUP BY fc.context, fs.overall_sentiment
ORDER BY fc.context, file_count DESC;
```

**Alternative: Sentiment histogram**

```sql
SELECT
    CASE
        WHEN score < -0.6 THEN 'very_negative'
        WHEN score < -0.2 THEN 'negative'
        WHEN score < 0.2 THEN 'neutral'
        WHEN score < 0.6 THEN 'positive'
        ELSE 'very_positive'
    END AS sentiment_bucket,
    COUNT(*) AS count
FROM file_sentiment
WHERE workspace_id = ?
GROUP BY sentiment_bucket
ORDER BY
    CASE sentiment_bucket
        WHEN 'very_negative' THEN 1
        WHEN 'negative' THEN 2
        WHEN 'neutral' THEN 3
        WHEN 'positive' THEN 4
        WHEN 'very_positive' THEN 5
    END;
```

**Index Used:** `idx_file_sentiment_file`

---

## 9. Find Files with Specific Dependencies

**Use Case:** Locate all files using a particular library or package.

```sql
SELECT
    f.relative_path,
    f.filename,
    fd.name AS dependency,
    fd.version,
    fd.type,
    fd.language
FROM file_dependencies fd
JOIN files f ON f.id = fd.file_id AND f.workspace_id = fd.workspace_id
WHERE fd.workspace_id = ?
  AND fd.name = ?
ORDER BY fd.version DESC, f.relative_path;
```

**Parameters:**
- `?1` - workspace_id
- `?2` - dependency name (e.g., "react", "lodash")

**Find files with outdated dependencies:**

```sql
SELECT
    f.relative_path,
    fd.name,
    fd.version AS current_version,
    ? AS latest_version  -- Provided by application
FROM file_dependencies fd
JOIN files f ON f.id = fd.file_id AND f.workspace_id = fd.workspace_id
WHERE fd.workspace_id = ?
  AND fd.name = ?
  AND fd.version != ?  -- Latest version
ORDER BY f.relative_path;
```

**Index Used:** `idx_file_dependencies_name`

---

## 10. List Files by Publication Date Range

**Use Case:** Find documents published within a specific time period.

```sql
SELECT
    f.relative_path,
    f.filename,
    fpi.publisher,
    fpi.publication_year,
    fpi.isbn,
    fpi.doi
FROM file_publication_info fpi
JOIN files f ON f.id = fpi.file_id AND f.workspace_id = fpi.workspace_id
WHERE fpi.workspace_id = ?
  AND CAST(fpi.publication_year AS INTEGER) >= ?
  AND CAST(fpi.publication_year AS INTEGER) <= ?
ORDER BY fpi.publication_year DESC, f.relative_path;
```

**Parameters:**
- `?1` - workspace_id
- `?2` - start year (e.g., 2020)
- `?3` - end year (e.g., 2024)

---

## Bonus Queries

### Find Import/Export Relationships

```sql
-- Files that import a specific file
SELECT
    f_from.relative_path AS importing_file,
    f_to.relative_path AS imported_file,
    fr.type,
    fr.language
FROM file_relationships fr
JOIN files f_from ON f_from.id = fr.from_file_id AND f_from.workspace_id = fr.workspace_id
JOIN files f_to ON f_to.id = fr.to_file_id AND f_to.workspace_id = fr.workspace_id
WHERE fr.workspace_id = ?
  AND fr.to_file_id = ?  -- The file being imported
ORDER BY f_from.relative_path;
```

### Get Named Entities by Type

```sql
SELECT
    fne.text AS entity,
    COUNT(DISTINCT fne.file_id) AS file_count,
    AVG(fne.confidence) AS avg_confidence
FROM file_named_entities fne
WHERE fne.workspace_id = ?
  AND fne.type = ?  -- e.g., 'PERSON', 'ORG'
GROUP BY fne.text
ORDER BY file_count DESC, avg_confidence DESC
LIMIT 50;
```

### Files by Geographic Region

```sql
SELECT
    fl.name AS location,
    fl.type AS location_type,
    COUNT(DISTINCT fl.file_id) AS file_count,
    GROUP_CONCAT(DISTINCT f.filename) AS sample_files
FROM file_locations fl
JOIN files f ON f.id = fl.file_id AND f.workspace_id = fl.workspace_id
WHERE fl.workspace_id = ?
  AND (fl.name LIKE '%Spain%' OR fl.name LIKE '%Madrid%')  -- Example filter
GROUP BY fl.name, fl.type
ORDER BY file_count DESC;
```

---

## Query Performance Guidelines

1. **Always filter by workspace_id first** - All indexes include workspace_id as the leading column.

2. **Use LIMIT for large result sets** - Especially for exploratory queries.

3. **Prefer exact matches over LIKE** - When possible, use `=` instead of `LIKE '%...%'`.

4. **Use covering indexes** - When querying only indexed columns, SQLite can satisfy the query from the index alone.

5. **Avoid JSON extraction in WHERE** - For JSON columns, filter in the application layer if possible.

6. **Batch operations** - For bulk inserts/updates, use transactions.

7. **Analyze query plans** - Use `EXPLAIN QUERY PLAN` to verify index usage:
   ```sql
   EXPLAIN QUERY PLAN
   SELECT ...
   ```
