# Facets Implementation Summary

This document summarizes the facets that have been implemented in the Cortex backend and their organization.

## Facet Organization

All facets are now organized using a centralized `FacetRegistry` that:
- Defines canonical names for each facet
- Supports aliases for backward compatibility
- Categorizes facets by type (Core, Organization, Temporal, Content, System, Specialized)
- Provides metadata about each facet (description, data source, availability)

See `FACETS_ORGANIZATION.md` for the complete reorganization plan.

## Newly Implemented Facets

### 1. Language Facet
- **Field**: `language`, `detected_language`
- **Type**: Terms
- **Source**: `file_metadata.detected_language`
- **Description**: Groups files by detected language (e.g., "es", "en", "fr")
- **Implementation**: `MetadataRepository.GetLanguageFacet()`

### 2. AI Category Facet
- **Field**: `ai_category`, `category`
- **Type**: Terms
- **Source**: `file_metadata.ai_category`
- **Description**: Groups files by AI-assigned category
- **Implementation**: `MetadataRepository.GetAICategoryFacet()`

### 3. Author Facet
- **Field**: `author`, `authors`
- **Type**: Terms
- **Source**: `file_authors` table (denormalized from AI context)
- **Description**: Groups files by extracted authors
- **Implementation**: `MetadataRepository.GetAuthorFacet()`

### 4. Publication Year Facet
- **Field**: `publication_year`, `year`
- **Type**: Terms
- **Source**: `file_publication_info.publication_year`
- **Description**: Groups files by publication year
- **Implementation**: `MetadataRepository.GetPublicationYearFacet()`

### 5. Owner Facet
- **Field**: `owner`, `owners`
- **Type**: Terms
- **Source**: `file_ownership` table joined with `system_users`
- **Description**: Groups files by owner (username) and ownership type
- **Implementation**: `FileRepository.GetOwnerFacet()`
- **Notes**: Returns both ownership types (owner, group_member, other) and specific usernames

### 6. Indexing Status Facet
- **Field**: `indexing_status`, `index_status`
- **Type**: Terms
- **Source**: `files` table indexing flags
- **Description**: Groups files by indexing completion status:
  - `complete`: All stages indexed (basic, mime, code, document, mirror)
  - `document_complete`: Up to document stage
  - `code_complete`: Up to code stage
  - `mime_complete`: Up to MIME stage
  - `basic_only`: Only basic indexing
  - `not_indexed`: Not indexed
- **Implementation**: `FileRepository.GetIndexingStatusFacet()`

### 7. Accessed Date Range Facet
- **Field**: `accessed_at`, `accessed`
- **Type**: Date Range
- **Source**: `files.accessed_at`
- **Description**: Groups files by last access date ranges (Today, Yesterday, This Week, This Month, This Year, Older)
- **Implementation**: `FileRepository.GetAccessedDateRangeFacet()`

### 8. Changed Date Range Facet
- **Field**: `changed_at`, `changed`
- **Type**: Date Range
- **Source**: `files.changed_at` (ctime - metadata change time)
- **Description**: Groups files by metadata change date ranges
- **Implementation**: `FileRepository.GetChangedDateRangeFacet()`

## Implementation Details

### Repository Interface Extensions

#### FileRepository
Added methods:
- `GetAccessedDateRangeFacet()`
- `GetChangedDateRangeFacet()`
- `GetOwnerFacet()`
- `GetIndexingStatusFacet()`

#### MetadataRepository
Added methods:
- `GetLanguageFacet()`
- `GetAICategoryFacet()`
- `GetAuthorFacet()`
- `GetPublicationYearFacet()`

### SQLite Implementation

All methods are implemented in:
- `backend/internal/infrastructure/persistence/sqlite/file_repository.go`
- `backend/internal/infrastructure/persistence/sqlite/metadata_repository.go`

### FacetExecutor Updates

The `FacetExecutor` in `backend/internal/application/query/facets.go` now supports:
- All new term facets via `executeTermsFacet()`
- New date range facets via `executeDateRangeFacet()`

## Usage Examples

### Query Language Facet
```go
facetReq := query.FacetRequest{
    Field: "language",
    Type:  query.FacetTypeTerms,
}
result, err := facetExecutor.ExecuteFacet(ctx, workspaceID, facetReq, nil)
```

### Query Author Facet
```go
facetReq := query.FacetRequest{
    Field: "author",
    Type:  query.FacetTypeTerms,
}
result, err := facetExecutor.ExecuteFacet(ctx, workspaceID, facetReq, nil)
```

### Query Accessed Date Range Facet
```go
facetReq := query.FacetRequest{
    Field: "accessed_at",
    Type:  query.FacetTypeDateRange,
}
result, err := facetExecutor.ExecuteFacet(ctx, workspaceID, facetReq, nil)
```

## Data Availability

### Always Available
- **Extension** - All files have extensions
- **Type** - All files have inferred types
- **Size** - All files have sizes
- **Last Modified** - All files have modification dates
- **Indexing Status** - All files have indexing state

### Conditionally Available
- **Language** - Only files with AI language detection
- **AI Category** - Only files with AI category assignment
- **Author** - Only files with extracted authors (from AI context)
- **Publication Year** - Only files with publication metadata
- **Owner** - Only files with OS ownership data
- **Accessed/Changed Dates** - Only files with these timestamps (OS-dependent)

## Performance Considerations

1. **Denormalized Tables**: Author and publication data use denormalized tables (`file_authors`, `file_publication_info`) for efficient querying
2. **Indexes**: All facet queries use existing indexes on workspace_id and relevant columns
3. **NULL Handling**: Queries handle NULL values gracefully, grouping them as "unknown" or "uncategorized"
4. **Filtering**: All facets support optional fileID filtering for scoped queries

## Future Enhancements

Potential additional facets to implement:
- **By Location** - Geographic locations
- **By Organization** - Organizations mentioned
- **By Sentiment** - Sentiment analysis results
- **By Code Complexity** - Code complexity ranges
- **By Duplicate Type** - Duplicate relationships
- **By Project Assignment Score** - AI confidence scores
- **By Temporal Pattern** - Access/modification patterns

See `FACETS_ANALYSIS.md` for the complete list of 50+ potential facets.

