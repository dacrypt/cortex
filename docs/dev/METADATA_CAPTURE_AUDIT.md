# Metadata Capture & Indexing Audit

## Executive Summary

This document provides a comprehensive audit of what information Cortex captures, extracts, and indexes about files throughout their lifecycle. The goal is to ensure we're capturing **all possible information** - technical, forensic, semantic, and contextual - to enable rich faceted browsing and advanced querying.

---

## 1. Information Currently Captured

### 1.1 Basic File Information (BasicStage)

**Captured:**
- ✅ File ID (SHA-256 hash of relative path)
- ✅ Relative path
- ✅ Absolute path
- ✅ Filename
- ✅ Extension
- ✅ File size (bytes)
- ✅ Last modified timestamp
- ✅ Created timestamp (from OS)
- ✅ Folder path
- ✅ Directory depth

**Persisted:**
- ✅ `files` table: `id`, `relative_path`, `absolute_path`, `filename`, `extension`, `file_size`, `last_modified`, `created_at`
- ✅ `files.enhanced` (JSON): `Stats`, `Folder`, `Depth`

**Missing:**
- ❌ File accessed timestamp (available in OS but not captured)
- ❌ File changed timestamp (metadata changed, different from modified)
- ❌ File backup timestamp (if available)
- ❌ Is read-only flag (captured in EnhancedMetadata but not in basic stats)
- ❌ Is hidden flag (captured in EnhancedMetadata but not in basic stats)

---

### 1.2 MIME Type Information (MimeStage)

**Captured:**
- ✅ MIME type (e.g., "application/pdf")
- ✅ MIME category (text, code, image, document, binary, archive, audio, video)
- ✅ Encoding (if applicable)

**Persisted:**
- ✅ `files.enhanced` (JSON): `MimeType.MimeType`, `MimeType.Category`, `MimeType.Encoding`

**Missing:**
- ❌ MIME type detection confidence
- ❌ Alternative MIME types (some files can be multiple types)
- ❌ File signature/magic bytes (for verification)

---

### 1.3 Code Analysis (CodeStage)

**Captured:**
- ✅ Lines of code
- ✅ Comment lines
- ✅ Blank lines
- ✅ Comment percentage
- ✅ Function count
- ✅ Class count
- ✅ Complexity score
- ✅ Imports list
- ✅ Exports list

**Persisted:**
- ✅ `files.enhanced` (JSON): `CodeMetrics.*`

**Missing:**
- ❌ Language version (e.g., Python 3.9, TypeScript 5.3)
- ❌ Framework/library detection (React, Vue, Django, etc.)
- ❌ Test coverage metrics
- ❌ Code quality scores (linting, complexity)
- ❌ Build system detection (package.json, go.mod, Cargo.toml)
- ❌ Dependency versions (from lock files)

---

### 1.4 Document Metadata (MetadataStage)

**Captured for PDFs:**
- ✅ PDF Info Dictionary: Author, Title, Subject, Keywords, Creator, Producer, CreatedDate, ModifiedDate
- ✅ XMP Metadata: Title, Description, Creator, Contributor, Rights, Copyright, Language, Rating, etc.
- ✅ PDF Technical: Version, Encrypted, Linearized, Tagged, PageLayout, PageMode
- ✅ Document Properties: Company, Category, Comments
- ✅ Hyperlinks (URLs found)
- ✅ Fonts used
- ✅ Color spaces
- ✅ Image count
- ✅ Form fields count
- ✅ Annotations count
- ✅ Page count
- ✅ Word count
- ✅ Character count
- ✅ Custom properties (key-value pairs)

**Captured for Images:**
- ✅ Basic: Width, Height, ColorDepth, ColorSpace, Format, Orientation
- ✅ EXIF: Camera make/model, Software, DateTimeOriginal, DateTimeDigitized, Artist, Copyright, ImageDescription
- ✅ Camera Settings: F-Number, ExposureTime, ISO, FocalLength, ExposureMode, WhiteBalance, Flash, MeteringMode
- ✅ GPS: Latitude, Longitude, Altitude, Location
- ✅ IPTC: ObjectName, Caption, Keywords, Copyright, Byline, Headline, Contact info
- ✅ XMP: Title, Description, Creator, Rights, Rating, Label, Subject
- ✅ Analysis: DominantColors, HasTransparency, IsAnimated, FrameCount

**Captured for Audio:**
- ✅ Basic: Duration, Bitrate, SampleRate, Channels, BitDepth, Codec, Format
- ✅ ID3 Tags: Title, Artist, Album, Year, Genre, Track, Disc, Composer, Conductor, Performer, Publisher, Comment, Lyrics, BPM, ISRC, Copyright, EncodedBy, AlbumArtist
- ✅ Vorbis Comments: Title, Artist, Album, Date, Genre, Track, Comment
- ✅ Album Art: HasAlbumArt, AlbumArtFormat, AlbumArtSize
- ✅ Technical: ReplayGain, Normalized, Lossless

**Captured for Video:**
- ✅ Basic: Duration, Width, Height, FrameRate, Bitrate, Codec, Container
- ✅ Video Stream: VideoCodec, VideoBitrate, VideoPixelFormat, VideoColorSpace, VideoAspectRatio
- ✅ Audio Stream: AudioCodec, AudioBitrate, AudioSampleRate, AudioChannels, AudioLanguage
- ✅ Metadata Tags: Title, Artist, Album, Genre, Year, Director, Producer, Copyright, Description, Comment
- ✅ Technical: HasSubtitles, SubtitleTracks, HasChapters, ChapterCount, Is3D, IsHD, Is4K

**Persisted:**
- ✅ `files.enhanced` (JSON): `DocumentMetrics.*`, `ImageMetadata.*`, `AudioMetadata.*`, `VideoMetadata.*`

**Missing:**
- ❌ Document revision history (if embedded)
- ❌ Document security settings (password protection, printing restrictions)
- ❌ Document structure (outline/bookmarks for PDFs)
- ❌ Embedded objects (images, videos, attachments)
- ❌ Document templates used
- ❌ Document workflows/approval chains (if metadata exists)

---

### 1.5 OS Metadata (OSMetadataStage)

**Captured:**
- ✅ Permissions: Octal, String representation, Owner/Group/Other read/write/execute, SetUID, SetGID, StickyBit
- ✅ Ownership: Owner (UID, Username, FullName, HomeDir, Shell), Group (GID, GroupName, Members)
- ✅ File Attributes: IsReadOnly, IsHidden, IsSystem, IsArchive, IsCompressed, IsEncrypted, IsImmutable, IsAppendOnly, IsNoDump
- ✅ Extended Attributes: xattr/ADS (key-value pairs)
- ✅ ACLs: Access Control List entries
- ✅ Timestamps: Created, Modified, Accessed, Changed (metadata), Backup
- ✅ File System: MountPoint, DeviceID, FileSystemType, BlockSize, Blocks, SELinuxContext

**Persisted:**
- ✅ `files.os_metadata` (JSON): Full `OSMetadata` structure
- ✅ `files.os_taxonomy` (JSON): `OSContextTaxonomy` (Security, Ownership, Temporal, System, Organization dimensions)
- ✅ `persons` table: Human identities
- ✅ `system_users` table: OS user accounts
- ✅ `file_ownership` table: File ownership relationships
- ✅ `file_access` table: File access relationships (ACLs)

**Missing:**
- ❌ File system quotas (if applicable)
- ❌ File system compression ratio
- ❌ Sparse file information
- ❌ Hard link count
- ❌ Symbolic link target (if symlink)
- ❌ File system mount options

---

### 1.6 Document Content (DocumentStage)

**Captured:**
- ✅ Document ID (SHA-256 hash)
- ✅ Document title (inferred from frontmatter, filename, or content)
- ✅ Frontmatter (YAML/TOML at document start)
- ✅ Document body (content without frontmatter)
- ✅ Document checksum (hash of body)
- ✅ Chunks (semantically grouped sections)
- ✅ Chunk metadata: Heading, HeadingPath, TokenCount, StartLine, EndLine
- ✅ Embeddings (vector representations for RAG)

**Persisted:**
- ✅ `documents` table: `id`, `file_id`, `relative_path`, `title`, `frontmatter`, `checksum`, `state`, `created_at`, `updated_at`
- ✅ `chunks` table: `id`, `document_id`, `ordinal`, `heading`, `heading_path`, `text`, `token_count`, `start_line`, `end_line`
- ✅ `chunk_embeddings` table: `chunk_id`, `vector`, `dimensions`, `updated_at`

**Missing:**
- ❌ Document structure tree (hierarchical outline)
- ❌ Document sections/headings hierarchy (beyond heading_path)
- ❌ Document footnotes/endnotes
- ❌ Document cross-references
- ❌ Document table of contents (if present)

---

### 1.7 Mirror Content (MirrorStage)

**Captured:**
- ✅ Mirror format (md or csv)
- ✅ Mirror file path
- ✅ Source file modification time
- ✅ Mirror update time
- ✅ Extracted text content (for PDFs, Office docs)

**Persisted:**
- ✅ `file_metadata.mirror_format`, `mirror_path`, `mirror_source_mtime`, `mirror_updated_at`
- ✅ Mirror files stored in `.cortex/mirror/` directory

**Missing:**
- ❌ Mirror extraction method used (pandoc, pdftotext, pdf library, etc.)
- ❌ Mirror extraction confidence/quality score
- ❌ Mirror extraction errors/warnings

---

### 1.8 AI-Generated Metadata (AIStage)

**Captured:**
- ✅ AI Summary: Summary text, Content hash, Key terms, Generated timestamp
- ✅ AI Category: Category name, Confidence, Updated timestamp
- ✅ AI Context: Authors, Editors, Translators, Contributors, Publisher, PublicationYear, PublicationPlace, Edition, ISBN, ISSN, DOI, DocumentDate, HistoricalPeriod, Locations, Regions, PeopleMentioned, Organizations, HistoricalEvents, References, OriginalLanguage, Genre, Subject, Audience, EnrichedMetadata (from external APIs)
- ✅ AI Related Files: RelativePath, Similarity, Reason
- ✅ Detected Language: Language code (e.g., "es", "en")

**Persisted:**
- ✅ `file_metadata.ai_summary` (JSON)
- ✅ `file_metadata.ai_summary_hash`
- ✅ `file_metadata.ai_key_terms` (JSON array)
- ✅ `file_metadata.ai_category` (JSON)
- ✅ `file_metadata.ai_context` (JSON)
- ✅ `file_metadata.ai_related` (JSON)
- ✅ `file_metadata.detected_language`

**Missing:**
- ❌ AI processing timestamp (when AI analysis was performed)
- ❌ AI model version used
- ❌ AI processing cost/tokens (for tracking)
- ❌ AI confidence scores per field (not just category)

---

### 1.9 Enrichment Data (EnrichmentStage)

**Captured:**
- ✅ Named Entities: Text, Type, StartPos, EndPos, Confidence, Context
- ✅ Citations: Text, Authors, Title, Year, DOI, URL, Type, Context, Confidence, Page
- ✅ Sentiment Analysis: OverallSentiment, Score, Confidence, Emotions map
- ✅ Tables: Rows, Headers, Caption, Page, RowCount, ColumnCount
- ✅ Formulas: Text, LaTeX, Context, Page, Type
- ✅ Dependencies: Name, Version, Type, Language, Path
- ✅ Duplicates: DocumentID, RelativePath, Similarity, Type, Reason
- ✅ OCR Results: Text, Confidence, Language, PageCount, ExtractedAt
- ✅ Transcription: Text, Language, Duration, Confidence, Segments, ExtractedAt

**Persisted:**
- ✅ `file_metadata.enrichment_data` (JSON)

**Missing:**
- ❌ Enrichment processing timestamp
- ❌ Enrichment method/tool used
- ❌ Enrichment errors/warnings

---

### 1.10 Relationships (RelationshipStage)

**Captured:**
- ✅ Document relationships: FromDocumentID, ToDocumentID, Type, Strength, Metadata

**Persisted:**
- ✅ `document_relationships` table

**Missing:**
- ❌ File-level relationships (not just document-level)
- ❌ Relationship confidence scores
- ❌ Relationship discovery method

---

### 1.11 Document State (StateStage)

**Captured:**
- ✅ Document state: draft, review, published, archived, etc.
- ✅ State change history: FromState, ToState, Reason, ChangedBy, ChangedAt

**Persisted:**
- ✅ `documents.state`, `documents.state_changed_at`
- ✅ `document_state_history` table

---

### 1.12 Suggested Metadata (SuggestionStage)

**Captured:**
- ✅ Suggested tags: Tag, Confidence, Reason, Source, Category
- ✅ Suggested projects: ProjectID, ProjectName, Confidence, Reason, Source, IsNew
- ✅ Suggested taxonomy: Category, Subcategory, Domain, Subdomain, ContentType, Purpose, Audience, Language, Confidence scores, Reasoning, Source
- ✅ Suggested topics: Topic names
- ✅ Suggested fields: FieldName, FieldValue, FieldType, Confidence, Reason, Source

**Persisted:**
- ✅ `suggested_metadata` table
- ✅ `suggested_tags` table
- ✅ `suggested_projects` table
- ✅ `suggested_taxonomy` table
- ✅ `suggested_taxonomy_topics` table
- ✅ `suggested_fields` table

---

### 1.13 User-Assigned Metadata (Manual)

**Captured:**
- ✅ Tags: User-assigned tags
- ✅ Contexts/Projects: User-assigned projects
- ✅ Notes: Free-form notes
- ✅ Type: File type (inferred from extension)

**Persisted:**
- ✅ `file_metadata.type`, `file_metadata.notes`
- ✅ `file_tags` table (many-to-many)
- ✅ `file_contexts` table (many-to-many)
- ✅ `file_context_suggestions` table

---

### 1.14 Projects (ProjectService)

**Captured:**
- ✅ Project hierarchy: ID, Name, Description, ParentID, Path
- ✅ Project-document relationships: ProjectID, DocumentID, Role

**Persisted:**
- ✅ `projects` table
- ✅ `project_documents` table
- ✅ `project_memberships` table

---

### 1.15 Usage Events (UsageService)

**Captured:**
- ✅ Document usage events: EventType, Context, Metadata, Timestamp

**Persisted:**
- ✅ `document_usage_events` table

---

## 2. Information NOT Currently Captured

### 2.1 Path Information

**Currently:**
- ✅ Relative path is stored
- ✅ Absolute path is stored
- ✅ Folder path is stored
- ✅ Directory depth is stored

**Missing:**
- ❌ **Path is NOT included in AI prompts** (project assignment, tag suggestions)
- ❌ **Path is NOT included in RAG embeddings** (only content + metadata, not path)
- ❌ Path components breakdown (e.g., `["docs", "projects", "website"]` from `docs/projects/website/README.md`)
- ❌ Path patterns/regular expressions
- ❌ Path similarity to other files
- ❌ Path-based project inference (files in same directory → same project)

**Impact:**
- Paths often contain semantic information (e.g., `docs/projects/website/README.md` suggests website project)
- Directory structure can indicate project relationships
- Path-based clustering could improve project assignment

---

### 2.2 File System Events

**Missing:**
- ❌ File creation events (when file was first created in workspace)
- ❌ File modification history (not just last modified)
- ❌ File access history (who accessed, when)
- ❌ File move/rename history
- ❌ File deletion events (soft delete tracking)

---

### 2.3 Content Analysis

**Missing:**
- ❌ Language detection confidence (not just detected language)
- ❌ Content encoding detection (UTF-8, Latin-1, etc.)
- ❌ Content structure analysis (headings hierarchy, list structures)
- ❌ Content quality metrics (readability, complexity)
- ❌ Content similarity to other files (beyond AI-related)
- ❌ Content change detection (what changed between versions)

---

### 2.4 Technical Metadata

**Missing:**
- ❌ File hash (MD5, SHA-256) for duplicate detection
- ❌ File magic bytes/signature
- ❌ File compression ratio
- ❌ File entropy (randomness measure, useful for encrypted/compressed files)
- ❌ File type detection confidence
- ❌ File corruption detection

---

### 2.5 Forensic Metadata

**Missing:**
- ❌ File creation software version
- ❌ File modification software version
- ❌ Embedded metadata extraction errors
- ❌ Metadata inconsistencies (e.g., creation date after modification date)
- ❌ Hidden metadata (steganography detection)
- ❌ File carving artifacts (if file was recovered)

---

### 2.6 Relationship Metadata

**Missing:**
- ❌ Import/export relationships (code files)
- ❌ Include/dependency relationships (code files)
- ❌ Reference relationships (documents referencing other documents)
- ❌ Version relationships (file1 is version 2 of file2)
- ❌ Template relationships (file1 is template for file2)
- ❌ Relationship confidence scores

---

### 2.7 Temporal Metadata

**Missing:**
- ❌ File age (time since creation)
- ❌ File staleness (time since last modification)
- ❌ File access frequency
- ❌ File modification frequency
- ❌ Temporal patterns (e.g., files modified together)
- ❌ Temporal clustering (files modified in same time window)

---

### 2.8 User Context

**Missing:**
- ❌ File creator (if available from OS)
- ❌ File last editor (if available from OS)
- ❌ File access patterns (who accesses what, when)
- ❌ User preferences (tags user commonly uses)
- ❌ User-defined categories/taxonomies

---

## 3. Information Persistence Gaps

### 3.1 EnhancedMetadata Not Fully Persisted

**Currently:**
- ✅ `files.enhanced` stores JSON of `EnhancedMetadata`
- ✅ This includes: Stats, Folder, Depth, Language, MimeType, CodeMetrics, DocumentMetrics, ImageMetadata, AudioMetadata, VideoMetadata, OSMetadata, OSContextTaxonomy, IndexedState, CustomData

**Issue:**
- ⚠️ EnhancedMetadata is stored as JSON blob, making it hard to query
- ⚠️ No indexes on EnhancedMetadata fields (can't efficiently query by MIME type, language, etc.)
- ⚠️ No separate tables for frequently queried metadata

**Recommendation:**
- Consider denormalizing frequently queried fields into separate columns
- Add indexes on commonly queried EnhancedMetadata fields

---

### 3.2 AI Context Not Fully Indexed

**Currently:**
- ✅ `file_metadata.ai_context` stores JSON of `AIContext`
- ✅ Includes: Authors, Editors, Translators, Publisher, PublicationYear, ISBN, ISSN, DOI, Locations, People, Organizations, Events, References, etc.

**Issue:**
- ⚠️ Stored as JSON, not queryable
- ⚠️ Can't efficiently search by author, ISBN, location, etc.

**Recommendation:**
- Create separate tables for: `file_authors`, `file_locations`, `file_people`, `file_organizations`, `file_events`, `file_references`
- Add indexes for efficient querying

---

### 3.3 Enrichment Data Not Indexed

**Currently:**
- ✅ `file_metadata.enrichment_data` stores JSON of `EnrichmentData`
- ✅ Includes: NamedEntities, Citations, Sentiment, Tables, Formulas, Dependencies, Duplicates, OCR, Transcription

**Issue:**
- ⚠️ Stored as JSON, not queryable
- ⚠️ Can't efficiently search by named entity, citation, dependency, etc.

**Recommendation:**
- Create separate tables for: `file_named_entities`, `file_citations`, `file_dependencies`, `file_duplicates`
- Add indexes for efficient querying

---

### 3.4 Path Information Not Indexed

**Currently:**
- ✅ `files.relative_path` is indexed
- ✅ `files.enhanced` (JSON) contains `Folder` and `Depth`

**Issue:**
- ⚠️ Can't efficiently query by path patterns (e.g., all files in `docs/projects/*`)
- ⚠️ Can't efficiently query by directory depth
- ⚠️ Path components not extracted for querying

**Recommendation:**
- Add `path_components` column (JSON array of path segments)
- Add `path_pattern` column (normalized path pattern)
- Add indexes on path components

---

## 4. Recommendations

### 4.1 Immediate Improvements

1. **Include Path in AI Context**
   - Add relative path to project assignment prompts
   - Add relative path to RAG embedding enrichment
   - Extract path components for semantic analysis

2. **Index Frequently Queried Metadata**
   - Create separate tables for authors, locations, people, organizations
   - Add indexes on EnhancedMetadata fields (MIME type, language, etc.)
   - Create path component indexes

3. **Capture Missing Timestamps**
   - Add `accessed_at` to FileStats
   - Add `changed_at` (metadata changed) to FileStats
   - Add `backup_at` if available

4. **Improve Content Analysis**
   - Add language detection confidence
   - Add content encoding detection
   - Add content structure analysis

### 4.2 Medium-Term Improvements

1. **Denormalize Critical Metadata**
   - Extract commonly queried fields from JSON blobs
   - Create materialized views for complex queries
   - Add full-text search indexes

2. **Add Relationship Tracking**
   - Track import/export relationships in code
   - Track reference relationships in documents
   - Track version relationships

3. **Add Temporal Analysis**
   - Track file access patterns
   - Track file modification patterns
   - Track temporal clustering

### 4.3 Long-Term Improvements

1. **Add Forensic Analysis**
   - File hash tracking for duplicate detection
   - Metadata consistency checking
   - Hidden metadata detection

2. **Add User Context**
   - Track user access patterns
   - Track user preferences
   - Track user-defined taxonomies

3. **Add Content Quality Metrics**
   - Readability scores
   - Complexity metrics
   - Quality scores

---

## 5. Faceted Browsing Capabilities

### 5.1 Current Facets

Based on current data, we can facet by:
- ✅ File type (extension)
- ✅ MIME type
- ✅ MIME category
- ✅ Language (detected)
- ✅ Tags (user-assigned)
- ✅ Projects/Contexts (user-assigned)
- ✅ Categories (AI-assigned)
- ✅ Authors (from AI context)
- ✅ Publishers (from AI context)
- ✅ Publication years (from AI context)
- ✅ Locations (from AI context)
- ✅ File size ranges
- ✅ Modification date ranges
- ✅ Directory depth
- ✅ Folder path
- ✅ Document state
- ✅ OS ownership (user, group)
- ✅ OS permissions
- ✅ Security taxonomy
- ✅ Temporal taxonomy

### 5.2 Missing Facets

We could facet by (if we capture):
- ❌ Path patterns
- ❌ Path components
- ❌ File age ranges
- ❌ Access frequency
- ❌ Modification frequency
- ❌ Content quality scores
- ❌ Readability levels
- ❌ Complexity levels
- ❌ Dependency types
- ❌ Relationship types
- ❌ Named entity types
- ❌ Citation types
- ❌ Sentiment scores
- ❌ File entropy ranges
- ❌ Compression ratios

---

## 6. Conclusion

Cortex currently captures a **comprehensive** amount of metadata about files, including:
- ✅ Basic file information
- ✅ Technical metadata (MIME, code metrics)
- ✅ Document metadata (PDF, images, audio, video)
- ✅ OS metadata (permissions, ownership, timestamps)
- ✅ AI-generated metadata (summaries, categories, context)
- ✅ Enrichment data (NER, citations, sentiment, etc.)
- ✅ Relationships and state

**However**, there are opportunities to improve:
1. **Path information** should be included in AI context and RAG embeddings
2. **Frequently queried metadata** should be indexed separately (not just JSON blobs)
3. **Missing metadata** should be captured (access timestamps, path components, etc.)
4. **Relationship tracking** could be expanded (imports, references, versions)

The system is well-positioned for faceted browsing, but indexing improvements would significantly enhance query performance and enable more sophisticated filtering.



