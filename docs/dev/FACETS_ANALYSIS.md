# Facets Analysis - Available Data & Potential Views

This document analyzes all available data in the Cortex backend database and identifies potential facets/views that could be built.

## Database Schema Overview

The Cortex backend has **26 migrations** creating a rich, multi-dimensional data model with:

- **Core file indexing** (files, metadata, tags, contexts)
- **AI/LLM enrichment** (summaries, suggestions, taxonomy, context extraction)
- **Document processing** (chunks, embeddings, relationships, states)
- **OS-level metadata** (permissions, ownership, ACLs, timestamps)
- **Enrichment data** (NER, citations, sentiment, dependencies, duplicates)
- **Temporal analysis** (access events, modification history, clusters)
- **Project management** (hierarchical projects, assignments, memberships)
- **User/Person tracking** (system users, persons, ownership, access)
- **Quality metrics** (benchmarks, model usage, extraction events)

## Available Data Fields

### Core File Data (`files` table)
- `id`, `workspace_id`, `relative_path`, `absolute_path`
- `filename`, `extension`
- `file_size`, `last_modified`, `created_at`
- `accessed_at`, `changed_at`, `backup_at`
- `file_hash_md5`, `file_hash_sha256`, `file_hash_sha512`
- `path_components` (JSON), `path_pattern`
- `os_metadata` (JSON), `os_taxonomy` (JSON)
- `enhanced` (JSON - EnhancedMetadata)
- `indexed_basic`, `indexed_mime`, `indexed_code`, `indexed_document`, `indexed_mirror`

### File Metadata (`file_metadata` table)
- `file_id`, `workspace_id`, `relative_path`, `type`
- `notes`, `ai_summary`, `ai_summary_hash`, `ai_key_terms`
- `ai_category`, `ai_category_confidence`, `ai_category_updated_at`
- `ai_related` (JSON), `ai_context` (JSON), `enrichment_data` (JSON)
- `detected_language`
- `mirror_format`, `mirror_path`, `mirror_source_mtime`, `mirror_updated_at`
- `created_at`, `updated_at`

### Tags & Contexts
- `file_tags` - User-assigned tags
- `file_contexts` - User-assigned projects/contexts
- `file_context_suggestions` - AI-suggested contexts
- `suggested_tags` - AI-suggested tags (with confidence, reason, category, source)
- `suggested_projects` - AI-suggested projects (with confidence, reason, source, is_new)

### AI Taxonomy (`suggested_taxonomy` table)
- `category`, `subcategory`, `domain`, `subdomain`
- `content_type`, `purpose`, `audience`, `language`
- `category_confidence`, `domain_confidence`, `content_type_confidence`
- `reasoning`, `source`
- `suggested_taxonomy_topics` - Many-to-many topics

### AI Context (denormalized from `ai_context` JSON)
- `file_authors` - Authors (name, role, affiliation, confidence)
- `file_locations` - Locations (name, type, coordinates, context)
- `file_people` - People mentioned (name, role, context, confidence)
- `file_organizations` - Organizations (name, type, context, confidence)
- `file_events` - Historical events (name, date, location, context, confidence)
- `file_references` - Bibliographic references (title, author, year, type, DOI, URL)
- `file_publication_info` - Publication metadata (publisher, year, place, ISBN, ISSN, DOI)

### Enrichment Data (denormalized from `enrichment_data` JSON)
- `file_named_entities` - NER entities (text, type, start_pos, end_pos, confidence, context)
- `file_citations` - Citations (text, authors, title, year, DOI, URL, type, confidence, page)
- `file_dependencies` - Code dependencies (name, version, type, language, path)
- `file_duplicates` - Duplicate relationships (duplicate_file_id, similarity, type, reason)
- `file_sentiment` - Sentiment analysis (overall_sentiment, score, confidence, emotions_json)

### Code Relationships
- `file_relationships` - Import/export relationships (from_file_id, to_file_id, type, language, confidence)

### OS Metadata (from `os_metadata` JSON)
- Permissions (octal, string, owner/group/other read/write/execute, setuid, setgid, sticky)
- Owner (UID, username, full_name, home_dir, shell)
- Group (GID, group_name, members)
- File attributes (readonly, hidden, system, archive, compressed, encrypted, immutable, append_only, no_dump)
- ACLs (type, identity, permissions, flags)
- Timestamps (created, modified, accessed, changed, backup)
- File system (mount_point, device_id, filesystem_type, block_size, blocks, selinux_context)

### OS Taxonomy (from `os_taxonomy` JSON)
- Security taxonomy (permission_level, security_category, security_attributes, has_acls, acl_complexity)
- Ownership taxonomy (owner_type, group_category, access_relations, ownership_pattern)
- Temporal taxonomy (temporal_pattern, access_frequency, time_category, temporal_relations)
- System taxonomy (system_file_type, filesystem_category, system_attributes, system_features)
- Organization taxonomy (user_grouping, group_grouping, project_grouping, org_patterns)

### Enhanced Metadata (from `enhanced` JSON)
- `FileStats`: size, created, modified, accessed, changed, backup, is_readonly, is_hidden
- `Folder`, `Depth`, `PathComponents`, `PathPattern`
- `Language`, `LanguageConfidence`, `ContentEncoding`
- `ContentStructure`: headings, lists, TOC, cross-refs, footnotes, sections
- `CodeImports`: import/export relationships
- `FileHash`: MD5, SHA256, SHA512
- `MetadataConsistency`: score, issues, warnings
- `TemporalMetrics`: age, staleness, access_frequency, modification_frequency, last_access_pattern
- `ContentQuality`: readability_score, complexity_score, quality_score, readability_level
- `MimeTypeInfo`: mime_type, category, encoding
- `CodeMetrics`: lines_of_code, comment_lines, blank_lines, comment_percentage, function_count, class_count, complexity, imports, exports
- `DocumentMetrics`: page_count, word_count, character_count, author, title, subject, keywords, creator, producer, dates, PDF metadata, XMP metadata, fonts, colors, etc.
- `ImageMetadata`: dimensions, color_depth, color_space, format, orientation, EXIF, GPS, IPTC, XMP
- `AudioMetadata`: duration, bitrate, sample_rate, channels, bit_depth, codec, format, ID3 tags, Vorbis comments, album art
- `VideoMetadata`: duration, dimensions, frame_rate, bitrate, codec, container, video/audio streams, metadata tags, technical metadata

### Temporal Data
- `file_access_events` - Access events (event_type, timestamp, metadata)
- `file_modification_history` - Modification history (timestamp, size_before, size_after, metadata)
- `temporal_clusters` - Time-based clusters (cluster_id, file_ids, time_window_start, time_window_end, pattern_type)

### Projects
- `projects` - Hierarchical projects (id, name, description, parent_id, path, nature, attributes, created_at, updated_at)
- `project_documents` - Project-document relationships (project_id, document_id, role)
- `project_assignments` - Project assignments (file_id, project_id, project_name, score, sources, status)
- `project_memberships` - Project memberships (project_id, person_id, role, joined_at)

### Documents & RAG
- `documents` - Document entities (id, file_id, relative_path, title, frontmatter, checksum, state, state_changed_at)
- `chunks` - Document chunks (id, document_id, ordinal, heading, heading_path, text, token_count, start_line, end_line)
- `chunk_embeddings` - Vector embeddings (chunk_id, dimensions, vector)
- `document_relationships` - Document relationships (from_document_id, to_document_id, type, strength, metadata, confidence, discovery_method)
- `document_usage_events` - Document usage (document_id, event_type, context, metadata, timestamp)
- `document_state_history` - State changes (document_id, from_state, to_state, reason, changed_by, changed_at)

### Users & Persons
- `persons` - Human identities (id, name, email, display_name, notes)
- `system_users` - OS user accounts (id, person_id, username, uid, full_name, home_dir, shell, is_system)
- `file_ownership` - File ownership (file_id, user_id, ownership_type, permissions, detected_at)
- `file_access` - File access (file_id, user_id, access_type, source, detected_at)

### Quality & Observability
- `benchmark_results` - AI quality metrics (test_suite, metric_type, precision, recall, f1_score, accuracy, etc.)
- `model_usage` - LLM usage tracking (model_id, provider, operation, tokens, cost, latency, success)
- `extraction_events` - Extraction events (file_id, stage, event_type, error_type, error_message, items_extracted, confidence, duration_ms)
- `file_traces` - File processing traces (stage, operation, prompt_path, output_path, model, tokens_used, duration_ms, error)

## Potential Facets/Views

Based on the available data, here are **50+ potential facets** organized as a deduplicated tree:

### Core File Attributes
- ✅ **By Extension** - Group by file extension
- ✅ **By Type** - Group by inferred file type
- ✅ **By Size** - Group by file size ranges
- ✅ **By Modified Date** - Group by modification date ranges (implemented as `modified`)
- ✅ **By Tag** - Group by user-assigned tags
- ✅ **By Project/Context** - Group by assigned projects
- ✅ **By Folder** - Group by folder hierarchy
- ✅ **By Indexing Status** - Group by indexing completion status

### Temporal
- ✅ **By Creation Date** - Group by file creation date (implemented as `created`)
- ✅ **By Access Date** - Group by last access date (implemented as `accessed`)
- ✅ **By Changed Date** - Group by metadata change date (ctime) (implemented as `changed`)
- **By Backup Date** - Group by backup timestamp
- **By Age** - Group by file age (time since creation)
- **By Staleness** - Group by time since last modification
- **By Access Frequency** - Group by access frequency patterns (frequent, occasional, rare, never)
- **By Modification Frequency** - Group by modification frequency
- ✅ **By Temporal Pattern** - Group by temporal patterns (recent, occasional, rare, never) (implemented as `temporal_pattern`)
- **By Time Category** - Group by time categories (created_recently, modified_this_week, accessed_today)
- **By Temporal Cluster** - Group by temporal clustering (edit_session, project_work, backup)

### Language & Content
- ✅ **By Detected Language** - Group by LLM-detected language (es, en, fr, etc.) (implemented as `language`)
- **By Language Confidence** - Group by language detection confidence
- **By Content Encoding** - Group by content encoding (UTF-8, Latin-1, etc.)
- ✅ **By Content Type** - Group by AI-suggested content type (implemented as `content_type`)
- ✅ **By Purpose** - Group by AI-suggested purpose (implemented as `purpose`)
- ✅ **By Audience** - Group by AI-suggested audience (implemented as `audience`)
- ✅ **By Readability Level** - Group by readability level (elementary, high_school, college, graduate, professional) (implemented as `readability_level`)
- **By Content Quality** - Group by content quality score ranges

### AI & Taxonomy
- ✅ **By AI Category** - Group by AI-assigned category (implemented as `category`)
- **By AI Category Confidence** - Group by category confidence ranges
- **By Taxonomy Category** - Group by AI taxonomy category
- **By Taxonomy Domain** - Group by AI taxonomy domain
- **By Taxonomy Subdomain** - Group by AI taxonomy subdomain
- **By Taxonomy Topic** - Group by AI-suggested topics
- **By AI Summary Status** - Group by whether AI summary exists
- **By AI Context Status** - Group by whether AI context exists
- **By Enrichment Status** - Group by whether enrichment data exists
- **By Suggestion Source** - Group by suggestion source (llm, rag_llm, etc.)
- **By Suggestion Confidence** - Group by suggestion confidence ranges

### People & Roles
- ✅ **By Author** - Group by extracted authors (implemented as `author`)
- **By Author Role** - Group by author role (author, co-author, contributor)
- **By Author Affiliation** - Group by author affiliation/institution
- **By People Mentioned** - Group by people mentioned in documents
- **By Person Role** - Group by role of mentioned people (saint, scientist, politician, etc.)
- **By Editor** - Group by editors
- **By Translator** - Group by translators
- **By Contributor** - Group by contributors

### Geography
- ✅ **By Location** - Group by extracted locations (implemented as `location`)
- **By Location Type** - Group by location type (city, country, region, continent)
- **By Region** - Group by geographic regions
- **By GPS Coordinate Range** - Group by GPS coordinate ranges (for images)

### Organizations & Publication
- ✅ **By Organization** - Group by mentioned organizations (implemented as `organization`)
- **By Organization Type** - Group by organization type (church, university, government, etc.)
- **By Publisher** - Group by publisher
- **By Publication Place** - Group by publication place
- ✅ **By Publication Year** - Group by publication year (implemented as `publication_year`)
- **By Publication Decade** - Group by publication decade
- **By ISBN** - Group by ISBN (for books)
- **By ISSN** - Group by ISSN (for journals)
- **By DOI** - Group by DOI
- **By Edition** - Group by edition
- **By Genre** - Group by literary genre
- **By Subject** - Group by subject matter

### Events & References
- **By Historical Event** - Group by mentioned historical events
- **By Event Date** - Group by event dates
- **By Reference Type** - Group by reference type (book, article, website, conference)
- **By Reference Author** - Group by reference authors
- **By Reference Year** - Group by reference publication years

### Code
- **By Programming Language** - Group by detected programming language
- ✅ **By Code Complexity** - Group by code complexity ranges (implemented as `complexity`)
- ✅ **By Function Count** - Group by function count ranges (implemented as `function_count`)
- **By Class Count** - Group by class count ranges
- ✅ **By Lines of Code** - Group by LOC ranges (implemented as `lines_of_code`)
- ✅ **By Comment Percentage** - Group by comment percentage ranges (implemented as `comment_percentage`)
- **By Import Count** - Group by number of imports
- **By Export Count** - Group by number of exports
- **By Dependency** - Group by code dependencies
- **By Dependency Type** - Group by dependency type (import, require, include)
- **By Dependency Language** - Group by dependency language
- **By Code Relationship Type** - Group by code relationship type (import, export, include, require, reference)

### Documents
- **By Page Count** - Group by document page count ranges
- **By Word Count** - Group by word count ranges
- **By Character Count** - Group by character count ranges
- **By Document State** - Group by document state (draft, published, archived, etc.)
- **By PDF Version** - Group by PDF version
- **By PDF Encryption** - Group by PDF encryption status
- **By PDF Tagged** - Group by PDF tagged status
- **By Document Font** - Group by fonts used in documents
- **By Document Color Space** - Group by color spaces used in documents
- **By Document Image Count** - Group by number of images in documents
- **By Form Field Count** - Group by number of form fields
- **By Annotation Count** - Group by number of annotations

### Images
- **By Image Dimensions** - Group by image size ranges (width x height)
- **By Image Format** - Group by image format (JPEG, PNG, TIFF, etc.)
- **By Image Color Depth** - Group by color depth (bits per pixel)
- **By Image Color Space** - Group by color space (RGB, CMYK, Grayscale)
- **By Camera Make** - Group by camera manufacturer
- **By Camera Model** - Group by camera model
- **By ISO** - Group by ISO settings
- **By Aperture** - Group by f-stop ranges
- **By Focal Length** - Group by focal length ranges
- **By Image GPS Location** - Group by GPS coordinates (for geotagged images)
- **By Orientation** - Group by EXIF orientation
- **By Has Transparency** - Group by transparency support
- **By Is Animated** - Group by animation support (GIFs)

### Audio
- **By Audio Duration** - Group by audio duration ranges
- **By Audio Bitrate** - Group by bitrate ranges
- **By Audio Sample Rate** - Group by sample rate ranges
- **By Audio Channels** - Group by channel count (mono, stereo, etc.)
- **By Audio Codec** - Group by audio codec (MP3, FLAC, AAC, etc.)
- **By Audio Container Format** - Group by container format
- **By Music Genre** - Group by music genre (from ID3 tags)
- **By Music Artist** - Group by artist (from ID3 tags)
- **By Music Album** - Group by album (from ID3 tags)
- **By Music Release Year** - Group by release year
- **By Has Album Art** - Group by album art presence

### Video
- **By Video Duration** - Group by video duration ranges
- **By Video Resolution** - Group by video resolution (720p, 1080p, 4K, etc.)
- **By Video Frame Rate** - Group by frame rate ranges
- **By Video Codec** - Group by video codec (H.264, H.265, VP9, etc.)
- **By Video Audio Codec** - Group by audio codec
- **By Video Container Format** - Group by container format (MP4, AVI, MKV, etc.)
- **By Aspect Ratio** - Group by aspect ratio (16:9, 4:3, etc.)
- **By Has Subtitles** - Group by subtitle presence
- **By Subtitle Languages** - Group by subtitle languages
- **By Has Chapters** - Group by chapter support
- **By Is 3D** - Group by 3D support
- **By Is HD/4K** - Group by HD/4K status

### Enrichment & Duplicates
- **By Named Entity Type** - Group by NER entity types (PERSON, LOCATION, ORGANIZATION, DATE, etc.)
- **By Named Entity** - Group by specific named entities
- **By Citation Type** - Group by citation types
- **By Citation Author** - Group by citation authors
- **By Citation Year** - Group by citation years
- ✅ **By Sentiment** - Group by sentiment (positive, negative, neutral, mixed) (implemented as `sentiment`)
- **By Sentiment Score** - Group by sentiment score ranges
- ✅ **By Duplicate Type** - Group by duplicate types (exact, near, version) (implemented as `duplicate_type`)
- **By Duplicate Similarity** - Group by duplicate similarity ranges
- **By Has OCR** - Group by OCR extraction status
- **By Has Transcription** - Group by transcription status

### OS & Security
- ✅ **By Owner** - Group by file owner (username) (implemented as `owner`)
- **By Owner Type** - Group by owner type (user, system, service, unknown)
- **By Group** - Group by file group
- **By Group Category** - Group by group category (admin, developer, service, custom)
- **By Permissions** - Group by permission octal (0644, 0755, etc.)
- **By Permission Level** - Group by permission level (public, group, private, restricted)
- **By Security Category** - Group by security categories
- **By Security Attributes** - Group by security attributes (encrypted, immutable, quarantined)
- **By Has ACLs** - Group by ACL presence
- **By ACL Complexity** - Group by ACL complexity (simple, complex)
- **By Access Type** - Group by access type (read, write, execute, full)
- **By Ownership Pattern** - Group by ownership patterns (single_owner, shared_group, multi_user)
- **By File Attributes** - Group by file attributes (readonly, hidden, system, archive, compressed, encrypted)
- **By File System Type** - Group by filesystem type
- **By Mount Point** - Group by mount point

### Projects
- **By Project** - Group by assigned project
- **By Project Nature** - Group by project nature (generic, library, application, etc.)
- **By Project Path** - Group by project hierarchy path
- ✅ **By Project Assignment Score** - Group by assignment score ranges (implemented as `project_score`)
- **By Project Assignment Status** - Group by assignment status
- **By Project Assignment Source** - Group by assignment source
- **By Project Role** - Group by project role (primary, secondary, etc.)
- **By Project Member** - Group by project members
- **By Project Member Role** - Group by member role (owner, contributor, viewer)

### Relationships
- **By Relationship Type** - Group by document relationship types
- **By Relationship Strength** - Group by relationship strength ranges
- **By Relationship Confidence** - Group by relationship confidence ranges
- **By Discovery Method** - Group by discovery method (explicit, implicit, rag, filename, version, template)

### Quality & Processing
- ✅ **By Indexing Status** - Group by indexing completion (basic, mime, code, document, mirror) (implemented as `indexing_status`)
- **By Indexing Completeness** - Group by indexing completeness percentage
- **By Has Mirror** - Group by mirror extraction status
- **By Mirror Format** - Group by mirror format (markdown, CSV)
- **By Extraction Status** - Group by extraction success/failure
- **By Extraction Stage** - Group by extraction stage
- **By Extraction Error Type** - Group by extraction error types
- **By Processing Status** - Group by processing status

### Hashing & Similarity
- **By Hash** - Group by file hash (for exact duplicates)
- **By Duplicate Group** - Group by duplicate groups
- **By Similarity Range** - Group by similarity ranges

### Path & Structure
- **By Path Pattern** - Group by normalized path patterns
- **By Path Component** - Group by path components
- **By Depth** - Group by directory depth
- **By Folder Name** - Group by immediate folder name
- **By Path Structure** - Group by path structure patterns

### Content Structure
- **By Heading Depth** - Group by maximum heading depth
- **By Has Lists** - Group by list presence
- **By List Count** - Group by list count ranges
- **By Has TOC** - Group by table of contents presence
- **By Has Cross-References** - Group by cross-reference presence
- **By Has Footnotes** - Group by footnote presence
- **By Has Endnotes** - Group by endnote presence
- **By Section Count** - Group by section count ranges

### Metadata Consistency
- **By Consistency Score** - Group by metadata consistency score ranges
- **By Has Issues** - Group by consistency issues presence
- **By Has Warnings** - Group by consistency warnings presence

### Temporal Analysis
- **By Access Event Type** - Group by access event types (read, write, open, close)
- **By Modification Pattern** - Group by modification patterns
- **By Size Change** - Group by size change ranges (from modification history)

### Composite Facets
- **By Language + Type** - Multi-dimensional: Language × File Type
- **By Owner + Project** - Multi-dimensional: Owner × Project
- **By Date + Type** - Multi-dimensional: Date Range × File Type
- **By Size + Extension** - Multi-dimensional: Size Range × Extension
- **By Tag + Project** - Multi-dimensional: Tag × Project
- **By Author + Year** - Multi-dimensional: Author × Publication Year
- **By Location + Event** - Multi-dimensional: Location × Historical Event
- **By Sentiment + Language** - Multi-dimensional: Sentiment × Language
- **By Complexity + Language** - Multi-dimensional: Code Complexity × Language

## Implementation Priority

### High Priority (Most Useful)
1. **By Detected Language** - Very useful for multilingual workspaces
2. **By Author** - Essential for document libraries
3. **By Publication Year** - Important for academic/research workspaces
4. **By Code Complexity** - Useful for codebases
5. **By Sentiment** - Interesting for content analysis
6. **By Owner** - Useful for multi-user workspaces
7. **By Project Assignment Score** - Helps identify AI confidence
8. **By Indexing Status** - Shows processing completeness
9. **By Temporal Pattern** - Shows file activity patterns
10. **By Duplicate Type** - Helps identify duplicates

### Medium Priority (Nice to Have)
1. **By Location** - Useful for geotagged content
2. **By Organization** - Useful for institutional workspaces
3. **By Event** - Interesting for historical documents
4. **By Citation Type** - Useful for academic workspaces
5. **By Image Dimensions** - Useful for image collections
6. **By Audio Duration** - Useful for audio collections
7. **By Video Resolution** - Useful for video collections
8. **By Permission Level** - Security-focused views
9. **By Content Quality** - Quality-focused views
10. **By Relationship Type** - Shows document connections

### Low Priority (Specialized Use Cases)
1. **By Camera Make/Model** - Very specialized
2. **By ISO/Aperture** - Photography-specific
3. **By GPS Coordinates** - Geotagging-specific
4. **By File System Type** - System administration
5. **By Mount Point** - System administration
6. **By Extraction Error Type** - Debugging/observability
7. **By Model Usage** - Cost monitoring
8. **By Benchmark Results** - Quality monitoring

## Technical Implementation Notes

### Query Performance
- Most facets can be implemented using SQL aggregations on indexed columns
- JSON fields require JSON extraction functions (SQLite JSON1 extension)
- Some facets may require denormalization for better performance
- Consider materialized views for frequently accessed facets

### Data Availability
- Not all files will have all metadata types
- Some facets will have sparse data (e.g., GPS coordinates only for images)
- Consider showing "Unknown" or "Not Available" categories
- Some facets require specific indexing stages to be enabled

### UI Considerations
- Some facets may have hundreds of values (e.g., extensions, tags)
- Consider pagination or "Top N" views
- Some facets are hierarchical (e.g., projects, path components)
- Consider accordion/collapsible views for large result sets
- Some facets benefit from visualizations (e.g., date ranges, size ranges)

## Summary

The Cortex backend database contains **extremely rich metadata** across **50+ potential facet dimensions**. 

### Implementation Status

**✅ Implemented: 32 facetas**
- Core: 8 facetas
- Organization: 3 facetas (tag, project, project_score)
- Temporal: 5 facetas (modified, created, accessed, changed, temporal_pattern)
- Content: 15 facetas (language, category, author, publication_year, sentiment, duplicate_type, location, organization, content_type, purpose, audience, domain, subdomain, topic, readability_level, complexity, function_count, lines_of_code, comment_percentage)
- System: 1 faceta (owner, indexing_status)

**📋 Remaining: ~25+ facetas potenciales** que podrían implementarse

### Progreso: ~64% de facetas de alta prioridad implementadas

The most impactful additions would be:
- **Temporal facets** (access patterns, modification history)
- **AI-generated facets** (language, category, taxonomy)
- **Content-specific facets** (authors, locations, events)
- **Code-specific facets** (complexity, dependencies, relationships)
- **OS/security facets** (ownership, permissions, access)

This rich data model provides a solid foundation for building sophisticated file organization and discovery interfaces.
