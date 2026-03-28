package query

import (
	"fmt"
)

// FacetCategory represents the category of a facet.
type FacetCategory string

const (
	FacetCategoryCore         FacetCategory = "core"
	FacetCategoryOrganization FacetCategory = "organization"
	FacetCategoryTemporal     FacetCategory = "temporal"
	FacetCategoryContent      FacetCategory = "content"
	FacetCategorySystem       FacetCategory = "system"
	FacetCategorySpecialized  FacetCategory = "specialized"
)

// AvailabilityLevel indicates how commonly available a facet is.
type AvailabilityLevel string

const (
	AvailabilityAlways      AvailabilityLevel = "always"      // All files have this
	AvailabilityConditional AvailabilityLevel = "conditional" // Some files have this
	AvailabilityRare        AvailabilityLevel = "rare"        // Very few files have this
)

// FacetDefinition defines a facet with its metadata.
type FacetDefinition struct {
	CanonicalName string
	Aliases       []string
	Category      FacetCategory
	Type          FacetType
	Description   string
	DataSource    string
	Availability  AvailabilityLevel
}

// FacetRegistry maintains a registry of all available facets.
type FacetRegistry struct {
	facets map[string]*FacetDefinition
}

// NewFacetRegistry creates a new facet registry with all facets registered.
func NewFacetRegistry() *FacetRegistry {
	r := &FacetRegistry{
		facets: make(map[string]*FacetDefinition),
	}
	r.registerAll()
	return r
}

// registerAll registers all available facets.
func (r *FacetRegistry) registerAll() {
	// Core facets
	r.register(&FacetDefinition{
		CanonicalName: "extension",
		Category:      FacetCategoryCore,
		Type:          FacetTypeTerms,
		Description:   "File extension",
		DataSource:    "files.extension",
		Availability:  AvailabilityAlways,
	})

	r.register(&FacetDefinition{
		CanonicalName: "type",
		Aliases:       []string{"file_type"},
		Category:      FacetCategoryCore,
		Type:          FacetTypeTerms,
		Description:   "Inferred file type",
		DataSource:    "files.enhanced (inferred)",
		Availability:  AvailabilityAlways,
	})

	r.register(&FacetDefinition{
		CanonicalName: "size",
		Aliases:       []string{"file_size"},
		Category:      FacetCategoryCore,
		Type:          FacetTypeNumericRange,
		Description:   "File size in bytes",
		DataSource:    "files.file_size",
		Availability:  AvailabilityAlways,
	})

	r.register(&FacetDefinition{
		CanonicalName: "indexing_status",
		Aliases:       []string{"index_status"},
		Category:      FacetCategoryCore,
		Type:          FacetTypeTerms,
		Description:   "Indexing completion status",
		DataSource:    "files.indexed_* flags",
		Availability:  AvailabilityAlways,
	})

	r.register(&FacetDefinition{
		CanonicalName: "indexing_error",
		Aliases:       []string{"index_error", "indexing_errors"},
		Category:      FacetCategoryCore,
		Type:          FacetTypeTerms,
		Description:   "Indexing errors by stage (document, mirror, code, etc.)",
		DataSource:    "files.enhanced.IndexingErrors",
		Availability:  AvailabilityConditional,
	})

	// Organization facets
	r.register(&FacetDefinition{
		CanonicalName: "tag",
		Category:      FacetCategoryOrganization,
		Type:          FacetTypeTerms,
		Description:   "User-assigned tags",
		DataSource:    "file_tags",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "folder",
		Category:      FacetCategoryOrganization,
		Type:          FacetTypeTerms,
		Description:   "Group files by folder name (first-level folder)",
		DataSource:    "files.relative_path (extracted)",
		Availability:  AvailabilityAlways,
	})

	r.register(&FacetDefinition{
		CanonicalName: "project",
		Aliases:       []string{"context"},
		Category:      FacetCategoryOrganization,
		Type:          FacetTypeTerms,
		Description:   "User-assigned projects/contexts",
		DataSource:    "file_contexts",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "cluster",
		Aliases:       []string{"document_cluster"},
		Category:      FacetCategoryOrganization,
		Type:          FacetTypeTerms,
		Description:   "AI-detected document clusters",
		DataSource:    "document_clusters",
		Availability:  AvailabilityConditional,
	})

	// Temporal facets
	r.register(&FacetDefinition{
		CanonicalName: "modified",
		Aliases:       []string{"last_modified", "mtime"},
		Category:      FacetCategoryTemporal,
		Type:          FacetTypeDateRange,
		Description:   "Last modification date",
		DataSource:    "files.last_modified",
		Availability:  AvailabilityAlways,
	})

	r.register(&FacetDefinition{
		CanonicalName: "created",
		Aliases:       []string{"created_at"},
		Category:      FacetCategoryTemporal,
		Type:          FacetTypeDateRange,
		Description:   "File creation date",
		DataSource:    "files.created_at",
		Availability:  AvailabilityAlways,
	})

	r.register(&FacetDefinition{
		CanonicalName: "accessed",
		Aliases:       []string{"accessed_at"},
		Category:      FacetCategoryTemporal,
		Type:          FacetTypeDateRange,
		Description:   "Last access date",
		DataSource:    "files.accessed_at",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "changed",
		Aliases:       []string{"changed_at"},
		Category:      FacetCategoryTemporal,
		Type:          FacetTypeDateRange,
		Description:   "Metadata change date (ctime)",
		DataSource:    "files.changed_at",
		Availability:  AvailabilityConditional,
	})

	// Content facets
	r.register(&FacetDefinition{
		CanonicalName: "language",
		Aliases:       []string{"detected_language"},
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "Detected language",
		DataSource:    "file_metadata.detected_language",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "category",
		Aliases:       []string{"ai_category"},
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "AI-assigned category",
		DataSource:    "file_metadata.ai_category",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "author",
		Aliases:       []string{"authors"},
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "Extracted authors",
		DataSource:    "file_authors",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "publication_year",
		Aliases:       []string{"year"},
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "Publication year",
		DataSource:    "file_publication_info.publication_year",
		Availability:  AvailabilityRare,
	})

	// System facets
	r.register(&FacetDefinition{
		CanonicalName: "owner",
		Aliases:       []string{"owners"},
		Category:      FacetCategorySystem,
		Type:          FacetTypeTerms,
		Description:   "File owner (OS metadata)",
		DataSource:    "file_ownership + system_users",
		Availability:  AvailabilityConditional,
	})

	// Enrichment facets
	r.register(&FacetDefinition{
		CanonicalName: "sentiment",
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "Sentiment analysis (positive, negative, neutral, mixed)",
		DataSource:    "file_sentiment.overall_sentiment",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "duplicate_type",
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "Duplicate relationship type (exact, near, version)",
		DataSource:    "file_duplicates.type",
		Availability:  AvailabilityRare,
	})

	// Geographic facets
	r.register(&FacetDefinition{
		CanonicalName: "location",
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "Geographic locations mentioned",
		DataSource:    "file_locations",
		Availability:  AvailabilityConditional,
	})

	// Organization facets (from AI context)
	r.register(&FacetDefinition{
		CanonicalName: "organization",
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "Organizations mentioned",
		DataSource:    "file_organizations",
		Availability:  AvailabilityConditional,
	})

	// Taxonomy facets
	r.register(&FacetDefinition{
		CanonicalName: "document_type",
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "AI-suggested document/content type (semantic classification)",
		DataSource:    "suggested_taxonomy.content_type",
		Availability:  AvailabilityConditional,
	})

	// MIME type facets
	r.register(&FacetDefinition{
		CanonicalName: "mime_type",
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "MIME content type (e.g., image/jpeg, application/pdf)",
		DataSource:    "files.enhanced->MimeType.MimeType",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "mime_category",
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "MIME category (e.g., image, document, text, code)",
		DataSource:    "files.enhanced->MimeType.Category",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "purpose",
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "AI-suggested purpose",
		DataSource:    "suggested_taxonomy.purpose",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "audience",
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "AI-suggested audience",
		DataSource:    "suggested_taxonomy.audience",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "complexity",
		Category:      FacetCategoryContent,
		Type:          FacetTypeNumericRange,
		Description:   "Code complexity score",
		DataSource:    "files.enhanced->CodeMetrics.Complexity",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "project_score",
		Aliases:       []string{"assignment_score"},
		Category:      FacetCategoryOrganization,
		Type:          FacetTypeNumericRange,
		Description:   "Project assignment confidence score (0.0-1.0)",
		DataSource:    "project_assignments.score",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "domain",
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "AI-suggested domain classification",
		DataSource:    "suggested_taxonomy.domain",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "subdomain",
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "AI-suggested subdomain classification",
		DataSource:    "suggested_taxonomy.subdomain",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "topic",
		Aliases:       []string{"topics"},
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "AI-suggested topics (many-to-many)",
		DataSource:    "suggested_taxonomy_topics.topic",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "event",
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "Historical events mentioned",
		DataSource:    "file_events.name",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "citation_type",
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "Citation type (book, article, website, conference)",
		DataSource:    "file_citations.type",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "relationship_type",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Document relationship type",
		DataSource:    "document_relationships.type",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "temporal_pattern",
		Aliases:       []string{"access_pattern", "last_access_pattern"},
		Category:      FacetCategoryTemporal,
		Type:          FacetTypeTerms,
		Description:   "Temporal access pattern (recent, occasional, rare, never)",
		DataSource:    "files.enhanced->TemporalMetrics.LastAccessPattern",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "readability_level",
		Aliases:       []string{"readability"},
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "Content readability level (elementary, high_school, college, graduate, professional)",
		DataSource:    "files.enhanced->ContentQuality.ReadabilityLevel",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "function_count",
		Category:      FacetCategoryContent,
		Type:          FacetTypeNumericRange,
		Description:   "Number of functions in code files",
		DataSource:    "files.enhanced->CodeMetrics.FunctionCount",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "lines_of_code",
		Aliases:       []string{"loc"},
		Category:      FacetCategoryContent,
		Type:          FacetTypeNumericRange,
		Description:   "Lines of code count",
		DataSource:    "files.enhanced->CodeMetrics.LinesOfCode",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "comment_percentage",
		Aliases:       []string{"comment_pct"},
		Category:      FacetCategoryContent,
		Type:          FacetTypeNumericRange,
		Description:   "Percentage of code that is comments",
		DataSource:    "files.enhanced->CodeMetrics.CommentPercentage",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "video_resolution",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Video resolution tier (720p, 1080p, 4K, etc.)",
		DataSource:    "files.enhanced->VideoMetadata.Width/Height",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "permission_level",
		Category:      FacetCategorySystem,
		Type:          FacetTypeTerms,
		Description:   "OS permission level (public, group, private, restricted)",
		DataSource:    "files.enhanced->OSContextTaxonomy.Security.PermissionLevel",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "content_encoding",
		Category:      FacetCategoryContent,
		Type:          FacetTypeTerms,
		Description:   "Detected content encoding",
		DataSource:    "files.enhanced->ContentEncoding",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "language_confidence",
		Category:      FacetCategoryContent,
		Type:          FacetTypeNumericRange,
		Description:   "Language detection confidence score (0.0 - 1.0)",
		DataSource:    "files.enhanced->LanguageConfidence",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "filesystem_type",
		Category:      FacetCategorySystem,
		Type:          FacetTypeTerms,
		Description:   "Filesystem type",
		DataSource:    "files.enhanced->OSMetadata.FileSystem.FileSystemType",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "mount_point",
		Category:      FacetCategorySystem,
		Type:          FacetTypeTerms,
		Description:   "Filesystem mount point",
		DataSource:    "files.enhanced->OSMetadata.FileSystem.MountPoint",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "security_category",
		Category:      FacetCategorySystem,
		Type:          FacetTypeTerms,
		Description:   "Security category labels",
		DataSource:    "files.enhanced->OSContextTaxonomy.Security.SecurityCategory",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "security_attributes",
		Category:      FacetCategorySystem,
		Type:          FacetTypeTerms,
		Description:   "Security attribute labels",
		DataSource:    "files.enhanced->OSContextTaxonomy.Security.SecurityAttributes",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "has_acls",
		Category:      FacetCategorySystem,
		Type:          FacetTypeTerms,
		Description:   "ACL presence",
		DataSource:    "files.enhanced->OSContextTaxonomy.Security.HasACLs",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "acl_complexity",
		Category:      FacetCategorySystem,
		Type:          FacetTypeTerms,
		Description:   "ACL complexity",
		DataSource:    "files.enhanced->OSContextTaxonomy.Security.ACLComplexity",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "owner_type",
		Category:      FacetCategorySystem,
		Type:          FacetTypeTerms,
		Description:   "Owner type classification",
		DataSource:    "files.enhanced->OSContextTaxonomy.Ownership.OwnerType",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "group_category",
		Category:      FacetCategorySystem,
		Type:          FacetTypeTerms,
		Description:   "Group category classification",
		DataSource:    "files.enhanced->OSContextTaxonomy.Ownership.GroupCategory",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "access_relation",
		Category:      FacetCategorySystem,
		Type:          FacetTypeTerms,
		Description:   "Access relation labels",
		DataSource:    "files.enhanced->OSContextTaxonomy.Ownership.AccessRelations",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "ownership_pattern",
		Category:      FacetCategorySystem,
		Type:          FacetTypeTerms,
		Description:   "Ownership pattern classification",
		DataSource:    "files.enhanced->OSContextTaxonomy.Ownership.OwnershipPattern",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "access_frequency",
		Category:      FacetCategoryTemporal,
		Type:          FacetTypeTerms,
		Description:   "Access frequency classification",
		DataSource:    "files.enhanced->OSContextTaxonomy.Temporal.AccessFrequency",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "time_category",
		Category:      FacetCategoryTemporal,
		Type:          FacetTypeTerms,
		Description:   "Time category labels",
		DataSource:    "files.enhanced->OSContextTaxonomy.Temporal.TimeCategory",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "system_file_type",
		Category:      FacetCategorySystem,
		Type:          FacetTypeTerms,
		Description:   "System file type classification",
		DataSource:    "files.enhanced->OSContextTaxonomy.System.SystemFileType",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "file_system_category",
		Category:      FacetCategorySystem,
		Type:          FacetTypeTerms,
		Description:   "Filesystem category classification",
		DataSource:    "files.enhanced->OSContextTaxonomy.System.FileSystemCategory",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "system_attributes",
		Category:      FacetCategorySystem,
		Type:          FacetTypeTerms,
		Description:   "System attribute labels",
		DataSource:    "files.enhanced->OSContextTaxonomy.System.SystemAttributes",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "system_features",
		Category:      FacetCategorySystem,
		Type:          FacetTypeTerms,
		Description:   "System feature labels",
		DataSource:    "files.enhanced->OSContextTaxonomy.System.SystemFeatures",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "content_quality",
		Category:      FacetCategoryContent,
		Type:          FacetTypeNumericRange,
		Description:   "Content quality score (0.0 - 1.0)",
		DataSource:    "files.enhanced->ContentQuality.QualityScore",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "image_dimensions",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeNumericRange,
		Description:   "Image size in total pixels (width × height)",
		DataSource:    "files.enhanced->ImageMetadata.Width/Height",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "image_format",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Image file format (JPEG, PNG, TIFF, etc.)",
		DataSource:    "files.enhanced->ImageMetadata.Format",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "image_color_space",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Image color space (RGB, CMYK, etc.)",
		DataSource:    "files.enhanced->ImageMetadata.ColorSpace",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "camera_make",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Camera manufacturer",
		DataSource:    "files.enhanced->ImageMetadata.EXIFCameraMake",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "camera_model",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Camera model",
		DataSource:    "files.enhanced->ImageMetadata.EXIFCameraModel",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "image_gps_location",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Image GPS location",
		DataSource:    "files.enhanced->ImageMetadata.GPSLocation",
		Availability:  AvailabilityRare,
	})

	r.register(&FacetDefinition{
		CanonicalName: "image_orientation",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Image EXIF orientation",
		DataSource:    "files.enhanced->ImageMetadata.Orientation",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "image_has_transparency",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Image transparency support",
		DataSource:    "files.enhanced->ImageMetadata.HasTransparency",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "image_is_animated",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Image animation support",
		DataSource:    "files.enhanced->ImageMetadata.IsAnimated",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "image_color_depth",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeNumericRange,
		Description:   "Image color depth (bits per pixel)",
		DataSource:    "files.enhanced->ImageMetadata.ColorDepth",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "image_iso",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeNumericRange,
		Description:   "Image ISO setting",
		DataSource:    "files.enhanced->ImageMetadata.EXIFISO",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "image_aperture",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeNumericRange,
		Description:   "Image aperture (f-number)",
		DataSource:    "files.enhanced->ImageMetadata.EXIFFNumber",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "image_focal_length",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeNumericRange,
		Description:   "Image focal length (mm)",
		DataSource:    "files.enhanced->ImageMetadata.EXIFFocalLength",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "audio_duration",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeNumericRange,
		Description:   "Audio duration in seconds",
		DataSource:    "files.enhanced->AudioMetadata.Duration",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "audio_bitrate",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeNumericRange,
		Description:   "Audio bitrate (kbps)",
		DataSource:    "files.enhanced->AudioMetadata.Bitrate",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "audio_sample_rate",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeNumericRange,
		Description:   "Audio sample rate (Hz)",
		DataSource:    "files.enhanced->AudioMetadata.SampleRate",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "audio_codec",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Audio codec",
		DataSource:    "files.enhanced->AudioMetadata.Codec",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "audio_format",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Audio container format",
		DataSource:    "files.enhanced->AudioMetadata.Format",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "audio_genre",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Audio genre",
		DataSource:    "files.enhanced->AudioMetadata.ID3Genre/VorbisGenre",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "audio_artist",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Audio artist",
		DataSource:    "files.enhanced->AudioMetadata.ID3Artist/VorbisArtist",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "audio_album",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Audio album",
		DataSource:    "files.enhanced->AudioMetadata.ID3Album/VorbisAlbum",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "audio_year",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Audio release year",
		DataSource:    "files.enhanced->AudioMetadata.ID3Year",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "audio_channels",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Audio channel count (mono, stereo, etc.)",
		DataSource:    "files.enhanced->AudioMetadata.Channels",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "audio_has_album_art",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Audio album art presence",
		DataSource:    "files.enhanced->AudioMetadata.HasAlbumArt",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "video_duration",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeNumericRange,
		Description:   "Video duration in seconds",
		DataSource:    "files.enhanced->VideoMetadata.Duration",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "video_bitrate",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeNumericRange,
		Description:   "Video bitrate (kbps)",
		DataSource:    "files.enhanced->VideoMetadata.Bitrate",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "video_frame_rate",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeNumericRange,
		Description:   "Video frame rate (fps)",
		DataSource:    "files.enhanced->VideoMetadata.FrameRate",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "video_codec",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Video codec",
		DataSource:    "files.enhanced->VideoMetadata.VideoCodec/Codec",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "video_audio_codec",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Video audio codec",
		DataSource:    "files.enhanced->VideoMetadata.AudioCodec",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "video_container",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Video container format",
		DataSource:    "files.enhanced->VideoMetadata.Container",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "video_aspect_ratio",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Video aspect ratio",
		DataSource:    "files.enhanced->VideoMetadata.VideoAspectRatio",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "video_has_subtitles",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Video has subtitles",
		DataSource:    "files.enhanced->VideoMetadata.HasSubtitles",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "video_subtitle_languages",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Video subtitle languages",
		DataSource:    "files.enhanced->VideoMetadata.SubtitleTracks",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "video_has_chapters",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Video has chapters",
		DataSource:    "files.enhanced->VideoMetadata.HasChapters",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "video_is_3d",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Video is 3D",
		DataSource:    "files.enhanced->VideoMetadata.Is3D",
		Availability:  AvailabilityConditional,
	})

	r.register(&FacetDefinition{
		CanonicalName: "video_quality_tier",
		Category:      FacetCategorySpecialized,
		Type:          FacetTypeTerms,
		Description:   "Video quality tier (SD, HD, 4K)",
		DataSource:    "files.enhanced->VideoMetadata.IsHD/Is4K/Width/Height",
		Availability:  AvailabilityConditional,
	})
}

// register registers a facet definition.
func (r *FacetRegistry) register(def *FacetDefinition) {
	r.facets[def.CanonicalName] = def
}

// Resolve resolves a field name (canonical or alias) to its facet definition.
func (r *FacetRegistry) Resolve(field string) (*FacetDefinition, error) {
	// First try canonical name
	if def, ok := r.facets[field]; ok {
		return def, nil
	}

	// Then try aliases
	for _, def := range r.facets {
		for _, alias := range def.Aliases {
			if alias == field {
				return def, nil
			}
		}
	}

	return nil, fmt.Errorf("unknown facet: %s", field)
}

// GetAll returns all registered facets.
func (r *FacetRegistry) GetAll() []*FacetDefinition {
	result := make([]*FacetDefinition, 0, len(r.facets))
	for _, def := range r.facets {
		result = append(result, def)
	}
	return result
}

// GetByCategory returns all facets in a category.
func (r *FacetRegistry) GetByCategory(category FacetCategory) []*FacetDefinition {
	result := make([]*FacetDefinition, 0)
	for _, def := range r.facets {
		if def.Category == category {
			result = append(result, def)
		}
	}
	return result
}

// GetByType returns all facets of a specific type.
func (r *FacetRegistry) GetByType(facetType FacetType) []*FacetDefinition {
	result := make([]*FacetDefinition, 0)
	for _, def := range r.facets {
		if def.Type == facetType {
			result = append(result, def)
		}
	}
	return result
}

// IsValid checks if a field name (canonical or alias) is a valid facet.
func (r *FacetRegistry) IsValid(field string) bool {
	_, err := r.Resolve(field)
	return err == nil
}

// GetCanonicalName returns the canonical name for a field (canonical or alias).
func (r *FacetRegistry) GetCanonicalName(field string) (string, error) {
	def, err := r.Resolve(field)
	if err != nil {
		return "", err
	}
	return def.CanonicalName, nil
}
